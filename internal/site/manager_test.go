package site

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/ols"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

const testSitesRoot = "/srv/aurapanel/sites"

// --- Sahte priv ops ---

type fakePriv struct {
	calls  []string
	failAt map[string]bool
	users  map[string]bool
}

func newFakePriv() *fakePriv {
	return &fakePriv{failAt: map[string]bool{}, users: map[string]bool{}}
}

func (f *fakePriv) do(op string) error {
	f.calls = append(f.calls, op)
	if f.failAt[op] {
		return fmt.Errorf("%s hata", op)
	}
	return nil
}

func (f *fakePriv) UserCreate(ctx context.Context, name, home string) error {
	if err := f.do("user.create"); err != nil {
		return err
	}
	f.users[name] = true
	return nil
}

func (f *fakePriv) UserDelete(ctx context.Context, name string) error {
	if err := f.do("user.delete"); err != nil {
		return err
	}
	delete(f.users, name)
	return nil
}

func (f *fakePriv) UserExists(ctx context.Context, name string) (bool, error) {
	f.calls = append(f.calls, "user.exists")
	return f.users[name], nil
}

func (f *fakePriv) SitePrepare(ctx context.Context, siteID, user string) error {
	return f.do("site.prepare")
}

func (f *fakePriv) SiteTeardown(ctx context.Context, siteID, user string) error {
	return f.do("site.teardown")
}

func (f *fakePriv) CgroupLimits(ctx context.Context, siteID string, l Limits) error {
	return f.do("cgroup.limits")
}

func (f *fakePriv) CgroupCleanup(ctx context.Context, siteID string) error {
	return f.do("cgroup.cleanup")
}

func (f *fakePriv) QuotaSet(ctx context.Context, user string, diskMB, inodes uint64) error {
	return f.do("quota.set")
}

// --- Sahte vhost uygulayıcı ---

type fakeVhost struct {
	calls   []string
	applied map[string]bool
	failAt  map[string]bool
}

func newFakeVhost() *fakeVhost {
	return &fakeVhost{applied: map[string]bool{}, failAt: map[string]bool{}}
}

func (f *fakeVhost) Apply(ctx context.Context, v ols.Vhost) error {
	f.calls = append(f.calls, "vhost.apply")
	if f.failAt["vhost.apply"] {
		return fmt.Errorf("vhost.apply hata")
	}
	f.applied[v.SiteID] = true
	return nil
}

func (f *fakeVhost) Remove(ctx context.Context, siteID string) error {
	f.calls = append(f.calls, "vhost.remove")
	if f.failAt["vhost.remove"] {
		return fmt.Errorf("vhost.remove hata")
	}
	delete(f.applied, siteID)
	return nil
}

// --- Yardımcılar ---

func testManager(t *testing.T) (*Manager, *store.Store, *fakePriv, *fakeVhost) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "site.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	// PHP sürüm envanteri (W6: Create artık PHP sürümünü bağlar).
	ctx := context.Background()
	for _, v := range []string{"8.2", "8.3", "8.4"} {
		if _, err := st.InsertPHPVersion(ctx, v, "/usr/local/lsws/lsphp"+strings.ReplaceAll(v, ".", "")+"/bin/lsphp"); err != nil {
			t.Fatal(err)
		}
	}
	fp := newFakePriv()
	fv := newFakeVhost()
	m := NewManager(st, fp, fv, audit.New(st), testSitesRoot)
	return m, st, fp, fv
}

func defaultReq() CreateRequest {
	return CreateRequest{Domain: "example.com", PHPVersion: "8.3"}
}

// --- Testler ---

