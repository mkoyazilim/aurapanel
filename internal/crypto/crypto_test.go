package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := c.Encrypt([]byte("süper-gizli-parola"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := c.Decrypt(enc)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != "süper-gizli-parola" {
		t.Fatalf("roundtrip bozuldu: %q", plain)
	}
}

// Şifreli metin üzerinde tek bayt değişikliği doğrulamayı kırmalı (AEAD).
func TestDecryptRejectsTampering(t *testing.T) {
	key, _ := GenerateKey()
	c, _ := New(key)
	enc, _ := c.Encrypt([]byte("parola"))

	tampered := []byte(enc)
	idx := len(tampered) / 2
	if tampered[idx] == 'A' {
		tampered[idx] = 'B'
	} else {
		tampered[idx] = 'A'
	}
	if _, err := c.Decrypt(string(tampered)); err == nil {
		t.Fatal("kurcalanmış veri çözüldü")
	}
}

func TestLoadKeyFile(t *testing.T) {
	key, _ := GenerateKey()
	p := filepath.Join(t.TempDir(), "master.key")
	if err := os.WriteFile(p, key, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadKey(p); err != nil {
		t.Fatalf("LoadKey: %v", err)
	}

	// Yanlış boyutlu anahtar reddedilmeli.
	bad := filepath.Join(t.TempDir(), "bad.key")
	os.WriteFile(bad, []byte("kısa"), 0o600)
	if _, err := LoadKey(bad); err == nil {
		t.Fatal("yanlış boyutlu anahtar kabul edildi")
	}
	// Olmayan dosya: asla otomatik anahtar üretilmez.
	if _, err := LoadKey(filepath.Join(t.TempDir(), "yok.key")); err == nil {
		t.Fatal("eksik anahtar dosyası hata üretmeli")
	}
}
