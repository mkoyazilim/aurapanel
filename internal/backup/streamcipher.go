// Package backup, site yedeklerini yönetir (ROADMAP W10):
// şifrele-önce-yükle (encrypt-then-upload), retention, restore.
package backup

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
)

// magic, yedek dosya biçimi imzası.
var magic = []byte("APBK1")

// chunkSize, AEAD şifreleme blok boyutu.
const chunkSize = 64 << 10

// EncryptWriter, yazılan düz metni chunked XChaCha20-Poly1305 ile
// şifreleyerek s değerine akıtır. Biçim: magic + tekrar
// [4B uzunluk][24B nonce][şifreli metin].
func EncryptWriter(key []byte, w io.Writer) (io.WriteCloser, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(magic); err != nil {
		return nil, err
	}
	return &encryptWriter{aead: aead, w: w}, nil
}

type encryptWriter struct {
	aead interface {
		Seal(dst, nonce, plaintext, additionalData []byte) []byte
		NonceSize() int
	}
	w io.Writer
}

func (e *encryptWriter) Write(p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		n := len(p)
		if n > chunkSize {
			n = chunkSize
		}
		nonce := make([]byte, e.aead.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			return total, err
		}
		ct := e.aead.Seal(nil, nonce, p[:n], nil)
		// Biçim: [4B uzunluk][24B nonce][şifreli metin+etiket] — nonce
		// ŞİFRELİ AKIŞA DAHİL EDİLİR; çözücü onu blok başından okur.
		blob := append(nonce, ct...)
		var lenBuf [4]byte
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(blob)))
		if _, err := e.w.Write(lenBuf[:]); err != nil {
			return total, err
		}
		if _, err := e.w.Write(blob); err != nil {
			return total, err
		}
		total += n
		p = p[n:]
	}
	return total, nil
}

func (e *encryptWriter) Close() error { return nil }

// DecryptReader, EncryptWriter çıktısını çözen okuyucu döndürür.
// Kurcalanmış veri doğrulama hatasıyla reddedilir (AEAD).
func DecryptReader(key []byte, r io.Reader) (io.Reader, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	head := make([]byte, len(magic))
	if _, err := io.ReadFull(r, head); err != nil {
		return nil, errors.New("yedek başlığı okunamadı")
	}
	if string(head) != string(magic) {
		return nil, errors.New("geçersiz yedek dosyası (imza uyuşmuyor)")
	}
	return &decryptReader{aead: aead, r: r}, nil
}

type decryptReader struct {
	aead interface {
		Open(dst, nonce, ciphertext, additionalData []byte) ([]byte, error)
		NonceSize() int
	}
	r       io.Reader
	pending []byte
	err     error
}

func (d *decryptReader) Read(p []byte) (int, error) {
	if d.err != nil {
		return 0, d.err
	}
	for len(d.pending) == 0 {
		var lenBuf [4]byte
		if _, err := io.ReadFull(d.r, lenBuf[:]); err != nil {
			d.err = err
			return 0, err
		}
		ctLen := int(binary.BigEndian.Uint32(lenBuf[:]))
		if ctLen <= 0 || ctLen > chunkSize+64 {
			d.err = errors.New("yedek blok boyutu bozuk")
			return 0, d.err
		}
		ct := make([]byte, ctLen)
		if _, err := io.ReadFull(d.r, ct); err != nil {
			d.err = fmt.Errorf("yedek okuma: %w", err)
			return 0, d.err
		}
		nonce, body := ct[:d.aead.NonceSize()], ct[d.aead.NonceSize():]
		plain, err := d.aead.Open(nil, nonce, body, nil)
		if err != nil {
			d.err = errors.New("yedek doğrulanamadı (bozuk/değiştirilmiş veri)")
			return 0, d.err
		}
		d.pending = plain
	}
	n := copy(p, d.pending)
	d.pending = d.pending[n:]
	return n, nil
}
