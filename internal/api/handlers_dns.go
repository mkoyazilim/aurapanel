package api

import (
	"net/http"

	"github.com/mkoyazilim/aurapanel/internal/dns"
)

func (s *Server) handleDNSZonesList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	// Simplified API for Phase 3
	writeJSON(w, http.StatusOK, []map[string]any{})
}

func (s *Server) handleDNSZoneCreate(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	var req struct {
		Domain      string   `json:"domain"`
		Nameservers []string `json:"nameservers"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	if !s.deps.Cfg.PowerDNS.Enabled {
		writeErr(w, http.StatusBadRequest, "powerdns is not enabled")
		return
	}

	client := dns.NewClient(dns.Config{
		Endpoint: s.deps.Cfg.PowerDNS.Endpoint,
		APIKey:   s.deps.Cfg.PowerDNS.APIKey,
		ServerID: s.deps.Cfg.PowerDNS.ServerID,
	})

	if err := client.CreateZone(r.Context(), req.Domain, req.Nameservers); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (s *Server) handleDNSZoneDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	domain := r.PathValue("domain")

	if !s.deps.Cfg.PowerDNS.Enabled {
		writeErr(w, http.StatusBadRequest, "powerdns is not enabled")
		return
	}

	client := dns.NewClient(dns.Config{
		Endpoint: s.deps.Cfg.PowerDNS.Endpoint,
		APIKey:   s.deps.Cfg.PowerDNS.APIKey,
		ServerID: s.deps.Cfg.PowerDNS.ServerID,
	})

	if err := client.DeleteZone(r.Context(), domain); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
