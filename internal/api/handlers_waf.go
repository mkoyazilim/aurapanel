package api

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// ─── OWASP CRS ───────────────────────────────────────────────────────────────

// GET /api/v1/waf/crs
func (s *Server) handleWAFCRSGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	cfg, err := s.deps.Store.GetWAFCRSConfig(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// PUT /api/v1/waf/crs
func (s *Server) handleWAFCRSUpdate(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req struct {
		CRSVersion string `json:"crs_version"`
		Paranoia   int    `json:"paranoia"`
		DryRun     bool   `json:"dry_run"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Paranoia < 1 || req.Paranoia > 4 {
		writeErr(w, http.StatusBadRequest, "paranoia must be 1-4")
		return
	}
	if err := s.deps.Store.UpdateWAFCRSConfig(r.Context(), store.WAFCRSConfig{
		CRSVersion: req.CRSVersion,
		Paranoia:   req.Paranoia,
		DryRun:     req.DryRun,
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{
		User: sess.Username, Action: "waf.crs_update",
		Target: fmt.Sprintf("crs=%s paranoia=%d dry_run=%v", req.CRSVersion, req.Paranoia, req.DryRun),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ─── Custom WAF Rules ────────────────────────────────────────────────────────

// GET /api/v1/sites/{id}/waf/rules
func (s *Server) handleWAFRulesList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	rules, err := s.deps.Store.ListWAFRules(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rules == nil {
		rules = []store.WAFRule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

// POST /api/v1/sites/{id}/waf/rules
func (s *Server) handleWAFRuleCreate(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	siteID := r.PathValue("id")
	var req struct {
		RuleID      string `json:"rule_id"`
		Phase       int    `json:"phase"`
		Action      string `json:"action"`
		Pattern     string `json:"pattern"`
		Description string `json:"description"`
		Enabled     bool   `json:"enabled"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.RuleID == "" || req.Pattern == "" {
		writeErr(w, http.StatusBadRequest, "rule_id and pattern required")
		return
	}
	// Syntax doğrulaması: pattern geçerli regex mi?
	if _, err := regexp.Compile(req.Pattern); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid regex pattern: "+err.Error())
		return
	}
	if req.Phase < 1 || req.Phase > 5 {
		req.Phase = 2
	}
	if req.Action == "" {
		req.Action = "deny"
	}
	id, err := s.deps.Store.InsertWAFRule(r.Context(), store.WAFRule{
		SiteID: siteID, RuleID: req.RuleID, Phase: req.Phase,
		Action: req.Action, Pattern: req.Pattern, Description: req.Description,
		Enabled: req.Enabled,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{
		User: sess.Username, Action: "waf.rule_create",
		Target: fmt.Sprintf("%s:%s", siteID, req.RuleID),
	})
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "rule_id": req.RuleID})
}

// PUT /api/v1/sites/{id}/waf/rules/{ruleid}
func (s *Server) handleWAFRuleUpdate(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	siteID    := r.PathValue("id")
	ruleIDStr := r.PathValue("ruleid")
	ruleID, err := strconv.ParseInt(ruleIDStr, 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid rule id")
		return
	}
	var req struct {
		Phase       int    `json:"phase"`
		Action      string `json:"action"`
		Pattern     string `json:"pattern"`
		Description string `json:"description"`
		Enabled     bool   `json:"enabled"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Pattern != "" {
		if _, err := regexp.Compile(req.Pattern); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid regex pattern: "+err.Error())
			return
		}
	}
	if err := s.deps.Store.UpdateWAFRule(r.Context(), ruleID, store.WAFRule{
		SiteID: siteID, Phase: req.Phase, Action: req.Action,
		Pattern: req.Pattern, Description: req.Description, Enabled: req.Enabled,
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{
		User: sess.Username, Action: "waf.rule_update",
		Target: fmt.Sprintf("%s:rule=%d", siteID, ruleID),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DELETE /api/v1/sites/{id}/waf/rules/{ruleid}
func (s *Server) handleWAFRuleDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	siteID    := r.PathValue("id")
	ruleIDStr := r.PathValue("ruleid")
	ruleID, err := strconv.ParseInt(ruleIDStr, 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid rule id")
		return
	}
	if err := s.deps.Store.DeleteWAFRule(r.Context(), ruleID, siteID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{
		User: sess.Username, Action: "waf.rule_delete",
		Target: fmt.Sprintf("%s:rule=%d", siteID, ruleID),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ─── WAF Dry-run test ────────────────────────────────────────────────────────

// POST /api/v1/sites/{id}/waf/test
// Body: {uri, method, headers: {}, body_sample: ""} — kuralları dry-run test eder.
func (s *Server) handleWAFTest(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	var req struct {
		URI        string            `json:"uri"`
		Method     string            `json:"method"`
		BodySample string            `json:"body_sample"`
		Headers    map[string]string `json:"headers"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	rules, err := s.deps.Store.ListWAFRules(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	type Match struct {
		RuleID  string `json:"rule_id"`
		Action  string `json:"action"`
		Field   string `json:"field"`
		Pattern string `json:"pattern"`
	}
	var matches []Match

	testTargets := map[string]string{
		"uri":         req.URI,
		"method":      req.Method,
		"body_sample": req.BodySample,
	}
	for k, v := range req.Headers {
		testTargets["header:"+k] = v
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		rx, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}
		for field, val := range testTargets {
			if rx.MatchString(val) {
				matches = append(matches, Match{
					RuleID: rule.RuleID, Action: rule.Action,
					Field: field, Pattern: rule.Pattern,
				})
				// dry_run loguna yaz
				s.deps.Store.InsertWAFRequestLog(r.Context(), store.WAFRequestLog{
					SiteID: siteID, RuleID: rule.RuleID, Action: rule.Action,
					ClientIP: "dry-run", URI: req.URI, Method: req.Method, DryRun: true,
				})
				break
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"matches":     matches,
		"match_count": len(matches),
		"dry_run":     true,
	})
}

// ─── WAF Request Log ─────────────────────────────────────────────────────────

// GET /api/v1/sites/{id}/waf/log?limit=
func (s *Server) handleWAFLog(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	logs, err := s.deps.Store.ListWAFRequestLog(r.Context(), siteID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if logs == nil {
		logs = []store.WAFRequestLog{}
	}
	writeJSON(w, http.StatusOK, logs)
}
