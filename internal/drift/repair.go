package drift

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mkoyazilim/aurapanel/internal/ols"
	"github.com/mkoyazilim/aurapanel/internal/site"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// repairOps, Repair'in ihtiyaç duyduğu yeniden-uygulama işlemleri.
type repairOps interface {
	UserExists(ctx context.Context, name string) (bool, error)
	UserCreate(ctx context.Context, name, shell string) error
	SitePrepare(ctx context.Context, siteID, user string) error
	CgroupLimits(ctx context.Context, siteID string, l site.Limits) error
	QuotaSet(ctx context.Context, user string, diskMB, inodes uint64) error
	VhostApply(ctx context.Context, v ols.Vhost) error
}

// Repairer, bir sitenin desired state'ini yeniden uygular (Repair).
// Adımlar idempotenttir: kısmi onarım sonrası Repair yeniden çağrılabilir.
type Repairer struct {
	store     *store.Store
	ops       repairOps
	sitesRoot string
	certsRoot string
}

// NewRepairer, Repairer oluşturur.
func NewRepairer(st *store.Store, ops repairOps, sitesRoot, certsRoot string) *Repairer {
	return &Repairer{store: st, ops: ops, sitesRoot: sitesRoot, certsRoot: certsRoot}
}

// Repair, siteyi desired state'e döndürür: site.prepare → cgroup.limits →
// quota.set → vhost.Apply (rollback güvenceli). Başarıda sitenin açık
// drift kayıtları çözülmüş işaretlenir; doğrulama sonraki taramada yapılır.
func (r *Repairer) Repair(ctx context.Context, siteID string) error {
	st, err := r.store.GetSite(ctx, siteID)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("site yok: %s", siteID)
	}

	var limits site.Limits
	if err := json.Unmarshal([]byte(st.Limits), &limits); err != nil {
		return fmt.Errorf("limits: %w", err)
	}

	domains, err := r.store.ListDomainsBySite(ctx, siteID)
	if err != nil {
		return err
	}
	aliases := []string{}
	for _, d := range domains {
		if d.Kind == "alias" {
			aliases = append(aliases, d.Domain)
		}
	}
	var phpVersion string
	if st.PHPVersionID.Valid {
		phpVersion, err = r.store.GetPHPVersion(ctx, st.PHPVersionID.Int64)
		if err != nil {
			return err
		}
	}

	user := st.LinuxUser
	steps := []struct {
		name string
		fn   func() error
	}{
		{"user.create", func() error {
			exists, err := r.ops.UserExists(ctx, user)
			if err != nil {
				return err
			}
			if !exists {
				return r.ops.UserCreate(ctx, user, "/usr/sbin/nologin")
			}
			return nil
		}},
		{"site.prepare", func() error { return r.ops.SitePrepare(ctx, siteID, user) }},
		{"cgroup.limits", func() error { return r.ops.CgroupLimits(ctx, siteID, limits) }},
		{"quota.set", func() error { return r.ops.QuotaSet(ctx, user, limits.DiskMB, limits.Inodes) }},
		{"vhost.apply", func() error {
			return r.ops.VhostApply(ctx, ols.Vhost{
				SiteID:     siteID,
				Domain:     st.Name,
				Aliases:    aliases,
				PHPVersion: phpVersion,
				IndexFiles: []string{"index.php", "index.html"},
			})
		}},
	}
	for _, step := range steps {
		if err := step.fn(); err != nil {
			return fmt.Errorf("repair %s: %w", step.name, err)
		}
	}

	return r.store.ResolveDriftEvents(ctx, siteID)
}
