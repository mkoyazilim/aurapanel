package ssl

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"path"
	"strings"

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

// LocalSolver, yerel sunucuda acme-challenge dosyalarını yönetir.
type LocalSolver struct {
	AcmeDir string
}

// NewLocalSolver oluşturur.
func NewLocalSolver(acmeDir string) *LocalSolver {
	// Dizinin var olduğundan emin ol.
	_ = os.MkdirAll(acmeDir, 0o755)
	return &LocalSolver{AcmeDir: acmeDir}
}

// Present, token dosyasını acme dizinine yazar.
func (s *LocalSolver) Present(ctx context.Context, domain, token, keyAuth string) error {
	filePath := path.Join(s.AcmeDir, token)
	// Dosya yolunun dışarı çıkmasını engelle (gerçi token güvenlidir ama yine de)
	if !strings.HasPrefix(filePath, path.Clean(s.AcmeDir)+string(os.PathSeparator)) {
		return fmt.Errorf("geçersiz token: %s", token)
	}
	return os.WriteFile(filePath, []byte(keyAuth), 0o644)
}

// CleanUp, token dosyasını siler.
func (s *LocalSolver) CleanUp(ctx context.Context, domain, token, keyAuth string) error {
	filePath := path.Join(s.AcmeDir, token)
	if !strings.HasPrefix(filePath, path.Clean(s.AcmeDir)+string(os.PathSeparator)) {
		return nil
	}
	return os.Remove(filePath)
}

// LetsEncryptClient, ACME üzerinden sertifika edinir (Let's Encrypt).
type LetsEncryptClient struct {
	DirectoryURL string // varsayılan: Let's Encrypt production
	Contact      string // kayıt e-postası
	Solver       Solver
}

// NewLetsEncryptClient, production ACME istemcisini oluşturur.
func NewLetsEncryptClient(contact string, solver Solver) *LetsEncryptClient {
	return &LetsEncryptClient{
		DirectoryURL: acme.LetsEncryptURL,
		Contact:      contact,
		Solver:       solver,
	}
}

func (c *LetsEncryptClient) createClient() *acme.Client {
	return &acme.Client{
		DirectoryURL: c.DirectoryURL,
	}
}

// Obtain, yeni sertifika + anahtar edinir (HTTP-01).
func (c *LetsEncryptClient) Obtain(ctx context.Context, domain string) (certPEM, keyPEM []byte, err error) {
	if c.Solver == nil {
		return nil, nil, fmt.Errorf("acme solver bağlı değil")
	}

	client := c.createClient()
	
	// 1. Hesap oluştur veya getir. (MVP: her seferinde yeni geçici hesap oluşturuyoruz, ileride e-posta tabanlı kalıcı hesap yapılabilir).
	account, err := client.Register(ctx, &acme.Account{
		Contact: []string{"mailto:" + c.Contact},
	}, acme.AcceptTOS)
	if err != nil && err != acme.ErrAccountAlreadyExists {
		return nil, nil, fmt.Errorf("acme register hatası: %w", err)
	}
	_ = account

	// 2. Order oluştur.
	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(domain))
	if err != nil {
		return nil, nil, fmt.Errorf("acme authorize order hatası: %w", err)
	}

	// 3. Challenge'ları çöz.
	for _, authzURL := range order.AuthzURLs {
		authz, err := client.GetAuthorization(ctx, authzURL)
		if err != nil {
			return nil, nil, fmt.Errorf("acme authz fetch hatası: %w", err)
		}
		if authz.Status == acme.StatusValid {
			continue
		}
		var challenge *acme.Challenge
		for _, ch := range authz.Challenges {
			if ch.Type == "http-01" {
				challenge = ch
				break
			}
		}
		if challenge == nil {
			return nil, nil, fmt.Errorf("http-01 challenge bulunamadı: %s", domain)
		}

		keyAuth, err := client.HTTP01ChallengeResponse(challenge.Token)
		if err != nil {
			return nil, nil, fmt.Errorf("keyauth üretilemedi: %w", err)
		}

		// Present
		if err := c.Solver.Present(ctx, domain, challenge.Token, keyAuth); err != nil {
			return nil, nil, fmt.Errorf("solver present hatası: %w", err)
		}
		defer c.Solver.CleanUp(context.Background(), domain, challenge.Token, keyAuth)

		// Challenge'ı doğrula
		if _, err := client.Accept(ctx, challenge); err != nil {
			return nil, nil, fmt.Errorf("challenge accept hatası: %w", err)
		}

		// Authz durumunu bekle
		if _, err := client.WaitAuthorization(ctx, authzURL); err != nil {
			return nil, nil, fmt.Errorf("authz wait hatası: %w", err)
		}
	}

	// 4. CSR oluştur ve sertifikayı al.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("key üretilemedi: %w", err)
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: domain},
		DNSNames: []string{domain},
	}, key)
	if err != nil {
		return nil, nil, fmt.Errorf("csr üretilemedi: %w", err)
	}
	
	der, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil {
		return nil, nil, fmt.Errorf("cert alınamadı: %w", err)
	}

	// PEM formatına çevir
	var certBuf bytes.Buffer
	for _, b := range der {
		pem.Encode(&certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: b})
	}
	
	keyDer, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	var keyBuf bytes.Buffer
	pem.Encode(&keyBuf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer})

	return certBuf.Bytes(), keyBuf.Bytes(), nil
}

// Renew, mevcut sertifika için yenileme yapar.
func (c *LetsEncryptClient) Renew(ctx context.Context, domain string, oldCertPEM []byte) (certPEM []byte, err error) {
	// Let's Encrypt için yenileme aslında yeniden Obtain demektir.
	// Private key'i değiştirebilir veya aynı tutabiliriz. Güvenlik için yeni key üretmek (Obtain çağırmak) daha iyidir.
	cert, _, err := c.Obtain(ctx, domain)
	return cert, err
}
