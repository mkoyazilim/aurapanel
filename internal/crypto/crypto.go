// Package crypto, geri okunması gereken secret'lar için encrypted-at-rest
// sağlar (ARCHITECTURE §4.2): XChaCha20-Poly1305 (AEAD).
//
// Master key, metadata DB'de ASLA bulunmaz; kurulumda üretilen 32 baytlık
// dosyadan yüklenir (0600).
package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/chacha20poly1305"
)

// KeySize, XChaCha20-Poly1305 anahtar boyutu.
const KeySize = chacha20poly1305.KeySize

// Cipher, AEAD şifreleme aracı.
type Cipher struct {
	aead interface {
		Seal(dst, nonce, plaintext, additionalData []byte) []byte
		Open(dst, nonce, ciphertext, additionalData []byte) ([]byte, error)
		NonceSize() int
	}
	nonceSize int
}

// LoadKey, anahtar dosyasını yükler. Dosya yoksa hata döner — anahtarı
// yalnızca kurulum üretir (asla otomatik türetme yok).
func LoadKey(path string) (*Cipher, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("master key okunamadı %s: %w", path, err)
	}
	if len(b) != KeySize {
		return nil, fmt.Errorf("master key boyutu hatalı: %d (beklenen %d)", len(b), KeySize)
	}
	return New(b)
}

// New, 32 baytlık anahtardan Cipher oluşturur.
func New(key []byte) (*Cipher, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead, nonceSize: aead.NonceSize()}, nil
}

// GenerateKey, yeni anahtar üretir (kurulum için).
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// Encrypt, düz metni şifreler: base64(nonce || ciphertext).
func (c *Cipher) Encrypt(plain []byte) (string, error) {
	nonce := make([]byte, c.nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nil, nonce, plain, nil)
	out := append(nonce, sealed...)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt, Encrypt çıktısını çözer.
func (c *Cipher) Decrypt(encoded string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("şifreli veri çözümlenemedi")
	}
	if len(decoded) <= c.nonceSize {
		return nil, errors.New("geçersiz ciphertext")
	}
	nonce := decoded[:c.nonceSize]
	ciphertext := decoded[c.nonceSize:]
	return c.aead.Open(nil, nonce, ciphertext, nil)
}

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}
