// Package site, site yaşam döngüsünü yönetir (ROADMAP W4).
//
// Oluşturma: user + dizinler + cgroup + quota + vhost zinciri — her
// adımda telafi (compensation): yarıda kalan site ASLA yarım durumda
// bırakılmaz. Silme: ters sırada, yeniden denenebilir (idempotent).
package site

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/ols"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// Limits, site kaynak sınırları (ARCHITECTURE §7).
type Limits struct {
	CPUMax     string `json:"cpu_max"`
	MemoryMax  uint64 `json:"memory_max"`
	MemoryHigh uint64 `json:"memory_high"`
	PIDsMax    uint64 `json:"pids_max"`
	DiskMB     uint64 `json:"disk_mb"`
	Inodes     uint64 `json:"inodes"`
}

// DefaultLimits, yeni sitelerin varsayılan limitleri (5 GB disk,
// 1 çekirdek, 1 GiB RAM). Panelden site başına değiştirilebilir.
func DefaultLimits() Limits {
	return Limits{
		CPUMax:     "100000",
		MemoryMax:  1 << 30,
		MemoryHigh: 512 << 20,
		PIDsMax:    512,
		DiskMB:     5120,
		Inodes:     200000,
	}
}

// withDefaults, eksik alanları varsayılanlarla doldurur (yalnızca Create'te).
func (l Limits) withDefaults() Limits {
	d := DefaultLimits()
	if l.CPUMax == "" {
		l.CPUMax = d.CPUMax
	}
	if l.MemoryMax == 0 {
		l.MemoryMax = d.MemoryMax
	}
	if l.MemoryHigh == 0 {
		l.MemoryHigh = d.MemoryHigh
	}
	if l.PIDsMax == 0 {
		l.PIDsMax = d.PIDsMax
	}
	if l.DiskMB == 0 {
		l.DiskMB = d.DiskMB
	}
	if l.Inodes == 0 {
		l.Inodes = d.Inodes
	}
	return l
}

// validateComplete, UpdateLimits için tam belirtim ister: kısmi
// güncellemede varsayılanların üzerine yazılması engellenir.
func (l Limits) validateComplete() error {
	if l.CPUMax == "" || l.MemoryMax == 0 || l.MemoryHigh == 0 || l.PIDsMax == 0 || l.DiskMB == 0 || l.Inodes == 0 {
		return fmt.Errorf("limitler tam belirtilmeli (cpu_max, memory_max, memory_high, pids_max, disk_mb, inodes)")
	}
	return nil
}

// CreateRequest, yeni site isteği.
type CreateRequest struct {
	Domain     string
	Aliases    []string
	PHPVersion string
	Limits     Limits
}

// Manager, site yaşam döngüsünü düzenler. Tüm dış sistemler arayüzler
// üzerinden gelir — birim testleri sahte uygulamalarla çalışır.
type Manager struct {
	store     *store.Store
	priv      privOps
	vhost     vhostApplier
	audit     *audit.Service
	sitesRoot string
}

// privOps, site yaşam döngüsünün ihtiyaç duyduğu priv helper işlemleri.
type privOps interface {
	UserCreate(ctx context.Context, name, home string) error
	UserDelete(ctx context.Context, name string) error
	UserExists(ctx context.Context, name string) (bool, error)
	SitePrepare(ctx context.Context, siteID, user string) error
	SiteTeardown(ctx context.Context, siteID, user string) error
	CgroupLimits(ctx context.Context, siteID string, l Limits) error
	CgroupCleanup(ctx context.Context, siteID string) error
	QuotaSet(ctx context.Context, user string, diskMB, inodes uint64) error
}

// vhostApplier, OLS vhost uygulama/kaldırma (ols.Pipeline bunu sağlar).
type vhostApplier interface {
	Apply(ctx context.Context, v ols.Vhost) error
	Remove(ctx context.Context, siteID string) error
}

// NewManager, Manager oluşturur.
func NewManager(st *store.Store, p privOps, v vhostApplier, au *audit.Service, sitesRoot string) *Manager {
	return &Manager{store: st, priv: p, vhost: v, audit: au, sitesRoot: sitesRoot}
}

