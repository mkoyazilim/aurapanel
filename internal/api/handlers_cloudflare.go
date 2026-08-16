package api

import (
	"net/http"
	"strings"

	"github.com/mkoyazilim/aurapanel/internal/cloudflare"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// ── Global Hesap (email + token) ─────────────────────────────────────────────

// GET /api/v1/cloudflare/account
func (s *Server) handleCFAccountGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	acc, err := s.deps.Store.GetCloudflareAccount(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if acc == nil {
		acc = &store.CloudflareAccount{}
	}
	// API token'ı maskele
	masked := *acc
	if masked.APIToken != "" {
		masked.APIToken = "••••••••" + masked.APIToken[max(0, len(masked.APIToken)-4):]
	}
	writeJSON(w, http.StatusOK, masked)
}

// POST /api/v1/cloudflare/account
func (s *Server) handleCFAccountSave(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req store.CloudflareAccount
	if !decodeBody(w, r, &req) {
		return
	}
	// Eğer token değiştirilmediyse (maskeli geldiyse) mevcut değeri koru
	if strings.HasPrefix(req.APIToken, "••••••••") {
		existing, _ := s.deps.Store.GetCloudflareAccount(r.Context())
		if existing != nil {
			req.APIToken = existing.APIToken
		}
	}
	if err := s.deps.Store.SaveCloudflareAccount(r.Context(), &req); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /api/v1/cloudflare/verify
func (s *Server) handleCFVerifyToken(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		Token string `json:"token"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.Cloudflare.VerifyToken(r.Context(), req.Token); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /api/v1/cloudflare/zones
func (s *Server) handleCFListZones(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	zones, err := s.deps.Cloudflare.ListZones(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, zones)
}

// ── Site Zone Bağlaması ───────────────────────────────────────────────────────

// GET /api/v1/sites/{id}/cloudflare
func (s *Server) handleCloudflareGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	settings, err := s.deps.Store.GetCloudflareSettings(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if settings == nil {
		settings = &store.CloudflareSettings{SiteID: siteID}
	}
	if settings.APIToken != "" {
		settings.APIToken = "••••••••" + settings.APIToken[max(0, len(settings.APIToken)-4):]
	}
	writeJSON(w, http.StatusOK, settings)
}

// POST /api/v1/sites/{id}/cloudflare
func (s *Server) handleCloudflareSave(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	var req store.CloudflareSettings
	if !decodeBody(w, r, &req) {
		return
	}
	req.SiteID = siteID
	if strings.HasPrefix(req.APIToken, "••••••••") {
		existing, _ := s.deps.Store.GetCloudflareSettings(r.Context(), siteID)
		if existing != nil {
			req.APIToken = existing.APIToken
		}
	}
	if err := s.deps.Store.SaveCloudflareSettings(r.Context(), &req); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── DNS Kayıtları ─────────────────────────────────────────────────────────────

// GET /api/v1/sites/{id}/cloudflare/dns
func (s *Server) handleCFDNSList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	recs, err := s.deps.Cloudflare.ListDNSRecords(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

// POST /api/v1/sites/{id}/cloudflare/dns
func (s *Server) handleCFDNSCreate(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var rec cloudflare.DNSRecord
	if !decodeBody(w, r, &rec) {
		return
	}
	out, err := s.deps.Cloudflare.CreateDNSRecord(r.Context(), r.PathValue("id"), rec)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// PATCH /api/v1/sites/{id}/cloudflare/dns/{recid}
func (s *Server) handleCFDNSUpdate(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var rec cloudflare.DNSRecord
	if !decodeBody(w, r, &rec) {
		return
	}
	out, err := s.deps.Cloudflare.UpdateDNSRecord(r.Context(), r.PathValue("id"), r.PathValue("recid"), rec)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// DELETE /api/v1/sites/{id}/cloudflare/dns/{recid}
func (s *Server) handleCFDNSDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if err := s.deps.Cloudflare.DeleteDNSRecord(r.Context(), r.PathValue("id"), r.PathValue("recid")); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── Zone Ayarları ─────────────────────────────────────────────────────────────

// GET /api/v1/sites/{id}/cloudflare/settings
func (s *Server) handleCFSettingsGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	cfg, err := s.deps.Cloudflare.GetZoneSettings(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// PATCH /api/v1/sites/{id}/cloudflare/settings/{key}
func (s *Server) handleCFSettingUpdate(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		Value any `json:"value"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.Cloudflare.UpdateZoneSetting(r.Context(), r.PathValue("id"), r.PathValue("key"), req.Value); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── Cache ─────────────────────────────────────────────────────────────────────

// POST /api/v1/sites/{id}/cloudflare/purge
func (s *Server) handleCloudflarePurge(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if err := s.deps.Cloudflare.PurgeCache(r.Context(), r.PathValue("id")); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /api/v1/sites/{id}/cloudflare/purge-urls
func (s *Server) handleCFPurgeURLs(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		URLs []string `json:"urls"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.Cloudflare.PurgeCacheByURLs(r.Context(), r.PathValue("id"), req.URLs); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── Firewall Kuralları ────────────────────────────────────────────────────────

// GET /api/v1/sites/{id}/cloudflare/firewall
func (s *Server) handleCFFirewallList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	rules, err := s.deps.Cloudflare.ListFirewallRules(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

// POST /api/v1/sites/{id}/cloudflare/firewall
func (s *Server) handleCFFirewallCreate(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var rule cloudflare.FirewallRule
	if !decodeBody(w, r, &rule) {
		return
	}
	out, err := s.deps.Cloudflare.CreateFirewallRule(r.Context(), r.PathValue("id"), rule)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// DELETE /api/v1/sites/{id}/cloudflare/firewall/{ruleid}
func (s *Server) handleCFFirewallDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if err := s.deps.Cloudflare.DeleteFirewallRule(r.Context(), r.PathValue("id"), r.PathValue("ruleid")); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── Analytics ────────────────────────────────────────────────────────────────

// GET /api/v1/sites/{id}/cloudflare/analytics?since=-1440
func (s *Server) handleCFAnalytics(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	since := r.URL.Query().Get("since")
	if since == "" {
		since = "-1440"
	}
	data, err := s.deps.Cloudflare.GetAnalytics(r.Context(), r.PathValue("id"), since)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// max, Go 1.21 öncesi için yerelde tanımlanmış (go built-in'de var, burada güvenli)
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
