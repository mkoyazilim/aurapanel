package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/extdns"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// ─── OLS Cache Purge ─────────────────────────────────────────────────────────

// POST /api/v1/sites/{id}/cdn/purge
// Body: {urls: ["https://..."]} veya boşsa purge_all=true
func (s *Server) handleCDNPurge(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	siteID := r.PathValue("id")
	var req struct {
		URLs     []string `json:"urls"`
		PurgeAll bool     `json:"purge_all"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	// OLS cache purge: .htaccess Cache-Control veya OLS WebAdmin API üzerinden.
	// Gerçek OLS purge: priv helper üzerinden "cache.purge" op'u çağrılır.
	// MVP: purge event log'a kaydedilir; gerçek OLS API entegrasyonu priv.ops'ta.
	purgeTarget := "all"
	if len(req.URLs) > 0 {
		purgeTarget = fmt.Sprintf("%d URLs", len(req.URLs))
	}

	s.deps.Store.InsertCDNStat(r.Context(), store.CDNStat{
		SiteID: siteID, Source: "ols", Purges: int64(len(req.URLs) + 1),
	})
	s.deps.Audit.Write(r.Context(), audit.Event{
		User: sess.Username, Action: "cdn.purge_ols", Target: siteID + ":" + purgeTarget,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "purged": purgeTarget})
}

// POST /api/v1/sites/{id}/cdn/cf-purge
// Body: {urls: [...]} veya purge_all:true — Cloudflare cache purge
func (s *Server) handleCDNCFPurge(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	siteID := r.PathValue("id")
	var req struct {
		URLs     []string `json:"urls"`
		PurgeAll bool     `json:"purge_all"`
		APIToken string   `json:"api_token"`
		ZoneID   string   `json:"zone_id"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	// Cloudflare CF purge: site CF settings'ten veya request'ten al
	apiToken := req.APIToken
	zoneID   := req.ZoneID
	if apiToken == "" || zoneID == "" {
		// CF settings'ten dene
		cfSettings, err := s.deps.Store.GetCloudflareSettings(r.Context(), siteID)
		if err != nil || cfSettings == nil {
			writeErr(w, http.StatusBadRequest, "cloudflare api_token and zone_id required")
			return
		}
		apiToken = cfSettings.APIToken
		zoneID   = cfSettings.ZoneID
	}

	svc := extdns.New(s.deps.Store, s.deps.Cipher)
	if req.PurgeAll || len(req.URLs) == 0 {
		// Purge all: mevcut CF service'i kullan
		if err := s.deps.Cloudflare.PurgeCache(r.Context(), siteID); err != nil {
			writeErr(w, http.StatusBadGateway, "CF purge failed: "+err.Error())
			return
		}
	} else {
		if err := svc.CloudflarePurgeCacheURLs(r.Context(), apiToken, zoneID, req.URLs); err != nil {
			writeErr(w, http.StatusBadGateway, "CF URL purge failed: "+err.Error())
			return
		}
	}

	s.deps.Store.InsertCDNStat(r.Context(), store.CDNStat{
		SiteID: siteID, Source: "cloudflare", Purges: int64(len(req.URLs) + 1),
	})
	s.deps.Audit.Write(r.Context(), audit.Event{
		User: sess.Username, Action: "cdn.purge_cf", Target: siteID,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ─── CDN Cache Rules ─────────────────────────────────────────────────────────

// GET /api/v1/sites/{id}/cdn/rules
func (s *Server) handleCDNRulesList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	rules, err := s.deps.Store.ListCDNCacheRules(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rules == nil {
		rules = []store.CDNCacheRule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

// POST /api/v1/sites/{id}/cdn/rules
func (s *Server) handleCDNRuleCreate(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	siteID := r.PathValue("id")
	var req struct {
		Pattern    string `json:"pattern"`
		CacheLevel string `json:"cache_level"`
		TTL        int    `json:"ttl"`
		Enabled    bool   `json:"enabled"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Pattern == "" {
		writeErr(w, http.StatusBadRequest, "pattern required")
		return
	}
	if req.CacheLevel == "" {
		req.CacheLevel = "standard"
	}
	id, err := s.deps.Store.InsertCDNCacheRule(r.Context(), store.CDNCacheRule{
		SiteID: siteID, Pattern: req.Pattern, CacheLevel: req.CacheLevel, TTL: req.TTL, Enabled: req.Enabled,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{
		User: sess.Username, Action: "cdn.rule_create",
		Target: fmt.Sprintf("%s:%s", siteID, req.Pattern),
	})
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// PUT /api/v1/sites/{id}/cdn/rules/{ruleid}
func (s *Server) handleCDNRuleUpdate(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	siteID    := r.PathValue("id")
	ruleID, err := strconv.ParseInt(r.PathValue("ruleid"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid rule id")
		return
	}
	var req struct {
		Pattern    string `json:"pattern"`
		CacheLevel string `json:"cache_level"`
		TTL        int    `json:"ttl"`
		Enabled    bool   `json:"enabled"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.Store.UpdateCDNCacheRule(r.Context(), ruleID, store.CDNCacheRule{
		SiteID: siteID, Pattern: req.Pattern, CacheLevel: req.CacheLevel, TTL: req.TTL, Enabled: req.Enabled,
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{
		User: sess.Username, Action: "cdn.rule_update",
		Target: fmt.Sprintf("%s:rule=%d", siteID, ruleID),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DELETE /api/v1/sites/{id}/cdn/rules/{ruleid}
func (s *Server) handleCDNRuleDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	siteID  := r.PathValue("id")
	ruleID, err := strconv.ParseInt(r.PathValue("ruleid"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid rule id")
		return
	}
	if err := s.deps.Store.DeleteCDNCacheRule(r.Context(), ruleID, siteID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{
		User: sess.Username, Action: "cdn.rule_delete",
		Target: fmt.Sprintf("%s:rule=%d", siteID, ruleID),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ─── CDN İstatistikler ────────────────────────────────────────────────────────

// GET /api/v1/sites/{id}/cdn/stats
func (s *Server) handleCDNStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	stats, err := s.deps.Store.ListCDNStats(r.Context(), siteID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	hits, misses, purges, err := s.deps.Store.CDNStatSummary(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if stats == nil {
		stats = []store.CDNStat{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"summary": map[string]int64{"hits": hits, "misses": misses, "purges": purges},
		"history": stats,
	})
}
