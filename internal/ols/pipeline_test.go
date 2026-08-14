package ols

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeInstaller, dosya sistemini hafızada tutan Installer.
type fakeInstaller struct {
	mu        sync.Mutex
	bundles   map[string]map[string]string // site → dosya → içerik
	calls     []string
	concurrent int
	maxConc    int
	slow       time.Duration

	failRead    bool
	failInstall bool
	failTest    bool
	failReload  bool
	failRemove  bool
	failRollbackTest bool // rollback doğrulamasında TestConfig başarısızlığı
	testCalls   int
}

func newFakeInstaller() *fakeInstaller {
	return &fakeInstaller{bundles: map[string]map[string]string{}}
}

func (f *fakeInstaller) enter() {
	f.mu.Lock()
	f.concurrent++
	if f.concurrent > f.maxConc {
		f.maxConc = f.concurrent
	}
}

func (f *fakeInstaller) exit() {
	f.concurrent--
	f.mu.Unlock()
}

func (f *fakeInstaller) ReadBundle(ctx context.Context, site string) ([]Artifact, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "read")
	if f.failRead {
		return nil, fmt.Errorf("read hata")
	}
	out := []Artifact{}
	for name, content := range f.bundles[site] {
		out = append(out, Artifact{RelPath: name, Content: []byte(content), Mode: 0o644})
	}
	return out, nil
}

func (f *fakeInstaller) InstallBundle(ctx context.Context, site string, files []Artifact) error {
	f.enter()
	defer f.exit()
	f.calls = append(f.calls, "install")
	if f.slow > 0 {
		time.Sleep(f.slow)
	}
	if f.failInstall {
		return fmt.Errorf("install hata")
	}
	if f.bundles[site] == nil {
		f.bundles[site] = map[string]string{}
	}
	for _, a := range files {
		f.bundles[site][a.RelPath] = string(a.Content)
	}
	return nil
}

func (f *fakeInstaller) RemoveBundle(ctx context.Context, site string, names []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "remove")
	if f.failRemove {
		return fmt.Errorf("remove hata")
	}
	for _, n := range names {
		delete(f.bundles[site], n)
	}
	return nil
}

func (f *fakeInstaller) TestConfig(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.testCalls++
	f.calls = append(f.calls, "test")
	// Gerçek dünya modeli: bozuk config İLK testte patlar; rollback sonrası
	// config eski (sağlam) hâline döndüğü için rollback testi geçer —
	// failRollbackTest bunu ayrıca simüle etmek içindir.
	if f.failRollbackTest {
		return fmt.Errorf("test hata")
	}
	if f.failTest && f.testCalls == 1 {
		return fmt.Errorf("test hata")
	}
	return nil
}

func (f *fakeInstaller) Reload(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "reload")
	if f.failReload {
		return fmt.Errorf("reload hata")
	}
	return nil
}

type fakeProber struct {
	fail bool
	last ProbeSpec
}

func (p *fakeProber) Probe(ctx context.Context, spec ProbeSpec) error {
	p.last = spec
	if p.fail {
		return fmt.Errorf("probe hata")
	}
	return nil
}

