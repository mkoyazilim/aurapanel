package site

import (
	"context"

	"github.com/mkoyazilim/aurapanel/internal/privclient"
)

// privImpl, privOps arayüzünün aurapanel-priv üzerinden üretim
// uygulamasıdır (Unix socket istemcisi).
type privImpl struct {
	c *privclient.Client
}

// NewPrivOps, privOps üretim uygulamasını döndürür.
func NewPrivOps(c *privclient.Client) privOps { return &privImpl{c: c} }

func (p *privImpl) UserCreate(ctx context.Context, name, home string) error {
	_, err := p.c.Call(ctx, "user.create", map[string]any{"name": name, "home": home})
	return err
}

func (p *privImpl) UserDelete(ctx context.Context, name string) error {
	_, err := p.c.Call(ctx, "user.delete", map[string]any{"name": name})
	return err
}

func (p *privImpl) UserExists(ctx context.Context, name string) (bool, error) {
	data, err := p.c.Call(ctx, "user.exists", map[string]any{"name": name})
	if err != nil {
		return false, err
	}
	exists, _ := data["exists"].(bool)
	return exists, nil
}

func (p *privImpl) SitePrepare(ctx context.Context, siteID, user string) error {
	_, err := p.c.Call(ctx, "site.prepare", map[string]any{"site": siteID, "user": user})
	return err
}

func (p *privImpl) SiteTeardown(ctx context.Context, siteID, user string) error {
	_, err := p.c.Call(ctx, "site.teardown", map[string]any{"site": siteID, "user": user})
	return err
}

func (p *privImpl) CgroupLimits(ctx context.Context, siteID string, l Limits) error {
	_, err := p.c.Call(ctx, "cgroup.limits", map[string]any{
		"site": siteID, "cpu_max": l.CPUMax,
		"memory_max": l.MemoryMax, "memory_high": l.MemoryHigh, "pids_max": l.PIDsMax,
	})
	return err
}

func (p *privImpl) CgroupCleanup(ctx context.Context, siteID string) error {
	_, err := p.c.Call(ctx, "cgroup.cleanup", map[string]any{"site": siteID})
	return err
}

func (p *privImpl) QuotaSet(ctx context.Context, user string, diskMB, inodes uint64) error {
	_, err := p.c.Call(ctx, "quota.set", map[string]any{"user": user, "disk_mb": diskMB, "inodes": inodes})
	return err
}
