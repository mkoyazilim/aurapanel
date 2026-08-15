package drift

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

// --- Test ortamı ---

func testStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "drift.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	return st
}

// seedSite, aktif bir site + PHP sürümü + domain/alias ekler.
func seedSite(t *testing.T, st *store.Store) (*store.Site, string) {
	t.Helper()
	ctx := context.Background()
	phpID, err := st.InsertPHPVersion(ctx, "8.3", "/usr/local/lsws/lsphp83/bin/lsphp")
	if err != nil {
		t.Fatal(err)
	}
	limits, _ := json.Marshal(site.DefaultLimits())
	s := &store.Site{
		ID: "site001", Name: "example.com", LinuxUser: "www-site001",
		HomeDir: "/srv/aurapanel/sites/site001/home",
		Status:  "active", FeatureFlags: `{}`, Limits: string(limits),
		PHPVersionID: nullableInt64(phpID),
	}
	if err := st.InsertSite(ctx, *s); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertDomain(ctx, store.Domain{SiteID: s.ID, Domain: "example.com", Kind: "main"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertDomain(ctx, store.Domain{SiteID: s.ID, Domain: "www.example.com", Kind: "alias"}); err != nil {
		t.Fatal(err)
	}
	return s, "8.3"
}

func nullableInt64(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: true} }

// desiredFor ile temiz bir istek hâli üret.
func cleanDesired(t *testing.T, st *store.Store) *desiredState {
	t.Helper()
	s, php := seedSite(t, st)
	domains, _ := st.ListDomainsBySite(context.Background(), s.ID)
	d, err := desiredFor(s, domains, php, nil, tSitesRoot, tCertsRoot)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// cleanActuals, diff'i SIFIR bulgu üreten gerçek durum (d'ye göre).
// NOT: Cgroup haritası KOPYALANIR — test mutasyonları desired state'i
// bozmamalı (aliasing).
func cleanActuals(d *desiredState) actuals {
	cgroup := map[string]string{}
	for k, v := range d.Cgroup {
		cgroup[k] = v
	}
	return actuals{
		VhostContent: d.VhostContent,
		UserExists:   true,
		Cgroup:       cgroup,
		Quota:        quotaActual{Available: true, DiskBlocks: d.DiskBlocks, Inodes: d.Inodes},
		Dirs: map[string]dirActual{
			"home": {Exists: true, Mode: 0o750},
			"logs": {Exists: true, Mode: 0o750},
			"tmp":  {Exists: true, Mode: 0o700},
		},
	}
}

// --- Diff testleri ---

func TestDiffClean(t *testing.T) {
	st := testStore(t)
	d := cleanDesired(t, st)
	if got := diff(d, cleanActuals(d)); len(got) != 0 {
		t.Fatalf("temiz durumda sapma bulundu: %+v", got)
	}
}

func TestDiffDetectsEverything(t *testing.T) {
	st := testStore(t)
	d := cleanDesired(t, st)

	// Her kaynağı tek tek boz ve ilgili sapmanın çıktığını doğrula.
	cases := []struct {
		name     string
		mutate   func(*actuals)
		resource string
		severity string
	}{
		{"vhost değiştirildi", func(a *actuals) { a.VhostContent = "# saldırı" }, "ols.vhost", sevCritical},
		{"vhost silindi", func(a *actuals) { a.VhostContent = "" }, "ols.vhost", sevCritical},
		{"kullanıcı yok", func(a *actuals) { a.UserExists = false }, "linux.user", sevCritical},
		{"cpu bozuldu", func(a *actuals) { a.Cgroup["cpu.max"] = "max" }, "cgroup.cpu.max", sevWarning},
		{"memory bozuldu", func(a *actuals) { a.Cgroup["memory.max"] = "1" }, "cgroup.memory.max", sevWarning},
		{"quota disk bozuldu", func(a *actuals) { a.Quota.DiskBlocks = 1 }, "quota.disk", sevWarning},
		{"quota inode bozuldu", func(a *actuals) { a.Quota.Inodes = 1 }, "quota.inodes", sevWarning},
		{"home silindi", func(a *actuals) { a.Dirs["home"] = dirActual{Exists: false} }, "fs.home", sevCritical},
		{"tmp mod bozuldu", func(a *actuals) { a.Dirs["tmp"] = dirActual{Exists: true, Mode: 0o777} }, "fs.tmp", sevWarning},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			act := cleanActuals(d)
			tc.mutate(&act)
			found := diff(d, act)
			for _, f := range found {
				if f.Resource == tc.resource {
					if f.Severity != tc.severity {
						t.Errorf("%s şiddeti %s, beklenen %s", tc.resource, f.Severity, tc.severity)
					}
					return
				}
			}
			t.Errorf("%s sapması bulunamadı; bulunanlar: %+v", tc.resource, found)
		})
	}
}

