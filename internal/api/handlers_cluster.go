package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/mkoyazilim/aurapanel/internal/agent"
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
