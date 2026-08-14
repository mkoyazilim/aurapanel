package php

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/ols"
	"github.com/mkoyazilim/aurapanel/internal/site"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

const (
	tSitesRoot = "/srv/aurapanel/sites"
	tCertsRoot = "/srv/aurapanel/state/certs"
)

// --- Ini doğrulama testleri ---

func TestValidateSettingsTable(t *testing.T) {
	good := map[string]string{
		"memory_limit":         "256M",
		"upload_max_filesize":  "64M",
		"max_execution_time":   "30",
		"max_input_vars":       "5000",
		"date.timezone":        "Europe/Istanbul",
		"display_errors":       "off",
		"session.gc_maxlifetime": "1440",
	}
	norm, err := ValidateSettings(good)
	if err != nil {
		t.Fatalf("geçerli ayarlar reddedildi: %v", err)
	}
	if norm["display_errors"] != "Off" {
		t.Errorf("bool normalize edilmedi: %q", norm["display_errors"])
	}

	bad := []map[string]string{
		{"open_basedir": "/home"},          // güvenlik profili kapsamında — kullanıcıya kapalı
		{"disable_functions": "exec"},      // güvenlik profili kapsamında
		{"error_log": "/etc/passwd"},       // panel türetir
		{"memory_limit": "256"},            // birim yok
		{"memory_limit": "9999999M"},       // aralık dışı
		{"max_execution_time": "-5"},       // negatif
		{"max_input_vars": "abc"},          // sayı değil
		{"date.timezone": "Mars/Olympus"},  // geçersiz bölge
		{"display_errors": "belki"},        // geçersiz bool
		{"exec": "1"},                      // allowlist dışı
		{},                                 // boş
	}
	for i, m := range bad {
		if _, err := ValidateSettings(m); err == nil {
			t.Errorf("durum %d reddedilmedi: %v", i, m)
		}
	}
}

func TestRenderIniDeterministic(t *testing.T) {
	m := map[string]string{"memory_limit": "256M", "max_execution_time": "30"}
	a := RenderIni(m)
	b := RenderIni(m)
	if a != b {
		t.Fatal("render deterministik değil")
	}
	if !strings.Contains(a, "memory_limit = 256M\n") {
		t.Fatalf("içerik hatalı: %q", a)
	}
}

// --- Service testleri ---

type fakePriv struct {
	detected map[string]bool
	ini      map[string]string
	calls    []string
}

func (f *fakePriv) DetectPHP(ctx context.Context) (map[string]bool, error) {
	f.calls = append(f.calls, "detect")
	return f.detected, nil
}

func (f *fakePriv) InstallIni(ctx context.Context, siteID, content string) error {
	f.calls = append(f.calls, "install_ini")
	f.ini[siteID] = content
	return nil
}

func (f *fakePriv) ReadIni(ctx context.Context, siteID string) (string, error) {
	f.calls = append(f.calls, "read_ini")
	return f.ini[siteID], nil
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

func testService(t *testing.T) (*Service, *store.Store, *fakePriv, *fakeVhost) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "php.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, v := range []string{"8.2", "8.3", "8.4"} {
		if _, err := st.InsertPHPVersion(ctx, v, "/usr/local/lsws/lsphp"+strings.ReplaceAll(v, ".", "")+"/bin/lsphp"); err != nil {
			t.Fatal(err)
		}
	}
	limits, _ := json.Marshal(site.DefaultLimits())
	st.InsertSite(ctx, store.Site{
		ID: "site001", Name: "example.com", LinuxUser: "www-site001",
		HomeDir: tSitesRoot + "/site001/home", Status: "active",
		FeatureFlags: `{}`, Limits: string(limits),
		PHPVersionID: sql.NullInt64{Int64: 1, Valid: true}, // 8.2
	})
	st.InsertDomain(ctx, store.Domain{SiteID: "site001", Domain: "example.com", Kind: "main"})
	// Gerçek site.Create akışı pool kaydını da oluşturur (W6).
	if err := st.UpsertPHPool(ctx, "site001", 1, `{}`); err != nil {
		t.Fatal(err)
	}

	fp := &fakePriv{detected: map[string]bool{"8.2": true, "8.3": true, "8.4": false}, ini: map[string]string{}}
	fv := &fakeVhost{}
	svc := NewService(st, fp, fv, audit.New(st), tSitesRoot, tCertsRoot)
	return svc, st, fp, fv
}

