// Package drift, configuration drift detection'ı uygular (ARCHITECTURE §6):
// desired state (SQLite) ile gerçek sistem (vhost, kullanıcı, cgroup,
// quota, dizinler) karşılaştırılır; sapmalar drift_events'e yazılır,
// Repair ile desired state yeniden uygulanır.
package drift

import (
	"context"
	"os"
)

// quotaActual, quota.get op'undan gelen gerçek durum.
type quotaActual struct {
	Available  bool
	Reason     string
	DiskBlocks uint64 // 1 KiB blok, hard limit
	Inodes     uint64 // hard limit
}

// dirActual, site.status op'undan gelen tek dizin durumu.
type dirActual struct {
	Exists bool
	Mode   os.FileMode
}

// actualCollector, gerçek sistem durumunu okur (priv helper üzerinden).
// Arayüz olması, scanner'ın sahte toplayıcılarla test edilmesini sağlar.
type actualCollector interface {
	VhostBundle(ctx context.Context, siteID string) (string, error)
	UserExists(ctx context.Context, name string) (bool, error)
	CgroupRead(ctx context.Context, siteID string) (map[string]string, error)
	QuotaGet(ctx context.Context, name string) (quotaActual, error)
	SiteStatus(ctx context.Context, siteID, name string) (map[string]dirActual, error)
}

// desiredState, bir sitenin SQLite'tan türetilen istek hâli.
type desiredState struct {
	VhostContent string
	User         string
	Cgroup       map[string]string
	DiskBlocks   uint64
	Inodes       uint64
	Dirs         map[string]os.FileMode
}

// actuals, bir sitenin toplanan gerçek durumu.
type actuals struct {
	VhostContent string
	UserExists   bool
	Cgroup       map[string]string
	Quota        quotaActual
	Dirs         map[string]dirActual
}

// finding, tek bir sapma (scanner bunu drift_events'e dönüştürür).
type finding struct {
	Resource string
	Expected string
	Actual   string
	Severity string
}
