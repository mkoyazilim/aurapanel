package ssl

import (
	"context"
	"fmt"

	"golang.org/x/crypto/acme"
)

// Solver, ACME HTTP-01 challenge'larını çözer (üretim uygulaması OLS
// entegrasyonuyla birlikte gelir — sunucu entegrasyon fazında bağlanır).
type Solver interface {
	// Serve, /.well-known/acme-challenge/<token> için keyAuth'u servis etmeye
	// başlar (ve kaldırır).
	Present(ctx context.Context, domain, token, keyAuth string) error
	CleanUp(ctx context.Context, domain, token, keyAuth string) error
}

// LetsEncryptClient, ACME üzerinden sertifika edinir (Let's Encrypt).
// Solver bağlı değilse Obtain/Renew hemen hata döner — sertifika akışı
// asla yarım challenge ile ilerlemez.
type LetsEncryptClient struct {
	DirectoryURL string // varsayılan: Let's Encrypt production
	Contact      string // kayıt e-postası
	AgreedTOS    bool
	Solver       Solver
}

// NewLetsEncryptClient, production ACME istemcisini oluşturur.
func NewLetsEncryptClient(contact string, solver Solver) *LetsEncryptClient {
	return &LetsEncryptClient{
		DirectoryURL: acme.LetsEncryptURL,
		Contact:      contact,
		AgreedTOS:    true,
		Solver:       solver,
	}
}

// Obtain, yeni sertifika + anahtar edinir (HTTP-01).
func (c *LetsEncryptClient) Obtain(ctx context.Context, domain string) (certPEM, keyPEM []byte, err error) {
	if c.Solver == nil {
		return nil, nil, fmt.Errorf("acme solver bağlı değil (OLS entegrasyonu bekleniyor)")
	}
	return nil, nil, fmt.Errorf("acme istemcisi entegrasyon fazında bağlanacak: %s", domain)
}

// Renew, mevcut sertifika için yenileme yapar.
func (c *LetsEncryptClient) Renew(ctx context.Context, domain string, oldCertPEM []byte) (certPEM []byte, err error) {
	if c.Solver == nil {
		return nil, fmt.Errorf("acme solver bağlı değil (OLS entegrasyonu bekleniyor)")
	}
	return nil, fmt.Errorf("acme istemcisi entegrasyon fazında bağlanacak: %s", domain)
}