func TestListVersions(t *testing.T) {
	svc, _, _, _ := testService(t)
	vs, err := svc.ListVersions(context.Background())
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(vs) != 3 {
		t.Fatalf("3 sürüm bekleniyordu: %+v", vs)
	}
	for _, v := range vs {
		want := v.Version != "8.4"
		if v.Installed != want {
			t.Errorf("%s Installed=%v, beklenen %v", v.Version, v.Installed, want)
		}
	}
}

func TestSwitchVersionHappy(t *testing.T) {
	svc, st, _, fv := testService(t)
	ctx := context.Background()

	if err := svc.SwitchVersion(ctx, "site001", "8.3"); err != nil {
		t.Fatalf("SwitchVersion: %v", err)
	}
	if len(fv.applied) != 1 || fv.applied[0].PHPVersion != "8.3" {
		t.Fatalf("vhost yanlış sürümle uygulandı: %+v", fv.applied)
	}
	s, _ := st.GetSite(ctx, "site001")
	ver, err := st.GetPHPVersion(ctx, s.PHPVersionID.Int64)
	if err != nil || ver != "8.3" {
		t.Fatalf("store sürümü: %s err=%v", ver, err)
	}
	pid, _, ok, _ := st.GetPHPool(ctx, "site001")
	if !ok || pid != 2 {
		t.Fatalf("pool güncellenmedi: pid=%d ok=%v", pid, ok)
	}
}

// Vhost başarısız olsa bile desired state korunur (drift onarımıyla
// yinelenebilir) — site asla yarım kalmaz.
func TestSwitchVersionVhostFailure(t *testing.T) {
	svc, st, _, fv := testService(t)
	fv.fail = true
	ctx := context.Background()

	err := svc.SwitchVersion(ctx, "site001", "8.4")
	if err == nil {
		t.Fatal("hata bekleniyordu")
	}
	if !strings.Contains(err.Error(), "desired state korundu") {
		t.Fatalf("yeniden denenebilirlik mesajı yok: %v", err)
	}
	s, _ := st.GetSite(ctx, "site001")
	ver, _ := st.GetPHPVersion(ctx, s.PHPVersionID.Int64)
	if ver != "8.4" {
		t.Fatalf("desired state güncellenmemiş: %s", ver)
	}
}

func TestSwitchVersionRejectsUnknown(t *testing.T) {
	svc, _, _, fv := testService(t)
	if err := svc.SwitchVersion(context.Background(), "site001", "9.9"); err == nil {
		t.Fatal("bilinmeyen sürüm kabul edildi")
	}
	if len(fv.applied) != 0 {
		t.Fatal("reddedilen switch vhost uyguladı")
	}
}

func TestSetIni(t *testing.T) {
	svc, st, fp, _ := testService(t)
	ctx := context.Background()

	// Geçersiz yönerge: priv çağrısı YAPILMAMALI.
	if err := svc.SetIni(ctx, "site001", map[string]string{"open_basedir": "/"}); err == nil {
		t.Fatal("engellenmiş yönerge kabul edildi")
	}
	if len(fp.calls) != 0 {
		t.Fatalf("doğrulama hatasında priv çağrıldı: %v", fp.calls)
	}

	// Geçerli: içerik priv'e gider, pool settings güncellenir.
	if err := svc.SetIni(ctx, "site001", map[string]string{"memory_limit": "512M", "display_errors": "Off"}); err != nil {
		t.Fatalf("SetIni: %v", err)
	}
	if !strings.Contains(fp.ini["site001"], "memory_limit = 512M\n") {
		t.Fatalf("ini içeriği hatalı: %q", fp.ini["site001"])
	}
	_, settings, _, _ := st.GetPHPool(ctx, "site001")
	if !strings.Contains(settings, `"memory_limit":"512M"`) {
		t.Fatalf("pool settings güncellenmedi: %s", settings)
	}
}