func TestCreateHappy(t *testing.T) {
	m, st, fp, fv := testManager(t)
	ctx := context.Background()

	id, err := m.Create(ctx, defaultReq())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id != "example" {
		t.Fatalf("ilk siteID example olmalı, %s geldi", id)
	}
	want := []string{"user.create", "site.prepare", "cgroup.limits", "quota.set"}
	if strings.Join(fp.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("priv adım sırası: %v (beklenen %v)", fp.calls, want)
	}
	if strings.Join(fv.calls, ",") != "vhost.apply" {
		t.Fatalf("vhost adımı: %v", fv.calls)
	}
	s, err := st.GetSite(ctx, id)
	if err != nil || s == nil {
		t.Fatalf("site kaydı: %v %v", s, err)
	}
	if s.Status != "active" || s.LinuxUser != "www-example" {
		t.Fatalf("kayıt hatalı: %+v", s)
	}
	if s.HomeDir != testSitesRoot+"/example/home" {
		t.Fatalf("home hatalı: %s", s.HomeDir)
	}
	if !fv.applied[id] {
		t.Fatal("vhost uygulanmadı")
	}
	// W6: PHP sürüm + pool kaydı oluşturulmuş olmalı.
	if !s.PHPVersionID.Valid {
		t.Fatal("php sürümü bağlanmadı")
	}
	if _, _, ok, err := st.GetPHPool(ctx, id); err != nil || !ok {
		t.Fatalf("php pool kaydı yok: ok=%v err=%v", ok, err)
	}
}

// Her adımda başarısızlık: tamamlanan adımlar ters sırada telafi edilmeli.
func TestCreateCompensationAtEachStep(t *testing.T) {
	cases := []struct {
		failOp  string
		onVhost bool
		want    []string // telafi çağrıları
	}{
		{"user.create", false, nil},
		{"site.prepare", false, []string{"user.delete", "site.teardown"}},
		// cgroup.limits yarıda kesilse de (kısmi yazım) temizlik şarttır.
		{"cgroup.limits", false, []string{"cgroup.cleanup", "user.delete", "site.teardown"}},

		{"vhost.apply", true, []string{"cgroup.cleanup", "user.delete", "site.teardown"}},
	}
	for _, tc := range cases {
		t.Run(tc.failOp, func(t *testing.T) {
			m, st, fp, fv := testManager(t)
			if tc.onVhost {
				fv.failAt[tc.failOp] = true
			} else {
				fp.failAt[tc.failOp] = true
			}

			if _, err := m.Create(context.Background(), defaultReq()); err == nil {
				t.Fatal("hata bekleniyordu")
			}
			// Telafi çağrıları, başarısız adımın SONRASINA eklenir.
			var got []string
			if tc.onVhost {
				got = fp.calls[len(fp.calls)-len(tc.want):]
			} else {
				i := lastIndex(fp.calls, tc.failOp)
				got = fp.calls[i+1:]
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("telafi: %v (beklenen %v)", got, tc.want)
			}
			if len(fp.users) != 0 {
				t.Fatal("kullanıcı kalıntısı var")
			}
			s, _ := st.GetSite(context.Background(), "example")
			if s == nil || s.Status != "failed" {
				t.Fatalf("site failed durumunda olmalı: %+v", s)
			}
		})
	}
}

func lastIndex(list []string, needle string) int {
	for i := len(list) - 1; i >= 0; i-- {
		if list[i] == needle {
			return i
		}
	}
	return -1
}

func TestCreateValidationRejects(t *testing.T) {
	m, st, fp, _ := testManager(t)
	for _, req := range []CreateRequest{
		{Domain: "BAD domain", PHPVersion: "8.3"},
		{Domain: "example.com", PHPVersion: "8.9"},
		{Domain: "example.com", PHPVersion: "8.3", Aliases: []string{"kötü alias"}},
	} {
		if _, err := m.Create(context.Background(), req); err == nil {
			t.Errorf("reddedilmedi: %+v", req)
		}
	}
	if len(fp.calls) != 0 {
		t.Fatalf("doğrulama hatasında priv çağrısı yapıldı: %v", fp.calls)
	}
	if s, _ := st.GetSite(context.Background(), "example"); s != nil {
		t.Fatal("doğrulama hatasında DB kaydı oluştu")
	}
}

