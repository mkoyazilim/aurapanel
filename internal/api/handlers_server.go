package api

import (
	"net/http"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

// GET /api/v1/server/metrics
func (s *Server) handleServerMetrics(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	cpuPct, _ := cpu.Percent(0, false)
	vm, _ := mem.VirtualMemory()
	uptime, _ := host.Uptime()
	du, _ := disk.Usage("/")

	var cpuVal float64
	if len(cpuPct) > 0 {
		cpuVal = cpuPct[0]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"cpu_percent":  cpuVal,
		"ram_percent":  vm.UsedPercent,
		"ram_used_mb":  vm.Used / 1024 / 1024,
		"ram_total_mb": vm.Total / 1024 / 1024,
		"disk_percent": du.UsedPercent,
		"disk_used_gb": du.Used / 1024 / 1024 / 1024,
		"disk_total_gb": du.Total / 1024 / 1024 / 1024,
		"uptime_sec":   uptime,
	})
}

// GET /api/v1/server/services
func (s *Server) handleServerServices(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	// Bu bilgiyi priv helper üzerinden almak daha güvenlidir.
	// Ama basit systemctl okumaları priv gerektirmeyebilir.
	// Yine de 'systemctl is-active' komutlarını privOps'a ekleyeceğiz.
	
	// Geçici olarak mock döndürelim, Phase 2'de bunu priv ops ile dolduracağız.
	writeJSON(w, http.StatusOK, map[string]any{
		"lsws":     "active",
		"mariadb":  "active",
		"fail2ban": "inactive",
		"sshd":     "active",
	})
}
