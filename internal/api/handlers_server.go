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
		"cpu_percent":   cpuVal,
		"ram_percent":   vm.UsedPercent,
		"ram_used_mb":   vm.Used / 1024 / 1024,
		"ram_total_mb":  vm.Total / 1024 / 1024,
		"disk_percent":  du.UsedPercent,
		"disk_used_gb":  du.Used / 1024 / 1024 / 1024,
		"disk_total_gb": du.Total / 1024 / 1024 / 1024,
		"uptime_sec":    uptime,
	})
}

// GET /api/v1/server/services
func (s *Server) handleServerServices(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	out, err := s.deps.Priv.Call(r.Context(), "server.services", nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /api/v1/server/action  {"action":"restart|start|stop","target":"lsws|mariadb|fail2ban|sshd"}
func (s *Server) handleServerAction(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var body struct {
		Action string `json:"action"`
		Target string `json:"target"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	out, err := s.deps.Priv.Call(r.Context(), "server.action", map[string]any{
		"action": body.Action,
		"target": body.Target,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /api/v1/server/firewall
func (s *Server) handleFirewallList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	out, err := s.deps.Priv.Call(r.Context(), "firewall.list", nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /api/v1/server/firewall  {"port":8080,"proto":"tcp","comment":"panel"}
func (s *Server) handleFirewallRuleAdd(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var body struct {
		Port    int    `json:"port"`
		Proto   string `json:"proto"`
		Comment string `json:"comment"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	out, err := s.deps.Priv.Call(r.Context(), "firewall.rule_add", map[string]any{
		"port": body.Port, "proto": body.Proto, "comment": body.Comment,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// DELETE /api/v1/server/firewall  body: {"port":8080,"proto":"tcp"}
func (s *Server) handleFirewallRuleDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var body struct {
		Port  int    `json:"port"`
		Proto string `json:"proto"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	out, err := s.deps.Priv.Call(r.Context(), "firewall.rule_delete", map[string]any{
		"port": body.Port, "proto": body.Proto,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// PUT /api/v1/server/ssh-port  {"new_port":2244,"old_port":22}
func (s *Server) handleSSHPortChange(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var body struct {
		NewPort int `json:"new_port"`
		OldPort int `json:"old_port"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	out, err := s.deps.Priv.Call(r.Context(), "firewall.ssh_port", map[string]any{
		"new_port": body.NewPort, "old_port": body.OldPort,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// PUT /api/v1/server/panel-port  {"new_port":9090}
func (s *Server) handlePanelPortChange(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var body struct {
		NewPort int `json:"new_port"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	out, err := s.deps.Priv.Call(r.Context(), "firewall.panel_port", map[string]any{
		"new_port": body.NewPort,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
