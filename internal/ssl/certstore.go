// Package ssl, sertifika yaşam döngüsünü yönetir (ROADMAP W7):
// ACME/Let's Encrypt edinimi, custom sertifika kurulumu, otomatik
// yenileme ve süre izleme.
package ssl

import (
	"fmt"
	"os"
	"path"
)

// CertStore, panelin cert deposu: certsRoot/<domain>/{fullchain.pem, privkey.pem}
// (ARCHITECTURE §7.1 state/certs). Panel kullanıcısına aittir — priv helper
// gerekmez. Özel anahtar 0600, zincir 0644; yazımlar ATOMİKTİR (tmp+rename).
type CertStore struct {
	root string
}

// NewCertStore, CertStore oluşturur.
func NewCertStore(root string) *CertStore { return &CertStore{root: root} }

// certDir, domain için dizin yolunu döndürür.
func (c *CertStore) certDir(domain string) string { return path.Join(c.root, domain) }

// Save, sertifika + anahtarı atomik yazar.
func (c *CertStore) Save(domain string, certPEM, keyPEM []byte) error {
	dir := c.certDir(domain)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("cert dizini: %w", err)
	}
	if err := atomicWrite(path.Join(dir, "fullchain.pem"), certPEM, 0o644); err != nil {
		return err
	}
	if err := atomicWrite(path.Join(dir, "privkey.pem"), keyPEM, 0o600); err != nil {
		return err
	}
	return nil
}

// Load, sertifika + anahtarı okur.
func (c *CertStore) Load(domain string) (certPEM, keyPEM []byte, err error) {
	dir := c.certDir(domain)
	certPEM, err = os.ReadFile(path.Join(dir, "fullchain.pem"))
	if err != nil {
		return nil, nil, fmt.Errorf("cert okunamadı: %w", err)
	}
	keyPEM, err = os.ReadFile(path.Join(dir, "privkey.pem"))
	if err != nil {
		return nil, nil, fmt.Errorf("key okunamadı: %w", err)
	}
	return certPEM, keyPEM, nil
}

// Remove, domain sertifikalarını siler.
func (c *CertStore) Remove(domain string) error {
	if err := os.RemoveAll(c.certDir(domain)); err != nil {
		return fmt.Errorf("cert silme: %w", err)
	}
	return nil
}

// atomicWrite, aynı dizinde tmp + rename ile atomik yazar.
func atomicWrite(target string, content []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(path.Dir(target), ".tmp-*")
	if err != nil {
		return fmt.Errorf("tmp dosya: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename sonrası artık yok; hata yolunda temizlik

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fmt.Errorf("yazma: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
