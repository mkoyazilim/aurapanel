package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/agent"
	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

func (s *Server) handleClusterList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	servers, err := s.deps.Store.ListServers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, servers)
}

func (s *Server) handleClusterAdd(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	var req struct {
		Name      string `json:"name"`
		IPAddress string `json:"ip_address"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	if req.Name == "" || req.IPAddress == "" {
		writeErr(w, http.StatusBadRequest, "name and ip_address are required")
		return
	}

	// Generate a unique API key for this server
	apiKeyBytes := make([]byte, 32)
	rand.Read(apiKeyBytes)
	apiKey := hex.EncodeToString(apiKeyBytes)

	// Generate Server ID
	idBytes := make([]byte, 8)
	rand.Read(idBytes)
	serverID := "srv_" + hex.EncodeToString(idBytes)

	srv := store.Server{
		ID:        serverID,
		Name:      req.Name,
		IPAddress: req.IPAddress,
		APIKey:    apiKey,
		Status:    "active",
	}

	if err := s.deps.Store.InsertServer(r.Context(), srv); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, srv)
}

func (s *Server) handleClusterDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	serverID := r.PathValue("id")
	if serverID == "" {
		writeErr(w, http.StatusBadRequest, "server id required")
		return
	}

	if err := s.deps.Store.DeleteServer(r.Context(), serverID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleClusterHealth(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	serverID := r.PathValue("id")
	srv, err := s.deps.Store.GetServer(r.Context(), serverID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "server not found")
		return
	}

	client := agent.NewClient()
	if err := client.HealthCheck(r.Context(), *srv); err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "offline", "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "online"})
}

// ServerMetrics, bir sunucudan toplanan metrik sonucu.
type ServerMetrics struct {
	ServerID   string  `json:"server_id"`
	Name       string  `json:"name"`
	IPAddress  string  `json:"ip_address"`
	Status     string  `json:"status"`
	CPUPercent float64 `json:"cpu_percent"`
	RAMPercent float64 `json:"ram_percent"`
	UptimeSec  uint64  `json:"uptime_sec"`
	Error      string  `json:"error,omitempty"`
}

// handleClusterMetrics, tüm agent sunuculardan paralel metrik toplar.
func (s *Server) handleClusterMetrics(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	servers, err := s.deps.Store.ListServers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	results := make([]ServerMetrics, len(servers))
	var wg sync.WaitGroup
	for i, srv := range servers {
		wg.Add(1)
		go func(idx int, srv store.Server) {
			defer wg.Done()
			sm := ServerMetrics{
				ServerID:  srv.ID,
				Name:      srv.Name,
				IPAddress: srv.IPAddress,
				Status:    srv.Status,
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			m, err := agent.NewClient().GetMetrics(ctx, srv)
			if err != nil {
				sm.Error = err.Error()
				sm.Status = "offline"
			} else {
				sm.CPUPercent = m.CPUPercent
				sm.RAMPercent = m.RAMPercent
				sm.UptimeSec = m.UptimeSec
			}
			results[idx] = sm
		}(i, srv)
	}
	wg.Wait()

	writeJSON(w, http.StatusOK, results)
}

// handleClusterEvents, cluster olay log'unu döndürür.
func (s *Server) handleClusterEvents(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	serverID := r.URL.Query().Get("server_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	events, err := s.deps.Store.ListClusterEvents(r.Context(), serverID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// handleClusterKeyRotate, seçili sunucunun API anahtarını döndürür.
func (s *Server) handleClusterKeyRotate(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}

	serverID := r.PathValue("id")
	srv, err := s.deps.Store.GetServer(r.Context(), serverID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "server not found")
		return
	}

	// Yeni anahtar üret.
	keyBytes := make([]byte, 32)
	rand.Read(keyBytes)
	newKey := hex.EncodeToString(keyBytes)

	// Agent'a bildir.
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := agent.NewClient().RotateKey(ctx, *srv, newKey); err != nil {
		writeErr(w, http.StatusBadGateway, "agent key rotation failed: "+err.Error())
		return
	}

	// Store'u güncelle.
	if err := s.deps.Store.RotateServerAPIKey(r.Context(), serverID, newKey); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Event kaydet.
	s.deps.Store.InsertClusterEvent(r.Context(), store.ClusterEvent{
		ServerID: serverID, EventType: "key_rotated", Detail: "rotated by admin",
	})
	s.deps.Audit.Write(r.Context(), audit.Event{
		User: sess.Username, Action: "cluster.key_rotate", Target: serverID,
	})

	writeJSON(w, http.StatusOK, map[string]string{"api_key": newKey, "status": "ok"})
}

// handleClusterCreateSite, hedef sunucuda site oluşturma isteği gönderir.
func (s *Server) handleClusterCreateSite(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}

	serverID := r.PathValue("id")
	srv, err := s.deps.Store.GetServer(r.Context(), serverID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "server not found")
		return
	}

	var payload map[string]any
	if !decodeBody(w, r, &payload) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	siteID, err := agent.NewClient().CreateSiteOnServer(ctx, *srv, payload)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "agent site create failed: "+err.Error())
		return
	}

	s.deps.Store.InsertClusterEvent(r.Context(), store.ClusterEvent{
		ServerID: serverID, EventType: "site_created", Detail: siteID,
	})
	s.deps.Audit.Write(r.Context(), audit.Event{
		User: sess.Username, Action: "cluster.site_create", Target: serverID + ":" + siteID,
	})

	writeJSON(w, http.StatusCreated, map[string]string{"id": siteID, "server_id": serverID})
}

// startHealthPoller, 60 saniyede bir tüm agent sunucuların health'ini kontrol eder.
func (s *Server) startHealthPoller() {
	tick := time.NewTicker(60 * time.Second)
	defer tick.Stop()
	for range tick.C {
		servers, err := s.deps.Store.ListServers(context.Background())
		if err != nil {
			continue
		}
		cli := agent.NewClient()
		for _, srv := range servers {
			srv := srv
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				evType := "health_ok"
				detail := ""
				if err := cli.HealthCheck(ctx, srv); err != nil {
					evType = "health_fail"
					detail = err.Error()
				}
				s.deps.Store.InsertClusterEvent(context.Background(), store.ClusterEvent{
					ServerID: srv.ID, EventType: evType, Detail: detail,
				})
			}()
		}
	}
}
