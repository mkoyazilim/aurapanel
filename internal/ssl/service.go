package ssl

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/ols"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// ACMEClient, sertifika edinme/yenileme soyutlaması (üretim: Let's Encrypt).
type ACMEClient interface {
	Obtain(ctx context.Context, domain string) (certPEM, keyPEM []byte, err error)
	Renew(ctx context.Context, domain string, oldCertPEM []byte) (certPEM []byte, err error)
}

// vhostApplier, vhost uygulama (ols.Pipeline — rollback güvenceli).
type vhostApplier interface {
	Apply(ctx context.Context, v ols.Vhost) error
}

// Service, sertifika yaşam döngüsü servisi.
type Service struct {
	store       *store.Store
	certs       *CertStore
	acme        ACMEClient
	vhost       vhostApplier
	audit       *audit.Service
	sitesRoot   string
	certsRoot   string
	renewBefore time.Duration
}

// NewService, Service oluşturur. renewBefore: yenileme eşiği (örn. 30 gün).
func NewService(st *store.Store, certs *CertStore, acme ACMEClient, v vhostApplier, au *audit.Service, sitesRoot, certsRoot string, renewBefore time.Duration) *Service {
	if renewBefore <= 0 {
		renewBefore = 30 * 24 * time.Hour
	}
	return &Service{store: st, certs: certs, acme: acme, vhost: v, audit: au,
		sitesRoot: sitesRoot, certsRoot: certsRoot, renewBefore: renewBefore}
}

// EnableLetsEncrypt, sitenin ana domaini için LE sertifikası edinir ve kurar.
func (s *Service) EnableLetsEncrypt(ctx context.Context, siteID string) error {
	st, err := s.requireSite(ctx, siteID)
	if err != nil {
		return err
	}
	domain, err := s.mainDomain(ctx, siteID)
	if err != nil {
		return err
	}
	certPEM, keyPEM, err := s.acme.Obtain(ctx, domain)
	if err != nil {
		return fmt.Errorf("acme edinimi: %w", err)
	}
	return s.install(ctx, st, domain, "letsencrypt", certPEM, keyPEM)
}

// InstallCustom, doğrulanmış custom sertifika kurar.
func (s *Service) InstallCustom(ctx context.Context, siteID string, certPEM, keyPEM []byte) error {
	st, err := s.requireSite(ctx, siteID)
	if err != nil {
		return err
	}
	domain, err := s.mainDomain(ctx, siteID)
	if err != nil {
		return err
	}
	return s.install(ctx, st, domain, "custom", certPEM, keyPEM)
}

// install, ortak kurulum akışı: çift doğrulama → depoya atomik yaz →
// DB desired state → vhost uygulama (rollback güvenceli; başarısızlık
// drift onarımıyla yinelenebilir).
func (s *Service) install(ctx context.Context, st *store.Site, domain, issuer string, certPEM, keyPEM []byte) error {
	cert, err := validatePair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("sertifika çifti: %w", err)
	}
	if err := s.certs.Save(domain, certPEM, keyPEM); err != nil {
		return err
	}

	d, err := s.store.GetDomainByName(ctx, domain)
	if err != nil {
		return err
	}
	if d == nil {
		return fmt.Errorf("domain kaydı yok: %s", domain)
	}

	if _, err := s.store.InsertSSLCert(ctx, store.SSLCert{
		SiteID: st.ID, DomainID: nullInt64(d.ID), Issuer: issuer,
		CertPath:  nullString(certPath(s.certsRoot, domain)),
		KeyPath:   nullString(keyPath(s.certsRoot, domain)),
		NotBefore: nullString(cert.NotBefore.UTC().Format(time.RFC3339)),
		NotAfter:  nullString(cert.NotAfter.UTC().Format(time.RFC3339)),
		AutoRenew: 1,
	}); err != nil {
		return err
	}
	if err := s.store.SetDomainSSL(ctx, d.ID, true); err != nil {
		return err
	}

	v, err := s.buildVhost(ctx, st)
	if err != nil {
		return err
	}
	if err := s.vhost.Apply(ctx, v); err != nil {
		return fmt.Errorf("vhost uygulanamadı: %w (desired state korundu; Repair ile yinelenebilir)", err)
	}

	s.audit.Write(ctx, audit.Event{
		Action: "ssl.install", Target: st.ID, Result: "success",
		Extra: map[string]any{"domain": domain, "issuer": issuer, "expires": cert.NotAfter.UTC().Format(time.RFC3339)},
	})
	return nil
}

