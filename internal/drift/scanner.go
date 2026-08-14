package drift

import (
	"context"
	"fmt"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// autoRepairSetting, auto-reconcile politikasının system_settings anahtarı.
const autoRepairSetting = "drift.auto_repair"

// Scanner, desired state ile gerçek sistemi periyodik/isteğe bağlı
// karşılaştırır (ARCHITECTURE §6). Sonuçlar drift_events tablosuna
// yazılır; auto-repair açıksa sapma tespitinde Repair tetiklenir.
type Scanner struct {
	store     *store.Store
	collect   actualCollector
	sitesRoot string
	certsRoot string
	audit     *audit.Service
	repairer  *Repairer
}

// NewScanner, Scanner oluşturur. repairer nil ise auto-repair devre dışıdır.
func NewScanner(st *store.Store, c actualCollector, sitesRoot, certsRoot string, au *audit.Service, repairer *Repairer) *Scanner {
	return &Scanner{store: st, collect: c, sitesRoot: sitesRoot, certsRoot: certsRoot, audit: au, repairer: repairer}
}

// Scan, tüm aktif siteleri bir kez tarar ve bulunan toplam sapma sayısını
// döndürür. Tarama hataları site başına audit'e yazılır ve tarama devam eder.
func (s *Scanner) Scan(ctx context.Context) (int, error) {
	sites, err := s.store.ListSites(ctx)
	if err != nil {
		return 0, err
	}

	// Silinen sitelerden arta kalan drift kayıtlarını temizle.
	if err := s.store.CleanOrphanedEvents(ctx); err != nil {
		s.audit.Write(ctx, audit.Event{
			Action: "drift.cleanup", Target: "system", Result: "failed",
			Extra: map[string]any{"error": err.Error()},
		})
	}

	total := 0
	for _, st := range sites {
		if st.Status != "active" {
			continue
		}
		n, err := s.scanSite(ctx, &st)
		if err != nil {
			s.audit.Write(ctx, audit.Event{
				Action: "drift.scan", Target: st.ID, Result: "failed",
				Extra: map[string]any{"error": err.Error()},
			})
			continue
		}
		total += n
	}
	return total, nil
}

// scanSite, tek siteyi tarar: bulunan sapmaları kaydeder ve gerekirse
// auto-repair'i tetikler.
func (s *Scanner) scanSite(ctx context.Context, st *store.Site) (int, error) {
	// Desired state: domainler + PHP sürümü store'dan.
	domains, err := s.store.ListDomainsBySite(ctx, st.ID)
	if err != nil {
		return 0, err
	}
	var phpVersion string
	if st.PHPVersionID.Valid {
		phpVersion, err = s.store.GetPHPVersion(ctx, st.PHPVersionID.Int64)
		if err != nil {
			return 0, err
		}
	}
	desired, err := desiredFor(st, domains, phpVersion, s.sitesRoot, s.certsRoot)
	if err != nil {
		return 0, err
	}

	// Gerçek durum.
	vhostContent, err := s.collect.VhostBundle(ctx, st.ID)
	if err != nil {
		return 0, err
	}
	userExists, err := s.collect.UserExists(ctx, st.LinuxUser)
	if err != nil {
		return 0, err
	}
	cgroup, err := s.collect.CgroupRead(ctx, st.ID)
	if err != nil {
		return 0, err
	}
	quota, err := s.collect.QuotaGet(ctx, st.LinuxUser)
	if err != nil {
		return 0, err
	}
	dirs, err := s.collect.SiteStatus(ctx, st.ID, st.LinuxUser)
	if err != nil {
		return 0, err
	}

	findings := diff(desired, actuals{
		VhostContent: vhostContent,
		UserExists:   userExists,
		Cgroup:       cgroup,
		Quota:        quota,
		Dirs:         dirs,
	})
	for _, f := range findings {
		if _, err := s.store.InsertDriftEvent(ctx, store.DriftEvent{
			SiteID: st.ID, Resource: f.Resource,
			Expected: f.Expected, Actual: f.Actual, Severity: f.Severity,
		}); err != nil {
			return len(findings), err
		}
	}
	if len(findings) > 0 {
		s.audit.Write(ctx, audit.Event{
			Action: "drift.detected", Target: st.ID, Result: "success",
			Extra: map[string]any{"count": len(findings)},
		})
	}

	// Auto-repair: politika açıksa ve repairer bağlıysa dene.
	if len(findings) > 0 && s.repairer != nil {
		on, err := s.autoRepairEnabled(ctx)
		if err != nil {
			return len(findings), err
		}
		if on {
			if err := s.repairer.Repair(ctx, st.ID); err != nil {
				s.audit.Write(ctx, audit.Event{
					Action: "drift.auto_repair", Target: st.ID, Result: "failed",
					Extra: map[string]any{"error": err.Error()},
				})
			} else {
				s.audit.Write(ctx, audit.Event{
					Action: "drift.auto_repair", Target: st.ID, Result: "success",
				})
			}
		}
	}
	return len(findings), nil
}

// autoRepairEnabled, system_settings politikasını okur (varsayılan: kapalı).
func (s *Scanner) autoRepairEnabled(ctx context.Context) (bool, error) {
	v, ok, err := s.store.GetSetting(ctx, autoRepairSetting)
	if err != nil {
		return false, err
	}
	return ok && v == "true", nil
}

// SetAutoRepair, auto-reconcile politikasını yazar (true/false).
func (s *Scanner) SetAutoRepair(ctx context.Context, enabled bool) error {
	v := "false"
	if enabled {
		v = "true"
	}
	if err := s.store.SetSetting(ctx, autoRepairSetting, v); err != nil {
		return fmt.Errorf("auto-repair ayarı: %w", err)
	}
	s.audit.Write(ctx, audit.Event{
		Action: "drift.auto_repair.policy", Target: "panel",
		Extra: map[string]any{"enabled": enabled},
	})
	return nil
}
