package ssl

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/ols"
	"github.com/mkoyazilim/aurapanel/internal/site"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

const (
	tSitesRoot = "/srv/aurapanel/sites"
)

// genKey, test için EC anahtarı üretir.
func genKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// certFor, verilen ANAHTARLA domain için sertifika üretir (yenileme
// testlerinde aynı anahtarın korunması için).
func certFor(t *testing.T, domain string, notAfter time.Time, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: domain},
		DNSNames:     []string{domain},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func keyPEM(t *testing.T, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	kb, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb})
}

// selfSigned, test için geçerli bir sertifika çifti üretir.
func selfSigned(t *testing.T, domain string, notAfter time.Time) (certPEM, keyPEMBytes []byte) {
	t.Helper()
	key := genKey(t)
	return certFor(t, domain, notAfter, key), keyPEM(t, key)
}

// --- Çift doğrulama ---

func TestValidatePair(t *testing.T) {
	c1, k1 := selfSigned(t, "example.com", time.Now().Add(24*time.Hour))
	c2, _ := selfSigned(t, "example.com", time.Now().Add(24*time.Hour))

	if _, err := validatePair(c1, k1); err != nil {
		t.Fatalf("geçerli çift reddedildi: %v", err)
	}
	if _, err := validatePair(c2, k1); err == nil {
		t.Fatal("eşleşmeyen anahtar kabul edildi")
	}
	if _, err := validatePair([]byte("çöp"), k1); err == nil {
		t.Fatal("bozuk sertifika kabul edildi")
	}
	if _, err := validatePair(c1, []byte("çöp")); err == nil {
		t.Fatal("bozuk anahtar kabul edildi")
	}
}

// --- CertStore ---

func TestCertStoreRoundTrip(t *testing.T) {
	cs := NewCertStore(filepath.Join(t.TempDir(), "certs"))
	c1, k1 := selfSigned(t, "example.com", time.Now().Add(24*time.Hour))
	if err := cs.Save("example.com", c1, k1); err != nil {
		t.Fatalf("Save: %v", err)
	}
	gotCert, gotKey, err := cs.Load("example.com")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(gotCert) != string(c1) || string(gotKey) != string(k1) {
		t.Fatal("roundtrip bozuldu")
	}
	if err := cs.Remove("example.com"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := cs.Load("example.com"); err == nil {
		t.Fatal("silinen cert okundu")
	}
}

// parseKeyForTest, test anahtarını ecdsa'ya çevirir.
func parseKeyForTest(t *testing.T, keyPEMBytes []byte) *ecdsa.PrivateKey {
	t.Helper()
	k, err := parseKey(keyPEMBytes)
	if err != nil {
		t.Fatal(err)
	}
	return k.(*ecdsa.PrivateKey)
}

// --- Service ---

type fakeACME struct {
	cert, key  []byte
	renewCert  []byte
	failObtain bool
	failRenew  bool
	obtains    []string
	renews     []string
}

func (f *fakeACME) Obtain(ctx context.Context, domain string) ([]byte, []byte, error) {
	f.obtains = append(f.obtains, domain)
	if f.failObtain {
		return nil, nil, fmt.Errorf("acme hata")
	}
	return f.cert, f.key, nil
}

func (f *fakeACME) Renew(ctx context.Context, domain string, oldCertPEM []byte) ([]byte, error) {
	f.renews = append(f.renews, domain)
	if f.failRenew {
		return nil, fmt.Errorf("renew hata")
	}
	return f.renewCert, nil
}

type fakeVhost struct {
	applied []ols.Vhost
	fail    bool
}

func (f *fakeVhost) Apply(ctx context.Context, v ols.Vhost) error {
	if f.fail {
		return fmt.Errorf("vhost hata")
	}
	f.applied = append(f.applied, v)
	return nil
}

func testService(t *testing.T) (*Service, *store.Store, *fakeACME, *fakeVhost, string) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "ssl.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	phpID, _ := st.InsertPHPVersion(ctx, "8.3", "/usr/local/lsws/lsphp83/bin/lsphp")
	limits, _ := json.Marshal(site.DefaultLimits())
	st.InsertSite(ctx, store.Site{
		ID: "site001", Name: "example.com", LinuxUser: "www-site001",
		HomeDir: tSitesRoot + "/site001/home", Status: "active",
		FeatureFlags: `{}`, Limits: string(limits),
		PHPVersionID: sql.NullInt64{Int64: phpID, Valid: true},
	})
	st.InsertDomain(ctx, store.Domain{SiteID: "site001", Domain: "example.com", Kind: "main"})
	st.InsertDomain(ctx, store.Domain{SiteID: "site001", Domain: "www.example.com", Kind: "alias"})
	st.UpsertPHPool(ctx, "site001", phpID, `{}`)

	certsRoot := filepath.Join(t.TempDir(), "certs")
	// AYNI anahtarla üretilmiş çift (ayrı üretimler eşleşmez — haklı red).
	acCert, acKey := selfSigned(t, "example.com", time.Now().Add(60*24*time.Hour))
	ac := &fakeACME{cert: acCert, key: acKey}
	fv := &fakeVhost{}
	svc := NewService(st, NewCertStore(certsRoot), ac, fv, audit.New(st), tSitesRoot, certsRoot, 30*24*time.Hour)
	return svc, st, ac, fv, certsRoot
}