func TestDeleteHappy(t *testing.T) {
	m, st, fp, fv := testManager(t)
	ctx := context.Background()
	id, err := m.Create(ctx, defaultReq())
	if err != nil {
		t.Fatal(err)
	}
	fp.calls = nil
	fv.calls = nil

	if err := m.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if strings.Join(fv.calls, ",") != "vhost.remove" {
		t.Fatalf("vhost kaldırma: %v", fv.calls)
	}
	want := []string{"cgroup.cleanup", "user.exists", "user.delete", "site.teardown"}
	if strings.Join(fp.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("silme sırası: %v (beklenen %v)", fp.calls, want)
	}
	if s, _ := st.GetSite(ctx, id); s != nil {
		t.Fatal("site kaydı silinmedi")
	}
	if fv.applied[id] {
		t.Fatal("vhost kaldırılmadı")
	}
}

// Yeniden deneme: kullanıcı zaten silinmişse userdel atlanmalı.
func TestDeleteRetryWithMissingUser(t *testing.T) {
	m, st, fp, _ := testManager(t)
	ctx := context.Background()
	id, err := m.Create(ctx, defaultReq())
	if err != nil {
		t.Fatal(err)
	}
	delete(fp.users, "www-"+id) // kullanıcı dışarıdan silinmiş gibi
	fp.calls = nil

	if err := m.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if strings.Contains(strings.Join(fp.calls, ","), "user.delete") {
		t.Fatal("olmayan kullanıcı için userdel çağrıldı")
	}
	if s, _ := st.GetSite(ctx, id); s != nil {
		t.Fatal("site silinmedi")
	}
}

// Başarısız silme: "deleting" durumunda kal, retry ile tamamlanabilsin.
func TestDeleteFailureThenRetry(t *testing.T) {
	m, st, fp, _ := testManager(t)
	ctx := context.Background()
	id, err := m.Create(ctx, defaultReq())
	if err != nil {
		t.Fatal(err)
	}
	fp.failAt["cgroup.cleanup"] = true

	if err := m.Delete(ctx, id); err == nil {
		t.Fatal("hata bekleniyordu")
	}
	s, _ := st.GetSite(ctx, id)
	if s == nil || s.Status != "deleting" {
		t.Fatalf("site deleting durumunda kalmalı: %+v", s)
	}

	fp.failAt["cgroup.cleanup"] = false
	if err := m.Delete(ctx, id); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if s, _ := st.GetSite(ctx, id); s != nil {
		t.Fatal("retry sonrası site silinmedi")
	}
}

func TestUpdateLimits(t *testing.T) {
	m, st, fp, _ := testManager(t)
	ctx := context.Background()
	id, err := m.Create(ctx, defaultReq())
	if err != nil {
		t.Fatal(err)
	}
	fp.calls = nil

	big := DefaultLimits()
	big.DiskMB = 10240
	big.MemoryMax = 2 << 30
	if err := m.UpdateLimits(ctx, id, big); err != nil {
		t.Fatalf("UpdateLimits: %v", err)
	}
	if got := strings.Join(fp.calls, ","); got != "cgroup.limits,quota.set" {
		t.Fatalf("çağrılar: %v", got)
	}
	s, _ := st.GetSite(ctx, id)
	if !strings.Contains(s.Limits, `"disk_mb":10240`) {
		t.Fatalf("DB limitleri güncellenmedi: %s", s.Limits)
	}

	// Kısmi güncelleme reddedilmeli.
	if err := m.UpdateLimits(ctx, id, Limits{DiskMB: 999}); err == nil {
		t.Fatal("kısmi limit güncellemesi kabul edildi")
	}
}

func TestFeatureFlags(t *testing.T) {
	m, st, _, _ := testManager(t)
	ctx := context.Background()
	id, err := m.Create(ctx, defaultReq())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetFeatureFlags(ctx, id, map[string]bool{"php": true, "database": false}); err != nil {
		t.Fatalf("SetFeatureFlags: %v", err)
	}
	s, _ := st.GetSite(ctx, id)
	if !strings.Contains(s.FeatureFlags, `"database":false`) {
		t.Fatalf("bayraklar kaydedilmedi: %s", s.FeatureFlags)
	}
	if err := m.SetFeatureFlags(ctx, id, map[string]bool{"bittorrent": true}); err == nil {
		t.Fatal("bilinmeyen özellik kabul edildi")
	}
}