// Quota etkin değilse drift ÜRETİLMEMELİ (kurulum sorunu, drift değil).
func TestDiffSkipsUnavailableQuota(t *testing.T) {
	st := testStore(t)
	d := cleanDesired(t, st)
	act := cleanActuals(d)
	act.Quota = quotaActual{Available: false, Reason: "quota etkin değil"}
	if got := diff(d, act); len(got) != 0 {
		t.Fatalf("etkin olmayan quota drift üretti: %+v", got)
	}
}

// --- Scanner testleri ---

type fakeCollector struct {
	vhost  map[string]string
	users  map[string]bool
	cgroup map[string]map[string]string
	quota  map[string]quotaActual
	dirs   map[string]map[string]dirActual
	failOp string
	calls  []string
}

func newFakeCollector() *fakeCollector {
	return &fakeCollector{
		vhost:  map[string]string{},
		users:  map[string]bool{},
		cgroup: map[string]map[string]string{},
		quota:  map[string]quotaActual{},
		dirs:   map[string]map[string]dirActual{},
	}
}

func (f *fakeCollector) VhostBundle(ctx context.Context, siteID string) (string, error) {
	f.calls = append(f.calls, "vhost:"+siteID)
	if f.failOp == "vhost" {
		return "", fmt.Errorf("vhost okuma hatası")
	}
	return f.vhost[siteID], nil
}

func (f *fakeCollector) UserExists(ctx context.Context, name string) (bool, error) {
	f.calls = append(f.calls, "user:"+name)
	return f.users[name], nil
}

func (f *fakeCollector) CgroupRead(ctx context.Context, siteID string) (map[string]string, error) {
	f.calls = append(f.calls, "cgroup:"+siteID)
	return f.cgroup[siteID], nil
}

func (f *fakeCollector) QuotaGet(ctx context.Context, name string) (quotaActual, error) {
	f.calls = append(f.calls, "quota:"+name)
	return f.quota[name], nil
}

func (f *fakeCollector) SiteStatus(ctx context.Context, siteID, name string) (map[string]dirActual, error) {
	f.calls = append(f.calls, "dirs:"+siteID)
	return f.dirs[siteID], nil
}

// syncCollector, sahte toplayıcıyı temiz bir desired state'e göre doldurur.
func (f *fakeCollector) sync(d *desiredState) {
	f.vhost["site001"] = d.VhostContent
	f.users[d.User] = true
	f.cgroup["site001"] = d.Cgroup
	f.quota[d.User] = quotaActual{Available: true, DiskBlocks: d.DiskBlocks, Inodes: d.Inodes}
	f.dirs["site001"] = map[string]dirActual{
		"home": {Exists: true, Mode: 0o750},
		"logs": {Exists: true, Mode: 0o750},
		"tmp":  {Exists: true, Mode: 0o700},
	}
}

func TestScanCleanNoDrift(t *testing.T) {
	st := testStore(t)
	d := cleanDesired(t, st)
	fc := newFakeCollector()
	fc.sync(d)

	sc := NewScanner(st, fc, tSitesRoot, tCertsRoot, audit.New(st), nil)
	n, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if n != 0 {
		t.Fatalf("temiz sistemde %d sapma", n)
	}
	open, _ := st.ListOpenDriftEvents(context.Background())
	if len(open) != 0 {
		t.Fatalf("drift kaydı oluştu: %+v", open)
	}
}