func TestEnableLetsEncryptHappy(t *testing.T) {
	svc, st, ac, fv, certsRoot := testService(t)
	ctx := context.Background()

	if err := svc.EnableLetsEncrypt(ctx, "site001"); err != nil {
		t.Fatalf("EnableLetsEncrypt: %v", err)
	}
	if len(ac.obtains) != 1 || ac.obtains[0] != "example.com" {
		t.Fatalf("acme çağrısı: %v", ac.obtains)
	}
	// Dosyalar depoda.
	if _, err := os.Stat(certsRoot + "/example.com/privkey.pem"); err != nil {
		t.Fatalf("key dosyası yok: %v", err)
	}
	// DB: cert kaydı + ssl_enabled.
	d, _ := st.GetDomainByName(ctx, "example.com")
	if d.SSLenabled != 1 {
		t.Fatal("ssl_enabled ayarlanmadı")
	}
	rec, _ := st.GetSSLCertByDomain(ctx, d.ID)
	if rec == nil || rec.Issuer != "letsencrypt" {
		t.Fatalf("cert kaydı hatalı: %+v", rec)
	}
	// Vhost SSL bloğu ile uygulandı.
	if len(fv.applied) != 1 || fv.applied[0].SSL == nil {
		t.Fatalf("vhost SSL'siz uygulandı: %+v", fv.applied)
	}
	if fv.applied[0].SSL.CertPath != certsRoot+"/example.com/fullchain.pem" {
		t.Fatalf("cert yolu hatalı: %s", fv.applied[0].SSL.CertPath)
	}
}

func TestEnableAcmeFailureNoMutation(t *testing.T) {
	svc, st, ac, fv, _ := testService(t)
	ac.failObtain = true
	ctx := context.Background()

	if err := svc.EnableLetsEncrypt(ctx, "site001"); err == nil {
		t.Fatal("hata bekleniyordu")
	}
	if len(fv.applied) != 0 {
		t.Fatal("acme hatasında vhost uygulandı")
	}
	d, _ := st.GetDomainByName(ctx, "example.com")
	if d.SSLenabled != 0 {
		t.Fatal("acme hatasında ssl_enabled değişti")
	}
}

func TestInstallCustomRejectsMismatch(t *testing.T) {
	svc, _, _, fv, _ := testService(t)
	c1, _ := selfSigned(t, "example.com", time.Now().Add(24*time.Hour))
	_, k2 := selfSigned(t, "example.com", time.Now().Add(24*time.Hour))

	if err := svc.InstallCustom(context.Background(), "site001", c1, k2); err == nil {
		t.Fatal("eşleşmeyen çift kabul edildi")
	}
	if len(fv.applied) != 0 {
		t.Fatal("geçersiz çift vhost uyguladı")
	}
}

// Vhost başarısız olsa bile desired state korunur (yeniden denenebilir).
func TestInstallVhostFailureKeepsDesired(t *testing.T) {
	svc, st, _, fv, _ := testService(t)
	fv.fail = true
	c1, k1 := selfSigned(t, "example.com", time.Now().Add(60*24*time.Hour))

	err := svc.InstallCustom(context.Background(), "site001", c1, k1)
	if err == nil {
		t.Fatal("hata bekleniyordu")
	}
	d, _ := st.GetDomainByName(context.Background(), "example.com")
	if d.SSLenabled != 1 {
		t.Fatal("desired state korunmadı")
	}
}

