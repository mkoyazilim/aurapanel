package drift

import (
	"context"
	"os"
	"path"
	"strings"

	"github.com/mkoyazilim/aurapanel/internal/ols"
	"github.com/mkoyazilim/aurapanel/internal/privclient"
	"github.com/mkoyazilim/aurapanel/internal/site"
)

// privCollector, actualCollector + repairOps arayüzlerinin priv helper
// üzerinden üretim uygulamasıdır.
type privCollector struct {
	c         *privclient.Client
	sitesRoot string
	certsRoot string
}

// NewPrivCollector, privCollector oluşturur.
func NewPrivCollector(c *privclient.Client, sitesRoot, certsRoot string) *privCollector {
	return &privCollector{c: c, sitesRoot: sitesRoot, certsRoot: certsRoot}
}

func (p *privCollector) VhostBundle(ctx context.Context, siteID string) (string, error) {
	data, err := p.c.Call(ctx, "ols.read_bundle", map[string]any{"site": siteID})
	if err != nil {
		return "", err
	}
	raw, _ := data["files"].([]any)
	for _, f := range raw {
		m, _ := f.(map[string]any)
		if name, _ := m["name"].(string); name == "vhconf.conf" {
			content, _ := m["content"].(string)
			return content, nil
		}
	}
	return "", nil // bundle yok → boş vhost
}

func (p *privCollector) UserExists(ctx context.Context, name string) (bool, error) {
	data, err := p.c.Call(ctx, "user.exists", map[string]any{"name": name})
	if err != nil {
		return false, err
	}
	exists, _ := data["exists"].(bool)
	return exists, nil
}

func (p *privCollector) CgroupRead(ctx context.Context, siteID string) (map[string]string, error) {
	data, err := p.c.Call(ctx, "cgroup.read", map[string]any{"site": siteID})
	if err != nil {
		return nil, err
	}
	raw, _ := data["values"].(map[string]any)
	out := map[string]string{}
	for k, v := range raw {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out, nil
}

func (p *privCollector) QuotaGet(ctx context.Context, name string) (quotaActual, error) {
	data, err := p.c.Call(ctx, "quota.get", map[string]any{"user": name})
	if err != nil {
		return quotaActual{}, err
	}
	out := quotaActual{}
	out.Available, _ = data["available"].(bool)
	if !out.Available {
		out.Reason, _ = data["reason"].(string)
		return out, nil
	}
	if f, ok := data["disk_blocks"].(float64); ok {
		out.DiskBlocks = uint64(f)
	}
	if f, ok := data["inodes"].(float64); ok {
		out.Inodes = uint64(f)
	}
	return out, nil
}

func (p *privCollector) SiteStatus(ctx context.Context, siteID, name string) (map[string]dirActual, error) {
	data, err := p.c.Call(ctx, "site.status", map[string]any{"site": siteID, "user": name})
	if err != nil {
		return nil, err
	}
	raw, _ := data["dirs"].(map[string]any)
	out := map[string]dirActual{}
	for d, v := range raw {
		m, _ := v.(map[string]any)
		exists, _ := m["exists"].(bool)
		da := dirActual{Exists: exists}
		if mode, ok := m["mode"].(float64); ok {
			da.Mode = os.FileMode(int(mode))
		}
		out[d] = da
	}
	return out, nil
}

// --- repairOps uygulaması ---

func (p *privCollector) UserCreate(ctx context.Context, name, shell string) error {
	siteID := strings.TrimPrefix(name, "www-")
	// ops.go kuralı: home = sitesRoot/<siteID>/home (site dizininin altındaki "home" alt dizini)
	home := path.Join(p.sitesRoot, siteID, "home")
	_, err := p.c.Call(ctx, "user.create", map[string]any{"name": name, "shell": shell, "home": home})
	return err
}

func (p *privCollector) SitePrepare(ctx context.Context, siteID, user string) error {
	_, err := p.c.Call(ctx, "site.prepare", map[string]any{"site": siteID, "user": user})
	return err
}

func (p *privCollector) CgroupLimits(ctx context.Context, siteID string, l site.Limits) error {
	_, err := p.c.Call(ctx, "cgroup.limits", map[string]any{
		"site": siteID, "cpu_max": l.CPUMax,
		"memory_max": l.MemoryMax, "memory_high": l.MemoryHigh, "pids_max": l.PIDsMax,
	})
	return err
}

func (p *privCollector) QuotaSet(ctx context.Context, user string, diskMB, inodes uint64) error {
	_, err := p.c.Call(ctx, "quota.set", map[string]any{"user": user, "disk_mb": diskMB, "inodes": inodes})
	return err
}

func (p *privCollector) VhostApply(ctx context.Context, v ols.Vhost) error {
	// Vhost uygulaması güvenli pipeline üzerinden yapılır (rollback'li).
	// Prober nil: health probe atlanır — doğrulama bir sonraki drift
	// taramasında içerik karşılaştırmasıyla yapılır.
	pipeline := ols.NewPipeline(p.sitesRoot, p.certsRoot, ols.NewPrivInstaller(p.c), nil)
	return pipeline.Apply(ctx, v)
}