// Manuel bozulan config algılanmalı ve kaydedilmeli (ARCHITECTURE §6 örneği).
func TestScanDetectsDrift(t *testing.T) {
	st := testStore(t)
	d := cleanDesired(t, st)
	fc := newFakeCollector()
	fc.sync(d)
	// "vhost silinmiş" + "PHP havuzu değiştirilmiş" senaryoları:
	fc.vhost["site001"] = ""
	fc.cgroup["site001"]["memory.max"] = "1"

	sc := NewScanner(st, fc, tSitesRoot, tCertsRoot, audit.New(st), nil)
	n, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if n != 2 {
		t.Fatalf("2 sapma bekleniyordu, %d bulundu", n)
	}
	open, _ := st.ListOpenDriftEvents(context.Background())
	if len(open) != 2 {
		t.Fatalf("drift kayıtları: %+v", open)
	}
	resources := map[string]bool{}
	for _, e := range open {
		resources[e.Resource] = true
		if e.SiteID != "site001" || e.Status != "open" {
			t.Fatalf("kayıt hatalı: %+v", e)
		}
	}
	if !resources["ols.vhost"] || !resources["cgroup.memory.max"] {
		t.Fatalf("beklenen kaynaklar yok: %v", resources)
	}
}

// --- Repair testleri ---

type fakeRepairOps struct {
	calls []string
	fail  string
}

func (f *fakeRepairOps) SitePrepare(ctx context.Context, siteID, user string) error {
	f.calls = append(f.calls, "site.prepare")
	if f.fail == "prepare" {
		return fmt.Errorf("prepare hata")
	}
	return nil
}

func (f *fakeRepairOps) CgroupLimits(ctx context.Context, siteID string, l site.Limits) error {
	f.calls = append(f.calls, "cgroup.limits")
	return nil
}

func (f *fakeRepairOps) QuotaSet(ctx context.Context, user string, diskMB, inodes uint64) error {
	f.calls = append(f.calls, "quota.set")
	return nil
}

func (f *fakeRepairOps) VhostApply(ctx context.Context, v ols.Vhost) error {
	f.calls = append(f.calls, "vhost.apply")
	if f.fail == "vhost" {
		return fmt.Errorf("vhost hata")
	}
	return nil
}

func TestRepairHappy(t *testing.T) {
	st := testStore(t)
	seedSite(t, st)
	fo := &fakeRepairOps{}

	r := NewRepairer(st, fo, tSitesRoot, tCertsRoot)
	if err := r.Repair(context.Background(), "site001"); err != nil {
		t.Fatalf("Repair: %v", err)
	}
	want := "site.prepare,cgroup.limits,quota.set,vhost.apply"
	if got := strings.Join(fo.calls, ","); got != want {
		t.Fatalf("repair sırası: %s (beklenen %s)", got, want)
	}
}

// Auto-repair: politika açıkken sapma otomatik onarılmalı.
func TestScanAutoRepair(t *testing.T) {
	st := testStore(t)
	d := cleanDesired(t, st)
	fc := newFakeCollector()
	fc.sync(d)
	fc.vhost["site001"] = "# bozuk" // sapma
	fo := &fakeRepairOps{}

	sc := NewScanner(st, fc, tSitesRoot, tCertsRoot, audit.New(st), NewRepairer(st, fo, tSitesRoot, tCertsRoot))
	if err := sc.SetAutoRepair(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if _, err := sc.Scan(context.Background()); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(fo.calls) == 0 {
		t.Fatal("auto-repair tetiklenmedi")
	}
	open, _ := st.ListOpenDriftEvents(context.Background())
	if len(open) != 0 {
		t.Fatalf("repair sonrası açık drift kaldı: %+v", open)
	}
}

// Auto-repair kapalıyken (varsayılan) onarım YAPILMAMALI.
func TestScanNoAutoRepairByDefault(t *testing.T) {
	st := testStore(t)
	d := cleanDesired(t, st)
	fc := newFakeCollector()
	fc.sync(d)
	fc.vhost["site001"] = ""
	fo := &fakeRepairOps{}

	sc := NewScanner(st, fc, tSitesRoot, tCertsRoot, audit.New(st), NewRepairer(st, fo, tSitesRoot, tCertsRoot))
	if _, err := sc.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fo.calls) != 0 {
		t.Fatal("politika kapalıyken repair yapıldı")
	}
}