// Create, izole bir siteyi uçtan uca kurar. Başarısızlıkta tamamlanan
// adımlar ters sırada telafi edilir ve site "failed" durumunda kalır.
func (m *Manager) Create(ctx context.Context, req CreateRequest) (string, error) {
	if err := ols.ValidateDomain(req.Domain); err != nil {
		return "", err
	}
	for _, a := range req.Aliases {
		if err := ols.ValidateDomain(a); err != nil {
			return "", err
		}
	}
	if !ols.SupportedPHP[req.PHPVersion] {
		return "", fmt.Errorf("desteklenmeyen PHP sürümü: %q", req.PHPVersion)
	}
	limits := req.Limits.withDefaults()

	siteID, err := m.store.NextSiteID(ctx)
	if err != nil {
		return "", err
	}
	user := "www-" + siteID
	home := path.Join(m.sitesRoot, siteID, "home")

	limitsJSON, err := json.Marshal(limits)
	if err != nil {
		return "", err
	}
	// DB kaydı önce: süreç izlenebilir olsun (durum makinesi).
	if err := m.store.InsertSite(ctx, store.Site{
		ID: siteID, Name: req.Domain, LinuxUser: user, HomeDir: home,
		Status: "creating", FeatureFlags: `{}`, Limits: string(limitsJSON),
	}); err != nil {
		return "", fmt.Errorf("site kaydı: %w", err)
	}

	auditSite := func(result string, extra map[string]any) {
		m.audit.Write(ctx, audit.Event{
			Action: "site.create", Target: siteID, Result: result,
			Extra: extra,
		})
	}

	// PHP sürüm + pool kaydı (W6): desired state'in parçası. Runtime'a
	// henüz dokunulmadığı için başarısızlıkta telafi gerekmez — site
	// "failed" işaretlenir, drift onarımıyla yinelenebilir.
	phpID, err := m.store.GetPHPVersionID(ctx, req.PHPVersion)
	if err != nil {
		m.store.SetSiteStatus(ctx, siteID, "failed")
		auditSite("failed", map[string]any{"step": "php.bind", "error": err.Error()})
		return "", fmt.Errorf("php sürüm bağlama: %w", err)
	}
	if err := m.store.SetSitePHPVersion(ctx, siteID, phpID); err != nil {
		m.store.SetSiteStatus(ctx, siteID, "failed")
		auditSite("failed", map[string]any{"step": "php.bind", "error": err.Error()})
		return "", fmt.Errorf("php sürüm bağlama: %w", err)
	}
	if err := m.store.UpsertPHPool(ctx, siteID, phpID, `{}`); err != nil {
		m.store.SetSiteStatus(ctx, siteID, "failed")
		auditSite("failed", map[string]any{"step": "php.pool", "error": err.Error()})
		return "", fmt.Errorf("php pool kaydı: %w", err)
	}

	auditSite("", map[string]any{"domain": req.Domain})

	steps := []struct {
		name string
		fn   func() error
	}{
		{"user.create", func() error { return m.priv.UserCreate(ctx, user, home) }},
		{"site.prepare", func() error { return m.priv.SitePrepare(ctx, siteID, user) }},
		{"cgroup.limits", func() error { return m.priv.CgroupLimits(ctx, siteID, limits) }},
		{"quota.set", func() error { return m.priv.QuotaSet(ctx, user, limits.DiskMB, limits.Inodes) }},
		{"vhost.apply", func() error {
			return m.vhost.Apply(ctx, ols.Vhost{
				SiteID:     siteID,
				Domain:     req.Domain,
				Aliases:    req.Aliases,
				PHPVersion: req.PHPVersion,
				IndexFiles: []string{"index.php", "index.html"},
			})
		}},
	}

	for i, step := range steps {
		if err := step.fn(); err != nil {
			m.compensateCreate(ctx, siteID, user, i)
			m.store.SetSiteStatus(ctx, siteID, "failed")
			auditSite("failed", map[string]any{"step": step.name, "error": err.Error()})
			return "", fmt.Errorf("site oluşturma %s adımı: %w", step.name, err)
		}
	}

	if err := m.store.SetSiteStatus(ctx, siteID, "active"); err != nil {
		// Runtime sağlıklı ama izlenemiyor → tutarlılık için telafi et.
		m.compensateCreate(ctx, siteID, user, len(steps))
		auditSite("failed", map[string]any{"step": "db.status", "error": err.Error()})
		return "", fmt.Errorf("site durumu kaydedilemedi: %w", err)
	}
	auditSite("success", nil)
	return siteID, nil
}