func TestApplyHappyPath(t *testing.T) {
	fi := newFakeInstaller()
	pr := &fakeProber{}
	p := NewPipeline(testSitesRoot, testCertsRoot, fi, pr)

	if err := p.Apply(context.Background(), validVhost()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	wantCalls := "read,install,test,reload"
	if got := strings.Join(fi.calls[:4], ","); got != wantCalls {
		t.Fatalf("çağrı sırası: %s (beklenen %s)", got, wantCalls)
	}
	if len(fi.calls) != 4 {
		t.Fatalf("rollback çağrıları olmamalı: %v", fi.calls)
	}
	if pr.last.Host != "example.com" || pr.last.Addr != "127.0.0.1:80" || pr.last.TLS {
		t.Fatalf("probe spec hatalı: %+v", pr.last)
	}
}

// Kasıtlı bozuk config (test aşaması): rollback eski içeriği geri getirmeli.
func TestApplyRollbackOnTestFailure(t *testing.T) {
	fi := newFakeInstaller()
	fi.bundles["site001"] = map[string]string{"vhconf.conf": "# eski vhost"}
	fi.failTest = true
	pr := &fakeProber{}
	p := NewPipeline(testSitesRoot, testCertsRoot, fi, pr)

	err := p.Apply(context.Background(), validVhost())
	if err == nil {
		t.Fatal("hata bekleniyordu")
	}
	if !strings.Contains(err.Error(), "rollback uygulandı") {
		t.Fatalf("rollback raporlanmadı: %v", err)
	}
	if got := fi.bundles["site001"]["vhconf.conf"]; got != "# eski vhost" {
		t.Fatalf("eski içerik geri gelmedi: %q", got)
	}
}

// Yeni site (boş snapshot) + health check başarısızlığı:
// yeni oluşturulan dosyalar tamamen kaldırılmalı.
func TestApplyRollbackRemovesNewFiles(t *testing.T) {
	fi := newFakeInstaller()
	pr := &fakeProber{fail: true}
	p := NewPipeline(testSitesRoot, testCertsRoot, fi, pr)

	if err := p.Apply(context.Background(), validVhost()); err == nil {
		t.Fatal("hata bekleniyordu")
	}
	if len(fi.bundles["site001"]) != 0 {
		t.Fatalf("yeni dosyalar kaldırılmadı: %v", fi.bundles["site001"])
	}
}

// Reload başarısızlığı da rollback'e girmeli.
func TestApplyRollbackOnReloadFailure(t *testing.T) {
	fi := newFakeInstaller()
	fi.failReload = true
	p := NewPipeline(testSitesRoot, testCertsRoot, fi, &fakeProber{})
	err := p.Apply(context.Background(), validVhost())
	if err == nil || !strings.Contains(err.Error(), "rollback") {
		t.Fatalf("reload hatası rollback'siz raporlandı: %v", err)
	}
}

// Snapshot alınamazsa HİÇBİR değişiklik yapılmamalı.
func TestApplyNoMutationWhenSnapshotFails(t *testing.T) {
	fi := newFakeInstaller()
	fi.failRead = true
	p := NewPipeline(testSitesRoot, testCertsRoot, fi, &fakeProber{})
	err := p.Apply(context.Background(), validVhost())
	if err == nil {
		t.Fatal("hata bekleniyordu")
	}
	if len(fi.calls) != 1 || fi.calls[0] != "read" {
		t.Fatalf("read dışında çağrı yapıldı: %v", fi.calls)
	}
}

// Rollback doğrulaması da başarısız olursa KRİTİK durum raporlanmalı.
func TestApplyRollbackVerificationFailure(t *testing.T) {
	fi := newFakeInstaller()
	fi.failTest = true
	fi.failRollbackTest = true
	p := NewPipeline(testSitesRoot, testCertsRoot, fi, &fakeProber{})
	err := p.Apply(context.Background(), validVhost())
	if err == nil || !strings.Contains(err.Error(), "ROLLBACK BAŞARISIZ") {
		t.Fatalf("kritik rollback hatası raporlanmadı: %v", err)
	}
}

// Eş zamanlı Apply'lar serileştirilmeli (OLS'e tek müdahale).
func TestApplySerialized(t *testing.T) {
	fi := newFakeInstaller()
	fi.slow = 50 * time.Millisecond
	p := NewPipeline(testSitesRoot, testCertsRoot, fi, &fakeProber{})

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.Apply(context.Background(), validVhost())
		}()
	}
	wg.Wait()
	if fi.maxConc != 1 {
		t.Fatalf("install eş zamanlı çalıştı (max=%d)", fi.maxConc)
	}
}

// Remove: dosyalar kaldırılır, test + reload çalışır.
func TestRemoveHappy(t *testing.T) {
	fi := newFakeInstaller()
	fi.bundles["site001"] = map[string]string{"vhconf.conf": "# eski"}
	p := NewPipeline(testSitesRoot, testCertsRoot, fi, &fakeProber{})

	if err := p.Remove(context.Background(), "site001"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(fi.bundles["site001"]) != 0 {
		t.Fatalf("bundle kaldırılmadı: %v", fi.bundles["site001"])
	}
	want := "read,remove,test,reload"
	if got := strings.Join(fi.calls[:4], ","); got != want {
		t.Fatalf("sıra: %s (beklenen %s)", got, want)
	}
}

// Remove sonrası config testi başarısız olursa snapshot geri yüklenmeli
// (site hizmette kalmalı).
func TestRemoveRollbackOnTestFailure(t *testing.T) {
	fi := newFakeInstaller()
	fi.bundles["site001"] = map[string]string{"vhconf.conf": "# eski"}
	fi.failTest = true
	p := NewPipeline(testSitesRoot, testCertsRoot, fi, &fakeProber{})

	err := p.Remove(context.Background(), "site001")
	if err == nil || !strings.Contains(err.Error(), "rollback") {
		t.Fatalf("rollback raporlanmadı: %v", err)
	}
	if got := fi.bundles["site001"]["vhconf.conf"]; got != "# eski" {
		t.Fatalf("eski vhost geri gelmedi: %q", got)
	}
}

// SSL'li vhost'ta probe 443 + TLS olmalı.
func TestProbeForSSL(t *testing.T) {
	fi := newFakeInstaller()
	pr := &fakeProber{}
	p := NewPipeline(testSitesRoot, testCertsRoot, fi, pr)
	v := validVhost()
	v.SSL = &SSLConfig{
		CertPath: testCertsRoot + "/example.com/fullchain.pem",
		KeyPath:  testCertsRoot + "/example.com/privkey.pem",
	}
	if err := p.Apply(context.Background(), v); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if pr.last.Addr != "127.0.0.1:443" || !pr.last.TLS {
		t.Fatalf("SSL probe hatalı: %+v", pr.last)
	}
}
