package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/dns"
)

// dnsClient, config'den PowerDNS istemcisi oluşturur; etkin değilse false döner.
func (s *Server) dnsClient(w http.ResponseWriter) (*dns.Client, bool) {
	if !s.deps.Cfg.PowerDNS.Enabled {
		writeErr(w, http.StatusBadRequest, "powerdns is not enabled")
		return nil, false
	}
	return dns.NewClient(dns.Config{
		Endpoint: s.deps.Cfg.PowerDNS.Endpoint,
		APIKey:   s.deps.Cfg.PowerDNS.APIKey,
		ServerID: s.deps.Cfg.PowerDNS.ServerID,
	}), true
}

// GET /api/v1/dns/zones
func (s *Server) handleDNSZonesList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	client, ok := s.dnsClient(w)
	if !ok {
		return
	}
	zones, err := client.ListZones(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if zones == nil {
		zones = []dns.Zone{}
	}
	writeJSON(w, http.StatusOK, zones)
}

// POST /api/v1/dns/zones
func (s *Server) handleDNSZoneCreate(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req struct {
		Domain      string   `json:"domain"`
		Nameservers []string `json:"nameservers"`
		Kind        string   `json:"kind"` // "Native"|"Master"|"Slave"
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Domain == "" {
		writeErr(w, http.StatusBadRequest, "domain required")
		return
	}
	client, ok := s.dnsClient(w)
	if !ok {
		return
	}
	if err := client.CreateZone(r.Context(), req.Domain, req.Nameservers); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{User: sess.Username, Action: "dns.zone_create", Target: req.Domain})
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok", "domain": req.Domain})
}

// DELETE /api/v1/dns/zones/{domain}
func (s *Server) handleDNSZoneDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	domain := r.PathValue("domain")
	client, ok := s.dnsClient(w)
	if !ok {
		return
	}
	if err := client.DeleteZone(r.Context(), domain); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{User: sess.Username, Action: "dns.zone_delete", Target: domain})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /api/v1/dns/zones/{domain}/records
func (s *Server) handleDNSRecordsList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	domain := r.PathValue("domain")
	client, ok := s.dnsClient(w)
	if !ok {
		return
	}
	zone, err := client.GetZone(r.Context(), domain)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, zone)
}

// POST /api/v1/dns/zones/{domain}/records
func (s *Server) handleDNSRecordCreate(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	domain := r.PathValue("domain")
	var req struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		TTL     int    `json:"ttl"`
		Content string `json:"content"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Name == "" || req.Type == "" || req.Content == "" {
		writeErr(w, http.StatusBadRequest, "name, type, content required")
		return
	}
	if req.TTL <= 0 {
		req.TTL = 3600
	}
	client, ok := s.dnsClient(w)
	if !ok {
		return
	}
	if err := client.AddRecord(r.Context(), domain, req.Name, req.Type, req.TTL, req.Content); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{
		User: sess.Username, Action: "dns.record_create",
		Target: fmt.Sprintf("%s %s %s", domain, req.Type, req.Name),
	})
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// DELETE /api/v1/dns/zones/{domain}/records
func (s *Server) handleDNSRecordDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	domain := r.PathValue("domain")
	var req struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	client, ok := s.dnsClient(w)
	if !ok {
		return
	}
	if err := client.DeleteRecord(r.Context(), domain, req.Name, req.Type); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{
		User: sess.Username, Action: "dns.record_delete",
		Target: fmt.Sprintf("%s %s %s", domain, req.Type, req.Name),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PUT /api/v1/dns/zones/{domain}/soa
func (s *Server) handleDNSSOAUpdate(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	domain := r.PathValue("domain")
	var req struct {
		MName string `json:"mname"`
		RName string `json:"rname"`
		TTL   int    `json:"ttl"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.MName == "" || req.RName == "" {
		writeErr(w, http.StatusBadRequest, "mname and rname required")
		return
	}
	if req.TTL <= 0 {
		req.TTL = 3600
	}
	client, ok := s.dnsClient(w)
	if !ok {
		return
	}
	if err := client.UpdateSOA(r.Context(), domain, req.MName, req.RName, req.TTL); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{User: sess.Username, Action: "dns.soa_update", Target: domain})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /api/v1/dns/zones/{domain}/export
func (s *Server) handleDNSZoneExport(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	domain := r.PathValue("domain")
	client, ok := s.dnsClient(w)
	if !ok {
		return
	}
	zone, err := client.ExportZone(r.Context(), domain)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zone"`, domain))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(zone))
}

// POST /api/v1/dns/zones/{domain}/dnssec
func (s *Server) handleDNSSECEnable(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	domain := r.PathValue("domain")
	client, ok := s.dnsClient(w)
	if !ok {
		return
	}
	if err := client.EnableDNSSEC(r.Context(), domain); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{User: sess.Username, Action: "dns.dnssec_enable", Target: domain})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DELETE /api/v1/dns/zones/{domain}/dnssec
func (s *Server) handleDNSSECDisable(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	domain := r.PathValue("domain")
	client, ok := s.dnsClient(w)
	if !ok {
		return
	}
	if err := client.DisableDNSSEC(r.Context(), domain); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{User: sess.Username, Action: "dns.dnssec_disable", Target: domain})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /api/v1/dns/zones/{domain}/cryptokeys
func (s *Server) handleDNSCryptoKeysList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	domain := r.PathValue("domain")
	client, ok := s.dnsClient(w)
	if !ok {
		return
	}
	keys, err := client.ListCryptoKeys(r.Context(), domain)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if keys == nil {
		keys = []dns.CryptoKey{}
	}
	writeJSON(w, http.StatusOK, keys)
}

// POST /api/v1/dns/zones/{domain}/cryptokeys
func (s *Server) handleDNSCryptoKeyAdd(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	domain := r.PathValue("domain")
	var req struct {
		KeyType   string `json:"key_type"`  // "ksk" | "zsk"
		Algorithm string `json:"algorithm"` // "ecdsa256" | "rsasha256"
		Bits      int    `json:"bits"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.KeyType == "" {
		req.KeyType = "zsk"
	}
	if req.Algorithm == "" {
		req.Algorithm = "ecdsa256"
	}
	if req.Bits == 0 {
		req.Bits = 256
	}
	client, ok := s.dnsClient(w)
	if !ok {
		return
	}
	key, err := client.AddCryptoKey(r.Context(), domain, req.KeyType, req.Algorithm, req.Bits)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{User: sess.Username, Action: "dns.cryptokey_add", Target: domain})
	writeJSON(w, http.StatusCreated, key)
}

// DELETE /api/v1/dns/zones/{domain}/cryptokeys/{keyid}
func (s *Server) handleDNSCryptoKeyDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	domain := r.PathValue("domain")
	keyIDStr := r.PathValue("keyid")
	keyID, err := strconv.Atoi(keyIDStr)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid key id")
		return
	}
	client, ok := s.dnsClient(w)
	if !ok {
		return
	}
	if err := client.DeleteCryptoKey(r.Context(), domain, keyID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{
		User: sess.Username, Action: "dns.cryptokey_delete",
		Target: fmt.Sprintf("%s key=%d", domain, keyID),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /api/v1/dns/zones/{domain}/rectify
func (s *Server) handleDNSRectify(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	domain := r.PathValue("domain")
	client, ok := s.dnsClient(w)
	if !ok {
		return
	}
	if err := client.RectifyZone(r.Context(), domain); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{User: sess.Username, Action: "dns.rectify", Target: domain})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
