package api

import (
	"net/http"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

func (s *Server) handleSystemStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	// CPU
	cpuPercents, err := cpu.Percent(time.Second, false)
	cpuUsage := 0.0
	if err == nil && len(cpuPercents) > 0 {
		cpuUsage = cpuPercents[0]
	}
	cpuCores, _ := cpu.Counts(true)

	// RAM
	vMem, err := mem.VirtualMemory()
	ramUsage := 0.0
	ramTotal := uint64(0)
	ramUsed := uint64(0)
	if err == nil {
		ramUsage = vMem.UsedPercent
		ramTotal = vMem.Total
		ramUsed = vMem.Used
	}

	// Disk (Root /)
	diskStat, err := disk.Usage("/")
	diskUsage := 0.0
	diskTotal := uint64(0)
	diskUsed := uint64(0)
	if err == nil {
		diskUsage = diskStat.UsedPercent
		diskTotal = diskStat.Total
		diskUsed = diskStat.Used
	}

	// Yanıt verisi
	data := map[string]any{
		"cpu": map[string]any{
			"usage": cpuUsage,
			"cores": cpuCores,
		},
		"ram": map[string]any{
			"usage": ramUsage,
			"total": ramTotal,
			"used":  ramUsed,
		},
		"disk": map[string]any{
			"usage": diskUsage,
			"total": diskTotal,
			"used":  diskUsed,
		},
		"timestamp": time.Now().UnixMilli(),
	}

	writeJSON(w, http.StatusOK, data)
}
