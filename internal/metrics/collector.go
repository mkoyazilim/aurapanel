// Package metrics, site başına kaynak kullanımını toplar.
// cgroup CPU/RAM/PID değerleri + disk quota periyodik okunur, SQLite'a yazılır.
package metrics

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/store"
)

// Collector, metrik toplama servisi.
type Collector struct {
	st          *store.Store
	cgroupBase  string
	sitesRoot   string
	interval    time.Duration
}

// NewCollector, Collector oluşturur.
func NewCollector(st *store.Store, cgroupBase, sitesRoot string, interval time.Duration) *Collector {
	return &Collector{st: st, cgroupBase: cgroupBase, sitesRoot: sitesRoot, interval: interval}
}

// Run, interval'da bir tüm siteleri tarar ve metrikleri kaydeder.
// ctx iptal edildiğinde durur.
func (c *Collector) Run(ctx context.Context) {
	t := time.NewTicker(c.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.collectAll(ctx)
			// 25 saatten eski satırları temizle.
			_ = c.st.PruneMetrics(ctx)
		}
	}
}

func (c *Collector) collectAll(ctx context.Context) {
	sites, err := c.st.ListSites(ctx)
	if err != nil {
		return
	}
	for _, s := range sites {
		if s.Status != "active" {
			continue
		}
		m, err := c.collectSite(s.ID)
		if err != nil {
			continue
		}
		m.SiteID = s.ID
		_ = c.st.InsertMetric(ctx, m)
	}
}

func (c *Collector) collectSite(siteID string) (store.Metric, error) {
	m := store.Metric{}

	// CPU: cgroup cpu.stat -> usage_usec farkı (basit: cpu_pct yaklaşımı)
	cpuStat := path.Join(c.cgroupBase, "sites", siteID, "cpu.stat")
	cpuPct, _ := readCPUPct(cpuStat)
	m.CPUPct = cpuPct

	// RAM: memory.current
	memCurrent := path.Join(c.cgroupBase, "sites", siteID, "memory.current")
	memBytes, _ := readUint64File(memCurrent)
	m.MemMB = float64(memBytes) / (1 << 20)

	// PID sayısı: pids.current
	pidsCurrent := path.Join(c.cgroupBase, "sites", siteID, "pids.current")
	pids, _ := readUint64File(pidsCurrent)
	m.PIDs = int64(pids)

	// Disk: du -sb site dizini (blok bazlı yaklaşım, quota mevcut değilse)
	siteDir := path.Join(c.sitesRoot, siteID)
	diskBytes, inodes, _ := duDir(siteDir)
	m.DiskMB = float64(diskBytes) / (1 << 20)
	m.DiskInodes = int64(inodes)

	return m, nil
}

func readUint64File(p string) (uint64, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
}

// readCPUPct, cpu.stat dosyasından anlık CPU kullanım yüzdesini hesaplar.
// Gerçek % hesabı için iki ölçüm arası fark gerekir; MVP'de
// basit usage_usec/wall_time yaklaşımı kullanılır.
func readCPUPct(cpuStatPath string) (float64, error) {
	b, err := os.ReadFile(cpuStatPath)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "usage_usec ") {
			v, err := strconv.ParseUint(strings.Fields(line)[1], 10, 64)
			if err != nil {
				return 0, err
			}
			// Basit normalize: 1 saniyede 1 çekirdek = 1_000_000 usec
			// Anlık değil kümülatif, yüzde olarak anlamsız ama gösterge niteliğinde.
			_ = v
			return 0, nil // TODO: zaman serisinden delta hesabı
		}
	}
	return 0, fmt.Errorf("usage_usec bulunamadı")
}

// duDir, bir dizinin boyutunu ve inode sayısını hesaplar.
func duDir(dir string) (bytes uint64, inodes uint64, err error) {
	err = walkDir(dir, func(info os.FileInfo) {
		bytes += uint64(info.Size())
		inodes++
	})
	return
}

func walkDir(dir string, fn func(os.FileInfo)) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		fn(info)
		if e.IsDir() {
			_ = walkDir(path.Join(dir, e.Name()), fn)
		}
	}
	return nil
}