// compensateCreate, tamamlanan adımları ters sırada geri alır (best effort;
// her telafi hatası audit'e yazılır). done = başarıyla tamamlanan adım sayısı.
func (m *Manager) compensateCreate(ctx context.Context, siteID, user string, done int) {
	// Adım sırası: 0 user.create, 1 site.prepare, 2 cgroup.limits,
	// 3 quota.set, 4 vhost.apply. vhost başarısızlığında Apply kendi
	// içinde rollback yapmıştır — done>4 yalnızca vhost sonrası hatalarda.
	type comp struct {
		name string
		fn   func() error
	}
	var comps []comp
	// done >= 2: cgroup.limits atomik değildir — yarıda kesilen op kısmi
	// durum bırakabilir; denendiği anda (i=2) temizlik zorunludur.
	if done >= 2 {
		comps = append(comps, comp{"cgroup.cleanup", func() error { return m.priv.CgroupCleanup(ctx, siteID) }})
	}
	if done > 0 {
		comps = append(comps, comp{"user.delete", func() error { return m.priv.UserDelete(ctx, user) }})
		// user.create home dizinini oluşturmuştur; site kökü bütün olarak
		// kaldırılır (teardown eksik dizinlerde de güvenli).
		comps = append(comps, comp{"site.teardown", func() error { return m.priv.SiteTeardown(ctx, siteID, user) }})
	}
	if done > 4 {
		comps = append(comps, comp{"vhost.remove", func() error { return m.vhost.Remove(ctx, siteID) }})
	}
	for _, c := range comps {
		if err := c.fn(); err != nil {
			m.audit.Write(ctx, audit.Event{
				Action: "site.create", Target: siteID, Result: "failed",
				Extra: map[string]any{"compensation": c.name, "error": err.Error()},
			})
		}
	}
}

// Delete, siteyi ters sırada kaldırır. Bir adım başarısız olursa site
// "deleting" durumunda kalır ve Delete yeniden çağrılabilir (adımlar
// idempotenttir: eksik kaynaklar hata üretmez).
func (m *Manager) Delete(ctx context.Context, siteID string) error {
	st, err := m.store.GetSite(ctx, siteID)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("site yok: %s", siteID)
	}
	user := st.LinuxUser

	if err := m.store.SetSiteStatus(ctx, siteID, "deleting"); err != nil {
		return err
	}
	auditDel := func(result string, extra map[string]any) {
		m.audit.Write(ctx, audit.Event{Action: "site.delete", Target: siteID, Result: result, Extra: extra})
	}
	auditDel("", nil)

	steps := []struct {
		name string
		fn   func() error
	}{
		{"vhost.remove", func() error { return m.vhost.Remove(ctx, siteID) }},
		{"cgroup.cleanup", func() error { return m.priv.CgroupCleanup(ctx, siteID) }},
		{"user.delete", func() error {
			exists, err := m.priv.UserExists(ctx, user)
			if err != nil {
				return err
			}
			if !exists {
				return nil // yeniden denemede zaten silinmiş
			}
			return m.priv.UserDelete(ctx, user)
		}},
		{"site.teardown", func() error { return m.priv.SiteTeardown(ctx, siteID, user) }},
	}
	for _, step := range steps {
		if err := step.fn(); err != nil {
			auditDel("failed", map[string]any{"step": step.name, "error": err.Error()})
			return fmt.Errorf("site silme %s adımı: %w", step.name, err)
		}
	}

	if err := m.store.DeleteSite(ctx, siteID); err != nil {
		return err
	}
	auditDel("success", nil)
	return nil
}

// UpdateLimits, site kaynak limitlerini günceller (tam belirtim zorunlu).
func (m *Manager) UpdateLimits(ctx context.Context, siteID string, l Limits) error {
	if err := l.validateComplete(); err != nil {
		return err
	}
	st, err := m.store.GetSite(ctx, siteID)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("site yok: %s", siteID)
	}
	if err := m.priv.CgroupLimits(ctx, siteID, l); err != nil {
		return fmt.Errorf("cgroup: %w", err)
	}
	if err := m.priv.QuotaSet(ctx, st.LinuxUser, l.DiskMB, l.Inodes); err != nil {
		return fmt.Errorf("quota: %w", err)
	}
	limitsJSON, err := json.Marshal(l)
	if err != nil {
		return err
	}
	if err := m.store.UpdateSiteLimits(ctx, siteID, string(limitsJSON)); err != nil {
		return err
	}
	m.audit.Write(ctx, audit.Event{Action: "site.limits.update", Target: siteID, Result: "success"})
	return nil
}

// AllowedFeatures, site özellik anahtarları (ARCHITECTURE §10.1).
var AllowedFeatures = map[string]bool{
	"php": true, "database": true, "cron": true, "sftp": true,
	"mail": true, "node": true, "git": true, "ssh": true,
}

// SetFeatureFlags, site özellik bayraklarını günceller. Kullanılmayan
// runtime'ların bağlanması W6+ paketlerinde uygulanır; burada bayraklar
// desired state olarak kaydedilir.
func (m *Manager) SetFeatureFlags(ctx context.Context, siteID string, flags map[string]bool) error {
	for k := range flags {
		if !AllowedFeatures[k] {
			return fmt.Errorf("bilinmeyen özellik: %q", k)
		}
	}
	b, err := json.Marshal(flags)
	if err != nil {
		return err
	}
	if err := m.store.UpdateSiteFeatureFlags(ctx, siteID, string(b)); err != nil {
		return err
	}
	m.audit.Write(ctx, audit.Event{Action: "site.features.update", Target: siteID, Result: "success"})
	return nil
}
