package agent
import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/mkoyazilim/aurapanel/internal/site"
)

// AgentServer, agent modunda çalışan minimal HTTP sunucusudur.
// Merkezi panel bu endpoint'lere mTLS üzerinden erişir.
type SiteManager interface {
	Create(ctx context.Context, req site.CreateRequest) (string, error)
}

type AgentServer struct {
	apiKey  string
	version string
	sites   SiteManager
}

// NewAgentServer, verilen API anahtarıyla agent HTTP sunucusu oluşturur.
func NewAgentServer(apiKey, version string, sites SiteManager) *AgentServer {
	return &AgentServer{apiKey: apiKey, version: version, sites: sites}
}

// RegisterRoutes, mux'a agent endpoint'lerini kaydeder.
func (a *AgentServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/agent/health", a.authMiddleware(a.handleHealth))
	mux.HandleFunc("GET /api/v1/agent/metrics", a.authMiddleware(a.handleMetrics))
	mux.HandleFunc("POST /api/v1/agent/sites", a.authMiddleware(a.handleCreateSite))
	mux.HandleFunc("PUT /api/v1/agent/key", a.authMiddleware(a.handleRotateKey))
}

// authMiddleware, Bearer token doğrulaması yapar.
func (a *AgentServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		auth := r.Header.Get("Authorization")
		if len(auth) <= len(prefix) || auth[:len(prefix)] != prefix || auth[len(prefix):] != a.apiKey {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func agentJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// handleHealth, agent'ın çalıştığını ve sürümünü bildirir.
func (a *AgentServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	agentJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": a.version,
	})
}

// handleMetrics, CPU/RAM/uptime metriklerini döndürür.
func (a *AgentServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	cpuPct, _ := cpu.Percent(0, false)
	vm, _ := mem.VirtualMemory()
	uptime, _ := host.Uptime()

	var cpuVal float64
	if len(cpuPct) > 0 {
		cpuVal = cpuPct[0]
	}
	var ramVal float64
	if vm != nil {
		ramVal = vm.UsedPercent
	}

	agentJSON(w, http.StatusOK, map[string]any{
		"cpu_percent": cpuVal,
		"ram_percent": ramVal,
		"uptime_sec":  uptime,
	})
}

// handleCreateSite, site oluşturma isteğini işler.
func (a *AgentServer) handleCreateSite(w http.ResponseWriter, r *http.Request) {
	var req site.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Domain == "" {
		http.Error(w, `{"error":"domain required"}`, http.StatusBadRequest)
		return
	}
	id, err := a.sites.Create(r.Context(), req)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	agentJSON(w, http.StatusCreated, map[string]string{
		"id":     id,
		"domain": req.Domain,
	})
}

// handleRotateKey, agent'ın API anahtarını günceller.
func (a *AgentServer) handleRotateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.APIKey == "" {
		http.Error(w, `{"error":"api_key required"}`, http.StatusBadRequest)
		return
	}
	a.apiKey = req.APIKey
	agentJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