// DisableSSL, site SSL'ini kapatır: önce vhost (SSL'siz) uygulanır,
// başarılıysa dosyalar ve kayıt temizlenir. Böylece OLS asla var olmayan
// sertifikalara işaret eden config ile bırakılmaz.
func (s *Service) DisableSSL(ctx context.Context, siteID string) error {
	st, err := s.requireSite(ctx, siteID)
	if err != nil {
		return err
	}
	domain, err := s.mainDomain(ctx, siteID)
	if err != nil {
		return err
	}
	d, err := s.store.GetDomainByName(ctx, domain)
	if err != nil {
		return err
	}
	if d == nil {
		return fmt.Errorf("domain kaydı yok: %s", domain)
	}

	// Desired state: SSL kapalı → sistem yakınsar.
	if err := s.store.SetDomainSSL(ctx, d.ID, false); err != nil {
		return err
	}
	v, err := s.buildVhost(ctx, st)
	if err != nil {
		return err
	}
	if err := s.vhost.Apply(ctx, v); err != nil {
		return fmt.Errorf("vhost uygulanamadı: %w (desired state korundu; Repair ile yinelenebilir)", err)
	}

	// Temizlik yalnızca sistem SSL'siz çalıştıktan SONRA.
	rec, err := s.store.GetSSLCertByDomain(ctx, d.ID)
	if err != nil {
		return err
	}
	if rec != nil {
		if err := s.store.DeleteSSLCert(ctx, rec.ID); err != nil {
			return err
		}
	}
	if err := s.certs.Remove(domain); err != nil {
		return err
	}
	s.audit.Write(ctx, audit.Event{Action: "ssl.disable", Target: st.ID, Result: "success"})
	return nil
}

// RenewDue, yenileme eşiğini geçmiş (auto_renew açık) sertifikaları yeniler.
// Dönen sayı: başarıyla yenilenen; hatalar audit'e yazılır ve akış sürer.
func (s *Service) RenewDue(ctx context.Context) (int, error) {
	cutoff := time.Now().UTC().Add(s.renewBefore).Format(time.RFC3339)
	due, err := s.store.ListSSLCertsExpiringBefore(ctx, cutoff)
	if err != nil {
		return 0, err
	}
	renewed := 0
	for _, rec := range due {
		if err := s.renewOne(ctx, &rec); err != nil {
			s.audit.Write(ctx, audit.Event{
				Action: "ssl.renew", Target: rec.SiteID, Result: "failed",
				Extra: map[string]any{"error": err.Error()},
			})
			continue
		}
		renewed++
	}
	return renewed, nil
}

func (s *Service) renewOne(ctx context.Context, rec *store.SSLCert) error {
	d, err := s.store.GetDomainByID(ctx, rec.DomainID.Int64)
	if err != nil {
		return err
	}
	if d == nil {
		return fmt.Errorf("domain kaydı yok: id=%d", rec.DomainID.Int64)
	}

	oldCert, keyPEM, err := s.certs.Load(d.Domain)
	if err != nil {
		return err
	}
	newCertPEM, err := s.acme.Renew(ctx, d.Domain, oldCert)
	if err != nil {
		return err
	}
	// Anahtar korunur (LE yenilemede anahtar değişmez); yalnızca zincir
	// doğrulanıp yeniden yazılır.
	if _, err := validatePair(newCertPEM, keyPEM); err != nil {
		return fmt.Errorf("yenilenen sertifika mevcut anahtarla eşleşmiyor: %w", err)
	}
	if err := s.certs.Save(d.Domain, newCertPEM, keyPEM); err != nil {
		return err
	}
	newCert, _ := parseCert(newCertPEM)
	if err := s.store.UpdateSSLCertAfterRenew(ctx, rec.ID,
		newCert.NotBefore.UTC().Format(time.RFC3339),
		newCert.NotAfter.UTC().Format(time.RFC3339)); err != nil {
		return err
	}

	// OLS yeniden yüklesin: vhost yeniden uygulanır (idempotent).
	st, err := s.requireSite(ctx, rec.SiteID)
	if err != nil {
		return err
	}
	v, err := s.buildVhost(ctx, st)
	if err != nil {
		return err
	}
	if err := s.vhost.Apply(ctx, v); err != nil {
		return fmt.Errorf("vhost: %w", err)
	}

	s.audit.Write(ctx, audit.Event{
		Action: "ssl.renew", Target: rec.SiteID, Result: "success",
		Extra: map[string]any{"domain": d.Domain},
	})
	return nil
}