func TestDisableSSLCleanupOrder(t *testing.T) {
	svc, st, _, fv, certsRoot := testService(t)
	ctx := context.Background()
	c1, k1 := selfSigned(t, "example.com", time.Now().Add(60*24*time.Hour))
	if err := svc.InstallCustom(ctx, "site001", c1, k1); err != nil {
		t.Fatal(err)
	}
	fv.applied = nil

	if err := svc.DisableSSL(ctx, "site001"); err != nil {
		t.Fatalf("DisableSSL: %v", err)
	}
	// Önce SSL'siz vhost uygulanmalı.
	if len(fv.applied) != 1 || fv.applied[0].SSL != nil {
		t.Fatalf("SSL'siz vhost uygulanmadı: %+v", fv.applied)
	}
	// Sonra temizlik.
	d, _ := st.GetDomainByName(ctx, "example.com")
	if d.SSLenabled != 0 {
		t.Fatal("ssl_enabled kapatılmadı")
	}
	rec, _ := st.GetSSLCertByDomain(ctx, d.ID)
	if rec != nil {
		t.Fatal("cert kaydı silinmedi")
	}
	if _, err := os.Stat(certsRoot + "/example.com"); !os.IsNotExist(err) {
		t.Fatal("cert dosyaları silinmedi")
	}
}

func TestRenewDue(t *testing.T) {
	svc, st, ac, fv, _ := testService(t)
	ctx := context.Background()

	// Süresi yaklaşan cert kur: renew öncesi not_after manipüle edilir.
	c1, k1 := selfSigned(t, "example.com", time.Now().Add(60*24*time.Hour))
	if err := svc.InstallCustom(ctx, "site001", c1, k1); err != nil {
		t.Fatal(err)
	}
	d, _ := st.GetDomainByName(ctx, "example.com")
	rec, _ := st.GetSSLCertByDomain(ctx, d.ID)
	// 10 gün sonra dolacakmış gibi işaretle (renewBefore 30 gün → due).
	if err := st.UpdateSSLCertAfterRenew(ctx, rec.ID, time.Now().UTC().Format(time.RFC3339), time.Now().Add(10*24*time.Hour).UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}

	// Yenilenecek cert: ORİJİNAL anahtarla imzalanmış yeni zincir
	// (LE yenilemede anahtar korunur — doğrulama bunu şart koşar).
	origKey := parseKeyForTest(t, k1)
	ac.renewCert = certFor(t, "example.com", time.Now().Add(90*24*time.Hour), origKey)

	n, err := svc.RenewDue(ctx)
	if err != nil {
		t.Fatalf("RenewDue: %v", err)
	}
	if n != 1 {
		t.Fatalf("1 yenileme bekleniyordu, %d oldu", n)
	}
	if len(ac.renews) != 1 {
		t.Fatal("acme.Renew çağrılmadı")
	}
	// DB güncellendi (yeni not_after 90 gün).
	rec2, _ := st.GetSSLCertByDomain(ctx, d.ID)
	if rec2.NotAfter.String < time.Now().Add(80*24*time.Hour).UTC().Format(time.RFC3339) {
		t.Fatalf("not_after güncellenmedi: %s", rec2.NotAfter.String)
	}
	// Vhost yeniden uygulandı (OLS reload).
	if len(fv.applied) != 2 {
		t.Fatalf("vhost yeniden uygulanmadı: %d", len(fv.applied))
	}
	// Artık due listesi boş.
	if n2, _ := svc.RenewDue(ctx); n2 != 0 {
		t.Fatalf("yenileme sonrası due kaldı: %d", n2)
	}
}

func TestExpiringSoon(t *testing.T) {
	svc, _, _, _, _ := testService(t)
	ctx := context.Background()
	c1, k1 := selfSigned(t, "example.com", time.Now().Add(60*24*time.Hour))
	if err := svc.InstallCustom(ctx, "site001", c1, k1); err != nil {
		t.Fatal(err)
	}
	// 60 gün > 30 gün eşik → boş.
	soon, err := svc.ExpiringSoon(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(soon) != 0 {
		t.Fatalf("eşik üstü cert 'yaklaşan' raporlandı: %+v", soon)
	}
}
