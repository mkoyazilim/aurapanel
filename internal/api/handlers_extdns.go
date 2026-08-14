package api

import (
	"net/http"
	"strconv"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/extdns"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// extDNSSvc, deps üzerinden extdns.Service'i döndürür.
func (s *Server) extDNSSvc() *extdns.Service {
	return s.deps.ExtDNS
}

// ─── Provider CRUD ───────────────────────────────────────────────────────────

// GET /api/v1/extdns/providers
func (s *Server) handleExtDNSList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	providers, err := s.deps.Store.ListExtDNSProviders(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if providers == nil {
		providers = []store.ExtDNSProvider{}
	}
	writeJSON(w, http.StatusOK, providers)
}

// POST /api/v1/extdns/providers
func (s *Server) handleExtDNSCreate(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req struct {
		Name        string            `json:"name"`
		Provider    string            `json:"provider"` // "cloudflare" | "route53"
		Credentials map[string]string `json:"credentials"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Name == "" || req.Provider == "" {
		writeErr(w, http.StatusBadRequest, "name and provider required")
		return
	}
	if req.Provider != "cloudflare" && req.Provider != "route53" {
		writeErr(w, http.StatusBadRequest, "provider must be cloudflare or route53")
		return
	}
	encCreds, err := s.extDNSSvc().EncryptCreds(req.Credentials)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "credential encryption failed: "+err.Error())
		return
	}
	id, err := s.deps.Store.InsertExtDNSProvider(r.Context(), store.ExtDNSProvider{
		Name:        req.Name,
		Provider:    req.Provider,
		Credentials: encCreds,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{User: sess.Username, Action: "extdns.provider_create", Target: req.Name})
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "name": req.Name, "provider": req.Provider})
}

// DELETE /api/v1/extdns/providers/{id}
func (s *Server) handleExtDNSDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.deps.Store.DeleteExtDNSProvider(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{User: sess.Username, Action: "extdns.provider_delete", Target: strconv.FormatInt(id, 10)})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ─── Cloudflare senkron ──────────────────────────────────────────────────────

// GET /api/v1/extdns/providers/{id}/cf/records
func (s *Server) handleExtDNSCFRecords(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	prov, creds, ok := s.resolveProvider(w, r)
	if !ok {
		return
	}
	if prov.Provider != "cloudflare" {
		writeErr(w, http.StatusBadRequest, "provider is not cloudflare")
		return
	}
	records, err := s.extDNSSvc().CloudflareListRecords(r.Context(), creds["api_token"], creds["zone_id"])
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, records)
}

// POST /api/v1/extdns/providers/{id}/cf/sync
// Body: { "records": [{name,type,content,ttl},...] } — local kayıtları CF'a iter.
func (s *Server) handleExtDNSCFSync(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	prov, creds, ok := s.resolveProvider(w, r)
	if !ok {
		return
	}
	if prov.Provider != "cloudflare" {
		writeErr(w, http.StatusBadRequest, "provider is not cloudflare")
		return
	}
	var req struct {
		Records []extdns.DNSRecord `json:"records"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	result, err := s.extDNSSvc().CloudflareSyncPush(r.Context(), creds["api_token"], creds["zone_id"], req.Records)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	detail := "added=" + strconv.Itoa(len(result.Added)) + " conflicts=" + strconv.Itoa(len(result.Conflicts))
	s.deps.Store.InsertExtDNSSyncLog(r.Context(), store.ExtDNSSyncLog{
		ProviderID: prov.ID, Direction: "push", Action: "sync", Detail: detail,
	})
	s.deps.Audit.Write(r.Context(), audit.Event{User: sess.Username, Action: "extdns.cf_sync", Target: prov.Name})
	writeJSON(w, http.StatusOK, result)
}

// ─── Route53 ─────────────────────────────────────────────────────────────────

// GET /api/v1/extdns/providers/{id}/r53/zones
func (s *Server) handleExtDNSR53Zones(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	prov, creds, ok := s.resolveProvider(w, r)
	if !ok {
		return
	}
	if prov.Provider != "route53" {
		writeErr(w, http.StatusBadRequest, "provider is not route53")
		return
	}
	zones, err := s.extDNSSvc().Route53ListZones(r.Context(), creds["access_key"], creds["secret_key"], creds["region"])
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, zones)
}

// ─── Senkron log ─────────────────────────────────────────────────────────────

// GET /api/v1/extdns/sync-log?provider_id=&limit=
func (s *Server) handleExtDNSSyncLog(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	provID, _ := strconv.ParseInt(r.URL.Query().Get("provider_id"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	logs, err := s.deps.Store.ListExtDNSSyncLog(r.Context(), provID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if logs == nil {
		logs = []store.ExtDNSSyncLog{}
	}
	writeJSON(w, http.StatusOK, logs)
}

// ─── Yardımcılar ─────────────────────────────────────────────────────────────

// resolveProvider, path'teki {id}'den provider + şifre çözülmüş creds döndürür.
func (s *Server) resolveProvider(w http.ResponseWriter, r *http.Request) (*store.ExtDNSProvider, map[string]string, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid provider id")
		return nil, nil, false
	}
	prov, err := s.deps.Store.GetExtDNSProvider(r.Context(), id)
	if err != nil || prov == nil {
		writeErr(w, http.StatusNotFound, "provider not found")
		return nil, nil, false
	}
	creds, err := s.extDNSSvc().DecryptCreds(prov.Credentials)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "credential decryption failed")
		return nil, nil, false
	}
	return prov, creds, true
}