// ExpiringSoon, izleme için yaklaşan süreleri raporlar.
func (s *Service) ExpiringSoon(ctx context.Context) ([]map[string]any, error) {
	cutoff := time.Now().Add(s.renewBefore).UTC().Format(time.RFC3339)
	due, err := s.store.ListSSLCertsExpiringBefore(ctx, cutoff)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(due))
	for _, rec := range due {
		d, err := s.store.GetDomainByID(ctx, rec.DomainID.Int64)
		if err != nil || d == nil {
			continue
		}
		out = append(out, map[string]any{
			"site_id":    rec.SiteID,
			"domain":     d.Domain,
			"not_after":  rec.NotAfter.String,
			"issuer":     rec.Issuer,
			"auto_renew": rec.AutoRenew == 1,
		})
	}
	return out, nil
}

// Info, sitenin ana domaini için SSL durumunu döndürür.
func (s *Service) Info(ctx context.Context, siteID string) (map[string]any, error) {
	domains, err := s.store.ListDomainsBySite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	var mainDomain *store.Domain
	for i, d := range domains {
		if d.Kind == "main" {
			mainDomain = &domains[i]
			break
		}
	}
	if mainDomain == nil {
		return map[string]any{"enabled": false}, nil
	}

	rec, err := s.store.GetSSLCertByDomain(ctx, mainDomain.ID)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return map[string]any{"enabled": false}, nil
	}

	return map[string]any{
		"enabled":    true,
		"domain":     mainDomain.Domain,
		"issuer":     rec.Issuer,
		"not_before": rec.NotBefore.String,
		"not_after":  rec.NotAfter.String,
		"auto_renew": rec.AutoRenew == 1,
	}, nil
}

// buildVhost, site kaydından vhost desired state'ini üretir; ana domainin
// ssl_enabled durumuna göre SSL bloğu eklenir.
func (s *Service) buildVhost(ctx context.Context, st *store.Site) (ols.Vhost, error) {
	domains, err := s.store.ListDomainsBySite(ctx, st.ID)
	if err != nil {
		return ols.Vhost{}, err
	}
	aliases := []string{}
	sslOn := false
	var mainDomain string
	for _, d := range domains {
		if d.Kind == "alias" {
			aliases = append(aliases, d.Domain)
		}
		if d.Kind == "main" {
			mainDomain = d.Domain
			sslOn = d.SSLenabled == 1
		}
	}
	var phpVersion string
	if st.PHPVersionID.Valid {
		phpVersion, err = s.store.GetPHPVersion(ctx, st.PHPVersionID.Int64)
		if err != nil {
			return ols.Vhost{}, err
		}
	}
	v := ols.Vhost{
		SiteID:     st.ID,
		Domain:     st.Name,
		Aliases:    aliases,
		PHPVersion: phpVersion,
		IndexFiles: []string{"index.php", "index.html"},
	}
	if sslOn && mainDomain != "" {
		v.SSL = &ols.SSLConfig{
			CertPath: certPath(s.certsRoot, mainDomain),
			KeyPath:  keyPath(s.certsRoot, mainDomain),
		}
	}
	return v, nil
}

// mainDomain, sitenin ana domainini döndürür.
func (s *Service) mainDomain(ctx context.Context, siteID string) (string, error) {
	domains, err := s.store.ListDomainsBySite(ctx, siteID)
	if err != nil {
		return "", err
	}
	for _, d := range domains {
		if d.Kind == "main" {
			return d.Domain, nil
		}
	}
	return "", fmt.Errorf("ana domain kaydı yok: %s", siteID)
}

func (s *Service) requireSite(ctx context.Context, siteID string) (*store.Site, error) {
	st, err := s.store.GetSite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, fmt.Errorf("site yok: %s", siteID)
	}
	return st, nil
}

func certPath(certsRoot, domain string) string { return certsRoot + "/" + domain + "/fullchain.pem" }
func keyPath(certsRoot, domain string) string  { return certsRoot + "/" + domain + "/privkey.pem" }

func nullInt64(v int64) sql.NullInt64     { return sql.NullInt64{Int64: v, Valid: true} }
func nullString(v string) sql.NullString  { return sql.NullString{String: v, Valid: true} }
