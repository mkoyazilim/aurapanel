package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (s *service) firstInstalledPHPVersionLocked() string {
	for _, item := range s.modules.PHPVersions {
		if item.Installed {
			return item.Version
		}
	}
	return "8.3"
}

func (s *service) handlePHPVersions(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.PHPVersions})
}

func (s *service) handlePHPInstall(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Version string `json:"version"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid PHP install payload.")
		return
	}
	version := strings.TrimSpace(payload.Version)
	if version == "" {
		writeError(w, http.StatusBadRequest, "PHP version is required.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for i := range s.modules.PHPVersions {
		if s.modules.PHPVersions[i].Version == version {
			s.modules.PHPVersions[i].Installed = true
			found = true
		}
	}
	if !found {
		s.modules.PHPVersions = append(s.modules.PHPVersions, PHPVersionInfo{Version: version, Installed: true})
	}
	if _, ok := s.modules.PHPIni[version]; !ok {
		s.modules.PHPIni[version] = defaultPHPIni(version)
	}
	s.appendActivityLocked("system", "php_install", fmt.Sprintf("PHP %s installed.", version), "")
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("PHP %s installed.", version)})
}

func (s *service) handlePHPRemove(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Version string `json:"version"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid PHP remove payload.")
		return
	}
	version := strings.TrimSpace(payload.Version)
	if version == "" {
		writeError(w, http.StatusBadRequest, "PHP version is required.")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	replacement := ""
	for _, item := range s.modules.PHPVersions {
		if item.Version != version && item.Installed {
			replacement = item.Version
			break
		}
	}
	if replacement == "" {
		replacement = "8.3"
	}
	for i := range s.modules.PHPVersions {
		if s.modules.PHPVersions[i].Version == version {
			s.modules.PHPVersions[i].Installed = false
		}
	}
	for i := range s.state.Websites {
		if s.state.Websites[i].PHPVersion == version || s.state.Websites[i].PHP == version {
			s.state.Websites[i].PHPVersion = replacement
			s.state.Websites[i].PHP = replacement
		}
	}
	for i := range s.state.Subdomains {
		if s.state.Subdomains[i].PHPVersion == version {
			s.state.Subdomains[i].PHPVersion = replacement
		}
	}
	s.appendActivityLocked("system", "php_remove", fmt.Sprintf("PHP %s removed.", version), "")
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("PHP %s removed.", version)})
}

func (s *service) handlePHPRestart(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Version string `json:"version"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid PHP restart payload.")
		return
	}
	version := strings.TrimSpace(payload.Version)
	if version == "" {
		version = s.firstInstalledPHPVersionLocked()
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("PHP %s restarted.", version)})
}

func (s *service) handlePHPIniGet(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Version string `json:"version"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid php.ini payload.")
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	version := strings.TrimSpace(payload.Version)
	content := s.modules.PHPIni[version]
	if content == "" {
		content = defaultPHPIni(version)
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: content})
}

func (s *service) handlePHPIniSave(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Version string `json:"version"`
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid php.ini save payload.")
		return
	}
	version := strings.TrimSpace(payload.Version)
	if version == "" {
		writeError(w, http.StatusBadRequest, "PHP version is required.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.PHPIni[version] = payload.Content
	s.appendActivityLocked("system", "php_ini_save", fmt.Sprintf("php.ini updated for %s.", version), "")
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("php.ini saved for PHP %s.", version)})
}

func (s *service) handleWebsiteAdvancedConfigGet(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.ensureDefaultSiteArtifactsLocked(domain)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.AdvancedConfig[domain]})
}

func (s *service) handleWebsiteCustomSSLGet(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.CustomSSL[domain]})
}

func (s *service) handleWebsiteCustomSSLSet(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain  string `json:"domain"`
		CertPEM string `json:"cert_pem"`
		KeyPEM  string `json:"key_pem"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid custom SSL payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.CustomSSL[domain] = WebsiteCustomSSL{CertPEM: payload.CertPEM, KeyPEM: payload.KeyPEM}
	if payload.CertPEM != "" && payload.KeyPEM != "" {
		s.recordIssuedCertificateLocked(domain, "custom-upload", strings.HasPrefix(domain, "*."))
	}
	s.appendActivityLocked("system", "ssl_custom", fmt.Sprintf("Custom SSL stored for %s.", domain), "")
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("Custom SSL saved for %s.", domain)})
}

func (s *service) handleWebsiteOpenBasedirSet(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain  string `json:"domain"`
		Enabled bool   `json:"enabled"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid open_basedir payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultSiteArtifactsLocked(domain)
	cfg := s.state.AdvancedConfig[domain]
	cfg.OpenBasedir = payload.Enabled
	s.state.AdvancedConfig[domain] = cfg
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Open Basedir updated.", Data: cfg})
}

func (s *service) handleWebsiteRewriteSet(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string `json:"domain"`
		Rules  string `json:"rules"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid rewrite payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultSiteArtifactsLocked(domain)
	cfg := s.state.AdvancedConfig[domain]
	cfg.RewriteRules = payload.Rules
	s.state.AdvancedConfig[domain] = cfg
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Rewrite rules updated.", Data: cfg})
}

func (s *service) handleWebsiteVhostConfigSet(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain  string `json:"domain"`
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid vhost config payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultSiteArtifactsLocked(domain)
	cfg := s.state.AdvancedConfig[domain]
	cfg.VhostConfig = payload.Content
	s.state.AdvancedConfig[domain] = cfg
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "VHost config updated.", Data: cfg})
}

func (s *service) handleSubdomainPHPSet(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		FQDN       string `json:"fqdn"`
		PHPVersion string `json:"php_version"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid subdomain PHP payload.")
		return
	}
	fqdn := normalizeDomain(payload.FQDN)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Subdomains {
		if s.state.Subdomains[i].FQDN == fqdn {
			s.state.Subdomains[i].PHPVersion = firstNonEmpty(payload.PHPVersion, s.firstInstalledPHPVersionLocked())
			writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Subdomain PHP version updated.", Data: s.state.Subdomains[i]})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Subdomain not found.")
}

func (s *service) handleSubdomainDelete(w http.ResponseWriter, r *http.Request) {
	fqdn := normalizeDomain(r.URL.Query().Get("fqdn"))
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.state.Subdomains
	filtered := items[:0]
	deleted := false
	for _, item := range items {
		if item.FQDN == fqdn {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.state.Subdomains = filtered
	if !deleted {
		writeError(w, http.StatusNotFound, "Subdomain not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Subdomain deleted."})
}

func (s *service) handleSubdomainConvert(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		FQDN       string `json:"fqdn"`
		Owner      string `json:"owner"`
		PHPVersion string `json:"php_version"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid subdomain conversion payload.")
		return
	}
	fqdn := normalizeDomain(payload.FQDN)
	s.mu.Lock()
	defer s.mu.Unlock()
	index := -1
	var source Subdomain
	for i := range s.state.Subdomains {
		if s.state.Subdomains[i].FQDN == fqdn {
			index = i
			source = s.state.Subdomains[i]
			break
		}
	}
	if index == -1 {
		writeError(w, http.StatusNotFound, "Subdomain not found.")
		return
	}
	s.state.Subdomains = append(s.state.Subdomains[:index], s.state.Subdomains[index+1:]...)
	s.state.Websites = append(s.state.Websites, Website{
		Domain:        fqdn,
		Owner:         firstNonEmpty(payload.Owner, "aura"),
		User:          firstNonEmpty(payload.Owner, "aura"),
		PHP:           firstNonEmpty(payload.PHPVersion, source.PHPVersion, s.firstInstalledPHPVersionLocked()),
		PHPVersion:    firstNonEmpty(payload.PHPVersion, source.PHPVersion, s.firstInstalledPHPVersionLocked()),
		Package:       "default",
		Email:         fmt.Sprintf("admin@%s", fqdn),
		Status:        "active",
		SSL:           source.SSLEnabled,
		DiskUsage:     "128 MB",
		Quota:         quotaForPackage(s.state.Packages, "default"),
		MailDomain:    false,
		ApacheBackend: false,
		CreatedAt:     time.Now().UTC().Unix(),
	})
	s.ensureDefaultSiteArtifactsLocked(fqdn)
	s.recountSitesLocked()
	s.appendActivityLocked("system", "subdomain_convert", fmt.Sprintf("%s converted into full website.", fqdn), "")
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Subdomain converted into website."})
}

func (s *service) handleAliasCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string `json:"domain"`
		Alias  string `json:"alias"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid alias payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	alias := normalizeDomain(payload.Alias)
	if domain == "" || alias == "" {
		writeError(w, http.StatusBadRequest, "Domain and alias are required.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.state.Aliases {
		if item.Domain == domain && item.Alias == alias {
			writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Alias already exists."})
			return
		}
	}
	s.state.Aliases = append(s.state.Aliases, DomainAlias{Domain: domain, Alias: alias})
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Alias added."})
}

func (s *service) handleAliasDelete(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	alias := normalizeDomain(r.URL.Query().Get("alias"))
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := s.state.Aliases[:0]
	deleted := false
	for _, item := range s.state.Aliases {
		if item.Domain == domain && item.Alias == alias {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.state.Aliases = filtered
	if !deleted {
		writeError(w, http.StatusNotFound, "Alias not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Alias deleted."})
}

func (s *service) handleWebsiteTraffic(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	if domain == "" {
		writeError(w, http.StatusBadRequest, "Domain is required.")
		return
	}
	hours := clampInt(queryInt(r, "hours", 24), 1, 168)
	series := make([]map[string]interface{}, 0, hours)
	totalHits := 0
	totalVisitors := 0
	totalBandwidth := int64(0)
	for i := hours - 1; i >= 0; i-- {
		hits := 120 + ((hours - i) * 13)
		visitors := 20 + ((hours - i) * 3)
		bandwidth := int64(hits * 4096)
		totalHits += hits
		totalVisitors += visitors
		totalBandwidth += bandwidth
		series = append(series, map[string]interface{}{
			"bucket":          time.Now().UTC().Add(-time.Duration(i) * time.Hour).Format("02 Jan 15:04"),
			"hits":            hits,
			"visitors":        visitors,
			"bandwidth_bytes": bandwidth,
		})
	}
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"totals": map[string]interface{}{
				"hits":            totalHits,
				"visitors":        totalVisitors,
				"bandwidth_bytes": totalBandwidth,
			},
			"series": series,
			"top_paths": []map[string]interface{}{
				{"path": "/", "hits": totalHits / 3, "bandwidth_bytes": totalBandwidth / 3},
				{"path": "/wp-login.php", "hits": totalHits / 5, "bandwidth_bytes": totalBandwidth / 6},
				{"path": "/assets/app.js", "hits": totalHits / 6, "bandwidth_bytes": totalBandwidth / 4},
			},
			"source_log": fmt.Sprintf("/home/%s/logs/access.log", domain),
		},
	})
}

func (s *service) handleDNSZonesList(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.DNSZones})
}

func (s *service) recalcDNSZoneLocked(domain string) {
	for i := range s.modules.DNSZones {
		if s.modules.DNSZones[i].Name == domain {
			s.modules.DNSZones[i].Records = len(s.modules.DNSRecords[domain])
			return
		}
	}
}

func (s *service) handleDNSZoneCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string `json:"domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid DNS zone payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	if domain == "" {
		writeError(w, http.StatusBadRequest, "Domain is required.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, zone := range s.modules.DNSZones {
		if zone.Name == domain {
			writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Zone already exists."})
			return
		}
	}
	s.modules.DNSZones = append(s.modules.DNSZones, DNSZone{ID: generateSecret(6), Name: domain, Kind: "native", Records: 2, DNSSECEnabled: false})
	s.modules.DNSRecords[domain] = []DNSRecord{
		{RecordType: "A", Name: domain, Content: "203.0.113.10", TTL: 3600},
		{RecordType: "TXT", Name: domain, Content: "v=spf1 mx a ~all", TTL: 3600},
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "DNS zone created."})
}

func (s *service) handleDNSZoneDelete(w http.ResponseWriter, domain string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := s.modules.DNSZones[:0]
	deleted := false
	for _, zone := range s.modules.DNSZones {
		if zone.Name == domain {
			deleted = true
			continue
		}
		filtered = append(filtered, zone)
	}
	s.modules.DNSZones = filtered
	delete(s.modules.DNSRecords, domain)
	if !deleted {
		writeError(w, http.StatusNotFound, "Zone not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "DNS zone deleted."})
}

func (s *service) handleDNSRecordsGet(w http.ResponseWriter, domain string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.DNSRecords[domain]})
}

func (s *service) handleDNSRecordCreate(w http.ResponseWriter, r *http.Request, domain string) {
	var payload DNSRecord
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid DNS record payload.")
		return
	}
	if strings.TrimSpace(payload.RecordType) == "" || strings.TrimSpace(payload.Name) == "" || strings.TrimSpace(payload.Content) == "" {
		writeError(w, http.StatusBadRequest, "Record type, name and content are required.")
		return
	}
	if payload.TTL == 0 {
		payload.TTL = 3600
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.DNSRecords[domain] = append(s.modules.DNSRecords[domain], payload)
	s.recalcDNSZoneLocked(domain)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "DNS record added."})
}

func (s *service) handleDNSRecordDelete(w http.ResponseWriter, r *http.Request, domain string) {
	recordType := strings.TrimSpace(r.URL.Query().Get("record_type"))
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.modules.DNSRecords[domain]
	filtered := items[:0]
	deleted := false
	for _, item := range items {
		if strings.EqualFold(item.RecordType, recordType) && item.Name == name {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.DNSRecords[domain] = filtered
	s.recalcDNSZoneLocked(domain)
	if !deleted {
		writeError(w, http.StatusNotFound, "DNS record not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "DNS record deleted."})
}

func (s *service) handleDNSReconcile(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string `json:"domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid DNS reconcile payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.modules.DNSRecords[domain]) == 0 {
		s.modules.DNSRecords[domain] = []DNSRecord{
			{RecordType: "A", Name: domain, Content: "203.0.113.10", TTL: 3600},
			{RecordType: "MX", Name: domain, Content: fmt.Sprintf("mail.%s", domain), TTL: 3600},
			{RecordType: "TXT", Name: domain, Content: "v=spf1 mx a ~all", TTL: 3600},
		}
	}
	s.recalcDNSZoneLocked(domain)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Zone reconciled.", Data: s.modules.DNSRecords[domain]})
}

func (s *service) handleDNSSECSet(w http.ResponseWriter, r *http.Request, domain string) {
	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid DNSSEC payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.modules.DNSZones {
		if s.modules.DNSZones[i].Name == domain {
			s.modules.DNSZones[i].DNSSECEnabled = payload.Enabled
			writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "DNSSEC updated.", Data: s.modules.DNSZones[i]})
			return
		}
	}
	writeError(w, http.StatusNotFound, "DNS zone not found.")
}

func (s *service) handleDefaultNameserversGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.DefaultNameservers})
}

func (s *service) handleDefaultNameserversSet(w http.ResponseWriter, r *http.Request) {
	var payload DefaultNameservers
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid nameserver payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.DefaultNameservers = payload
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Default nameservers saved.", Data: payload})
}

func (s *service) handleDefaultNameserversWizard(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		BaseDomain string `json:"base_domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid nameserver wizard payload.")
		return
	}
	base := normalizeDomain(payload.BaseDomain)
	data := DefaultNameservers{NS1: fmt.Sprintf("ns1.%s", base), NS2: fmt.Sprintf("ns2.%s", base)}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: data})
}

func (s *service) handleDefaultNameserversReset(w http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.DefaultNameservers = DefaultNameservers{}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.DefaultNameservers})
}

func (s *service) handleMailboxesList(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.Mailboxes})
}

func (s *service) handleMailboxCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain   string `json:"domain"`
		Username string `json:"username"`
		Password string `json:"password"`
		QuotaMB  int    `json:"quota_mb"`
		Owner    string `json:"owner"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid mailbox payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	username := sanitizeName(payload.Username)
	if domain == "" || username == "" {
		writeError(w, http.StatusBadRequest, "Domain and username are required.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	address := fmt.Sprintf("%s@%s", username, domain)
	for _, mailbox := range s.modules.Mailboxes {
		if mailbox.Address == address {
			writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Mailbox already exists."})
			return
		}
	}
	s.modules.Mailboxes = append(s.modules.Mailboxes, Mailbox{
		Address: address,
		Domain:  domain,
		User:    username,
		Owner:   firstNonEmpty(payload.Owner, "aura"),
		QuotaMB: maxInt(payload.QuotaMB, 256),
		UsedMB:  0,
	})
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Mailbox created."})
}

func (s *service) handleMailboxDelete(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Address string `json:"address"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid mailbox delete payload.")
		return
	}
	address := strings.TrimSpace(strings.ToLower(payload.Address))
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := s.modules.Mailboxes[:0]
	deleted := false
	for _, item := range s.modules.Mailboxes {
		if item.Address == address {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.Mailboxes = filtered
	if !deleted {
		writeError(w, http.StatusNotFound, "Mailbox not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Mailbox deleted."})
}

func (s *service) handleMailboxPassword(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Address     string `json:"address"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid mailbox password payload.")
		return
	}
	if strings.TrimSpace(payload.Address) == "" || strings.TrimSpace(payload.NewPassword) == "" {
		writeError(w, http.StatusBadRequest, "Address and new password are required.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Mailbox password updated."})
}

func (s *service) handleMailForwardsList(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.MailForwards})
}

func (s *service) handleMailForwardCreate(w http.ResponseWriter, r *http.Request) {
	var payload MailForward
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid mail forward payload.")
		return
	}
	if payload.Domain == "" || payload.Source == "" || payload.Target == "" {
		writeError(w, http.StatusBadRequest, "Domain, source and target are required.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.MailForwards = append(s.modules.MailForwards, MailForward{Domain: normalizeDomain(payload.Domain), Source: payload.Source, Target: payload.Target})
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Mail forward added."})
}

func (s *service) handleMailForwardDelete(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string `json:"domain"`
		Source string `json:"source"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid mail forward delete payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.modules.MailForwards
	filtered := items[:0]
	deleted := false
	for _, item := range items {
		if item.Domain == normalizeDomain(payload.Domain) && item.Source == payload.Source {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.MailForwards = filtered
	if !deleted {
		writeError(w, http.StatusNotFound, "Mail forward not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Mail forward deleted."})
}

func (s *service) handleMailCatchAllSet(w http.ResponseWriter, r *http.Request) {
	var payload MailCatchAll
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid catch-all payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	if domain == "" {
		writeError(w, http.StatusBadRequest, "Domain is required.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	payload.Domain = domain
	s.modules.MailCatchAll[domain] = payload
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Catch-all updated.", Data: payload})
}

func (s *service) handleMailRoutingList(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.MailRouting})
}

func (s *service) handleMailRoutingCreate(w http.ResponseWriter, r *http.Request) {
	var payload MailRoutingRule
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid mail routing payload.")
		return
	}
	if payload.Domain == "" || payload.Pattern == "" || payload.Target == "" {
		writeError(w, http.StatusBadRequest, "Domain, pattern and target are required.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	payload.ID = firstNonEmpty(payload.ID, generateSecret(6))
	payload.Domain = normalizeDomain(payload.Domain)
	s.modules.MailRouting = append(s.modules.MailRouting, payload)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Mail routing rule saved."})
}

func (s *service) handleMailRoutingDelete(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string `json:"domain"`
		ID     string `json:"id"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid mail routing delete payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.modules.MailRouting
	filtered := items[:0]
	deleted := false
	for _, item := range items {
		if item.Domain == normalizeDomain(payload.Domain) && item.ID == payload.ID {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.MailRouting = filtered
	if !deleted {
		writeError(w, http.StatusNotFound, "Routing rule not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Mail routing rule deleted."})
}

func (s *service) handleMailDKIMGet(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.MailDKIM[domain]})
}

func (s *service) handleMailDKIMRotate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string `json:"domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid DKIM rotation payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	record := DKIMRecord{
		Domain:    domain,
		Selector:  "selector1",
		PublicKey: fmt.Sprintf("v=DKIM1; k=rsa; p=%s", generateSecret(24)),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.MailDKIM[domain] = record
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "DKIM key rotated.", Data: record})
}

func (s *service) handleMailWebmailSSO(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Address    string `json:"address"`
		TTLSeconds int    `json:"ttl_seconds"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid webmail SSO payload.")
		return
	}
	address := strings.TrimSpace(strings.ToLower(payload.Address))
	if address == "" {
		writeError(w, http.StatusBadRequest, "Mailbox address is required.")
		return
	}
	token := generateSecret(12)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.WebmailTokens[token] = WebmailToken{
		Token:     token,
		Address:   address,
		ExpiresAt: time.Now().UTC().Add(time.Duration(maxInt(payload.TTLSeconds, 60)) * time.Second),
	}
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"url": fmt.Sprintf("/api/v1/mail/webmail/sso/consume?token=%s", token),
		},
	})
}

func (s *service) handleMailWebmailConsume(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	s.mu.RLock()
	item, ok := s.modules.WebmailTokens[token]
	s.mu.RUnlock()
	if !ok || item.ExpiresAt.Before(time.Now().UTC()) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte("<html><body><h1>Webmail token expired</h1></body></html>"))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(fmt.Sprintf("<html><body style=\"font-family:sans-serif;background:#0f172a;color:#e2e8f0;padding:32px\"><h1>Roundcube SSO</h1><p>Mailbox: <strong>%s</strong></p><p>Simulation mode active. Token consumed by Go panel-service.</p></body></html>", item.Address)))
}

func (s *service) transferAccountsLocked(kind string) *[]TransferAccount {
	if kind == "sftp" {
		return &s.modules.SFTPUsers
	}
	return &s.modules.FTPUsers
}

func (s *service) handleTransferList(w http.ResponseWriter, r *http.Request, kind string) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	source := *s.transferAccountsLocked(kind)
	items := make([]TransferAccount, 0, len(source))
	for _, item := range source {
		if domain == "" || item.Domain == domain {
			items = append(items, item)
		}
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
}

func (s *service) handleTransferCreate(w http.ResponseWriter, r *http.Request, kind string) {
	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
		HomeDir  string `json:"home_dir"`
		Domain   string `json:"domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid transfer user payload.")
		return
	}
	if payload.Username == "" || payload.Password == "" || payload.HomeDir == "" {
		writeError(w, http.StatusBadRequest, "Username, password and home directory are required.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.transferAccountsLocked(kind)
	*items = append(*items, TransferAccount{
		Username:  sanitizeName(payload.Username),
		Domain:    normalizeDomain(payload.Domain),
		HomeDir:   normalizeVirtualPath(payload.HomeDir),
		CreatedAt: time.Now().UTC().Unix(),
	})
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: strings.ToUpper(kind) + " account created."})
}

func (s *service) handleTransferPassword(w http.ResponseWriter, r *http.Request, kind string) {
	var payload struct {
		Username    string `json:"username"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid password update payload.")
		return
	}
	if payload.Username == "" || payload.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "Username and new password are required.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: strings.ToUpper(kind) + " password updated."})
}

func (s *service) handleTransferDelete(w http.ResponseWriter, r *http.Request, kind string) {
	var payload struct {
		Username string `json:"username"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid transfer delete payload.")
		return
	}
	key := sanitizeName(payload.Username)
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.transferAccountsLocked(kind)
	filtered := (*items)[:0]
	deleted := false
	for _, item := range *items {
		if item.Username == key {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	*items = filtered
	if !deleted {
		writeError(w, http.StatusNotFound, "Transfer account not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: strings.ToUpper(kind) + " account deleted."})
}

func (s *service) handleCronJobsList(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.CronJobs})
}

func (s *service) handleCronJobCreate(w http.ResponseWriter, r *http.Request) {
	var payload CronJob
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid cron job payload.")
		return
	}
	if payload.Command == "" {
		writeError(w, http.StatusBadRequest, "Cron command is required.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	payload.ID = generateSecret(6)
	s.modules.CronJobs = append(s.modules.CronJobs, payload)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Cron job created.", Data: payload})
}

func (s *service) handleCronJobDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.modules.CronJobs
	filtered := items[:0]
	deleted := false
	for _, item := range items {
		if item.ID == id {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.CronJobs = filtered
	if !deleted {
		writeError(w, http.StatusNotFound, "Cron job not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Cron job deleted."})
}

func (s *service) handleOLSTuningGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.OLSConfig})
}

func (s *service) handleOLSTuningSet(w http.ResponseWriter, r *http.Request, apply bool) {
	var payload OLSTuningConfig
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid OLS tuning payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.OLSConfig = payload
	message := "OpenLiteSpeed tuning saved."
	if apply {
		message = "OpenLiteSpeed tuning saved and apply scheduled."
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: message, Data: s.modules.OLSConfig})
}

func (s *service) handleFilesList(w http.ResponseWriter, r *http.Request) {
	path := normalizeVirtualPath(r.URL.Query().Get("path"))
	if path == "/" || path == "" {
		var payload struct {
			Path string `json:"path"`
		}
		if r.Method == http.MethodPost && decodeJSON(r, &payload) == nil {
			path = normalizeVirtualPath(payload.Path)
		}
	}
	if path == "" || path == "/" {
		path = "/home"
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.listVirtualEntriesLocked(path)})
}

func (s *service) handleFileRead(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid file read payload.")
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.getVirtualFileLocked(payload.Path)
	if !ok || item.IsDir {
		writeError(w, http.StatusNotFound, "File not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: item.Content})
}

func (s *service) handleFileWrite(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid file write payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upsertVirtualFileLocked(payload.Path, payload.Content, "0644")
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "File written."})
}

func (s *service) handleFileRename(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid rename payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.moveVirtualPathLocked(payload.OldPath, payload.NewPath)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Path renamed."})
}

func (s *service) handleFileTrash(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid trash payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	source := normalizeVirtualPath(payload.Path)
	dest := normalizeVirtualPath("/home/backups/trash-" + strings.ReplaceAll(strings.Trim(source, "/"), "/", "-"))
	s.moveVirtualPathLocked(source, dest)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Item moved to trash."})
}

func (s *service) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid delete payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteVirtualPathLocked(payload.Path)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Item deleted."})
}

func (s *service) handleFileCompress(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Format   string   `json:"format"`
		DestPath string   `json:"dest_path"`
		Sources  []string `json:"sources"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid compress payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	content := "Compressed archive generated from:\n" + strings.Join(payload.Sources, "\n")
	s.upsertVirtualFileLocked(payload.DestPath, content, "0644")
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Archive created."})
}

func (s *service) handleFileExtract(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ArchivePath string `json:"archive_path"`
		DestDir     string `json:"dest_dir"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid extract payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureVirtualDirLocked(payload.DestDir)
	base := strings.TrimSuffix(virtualBaseName(payload.ArchivePath), ".zip")
	base = strings.TrimSuffix(base, ".tar.gz")
	s.ensureVirtualDirLocked(normalizeVirtualPath(payload.DestDir + "/" + base))
	s.upsertVirtualFileLocked(normalizeVirtualPath(payload.DestDir+"/"+base+"/README.txt"), "Extracted archive content placeholder.\n", "0644")
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Archive extracted."})
}

func (s *service) handleFileCreateDir(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid create directory payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureVirtualDirLocked(payload.Path)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Directory created."})
}

func (s *service) handleBackupDestinationsGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.BackupDestinations})
}

func (s *service) handleBackupDestinationSet(w http.ResponseWriter, r *http.Request) {
	var payload BackupDestination
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid backup destination payload.")
		return
	}
	payload.ID = firstNonEmpty(payload.ID, generateSecret(6))
	s.mu.Lock()
	defer s.mu.Unlock()
	replaced := false
	for i := range s.modules.BackupDestinations {
		if s.modules.BackupDestinations[i].ID == payload.ID {
			s.modules.BackupDestinations[i] = payload
			replaced = true
			break
		}
	}
	if !replaced {
		s.modules.BackupDestinations = append(s.modules.BackupDestinations, payload)
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Backup destination saved.", Data: payload})
}

func (s *service) handleBackupDestinationDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.modules.BackupDestinations
	filtered := items[:0]
	deleted := false
	for _, item := range items {
		if item.ID == id {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.BackupDestinations = filtered
	if !deleted {
		writeError(w, http.StatusNotFound, "Backup destination not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Backup destination deleted."})
}

func (s *service) handleBackupSchedulesGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.BackupSchedules})
}

func (s *service) handleBackupScheduleSet(w http.ResponseWriter, r *http.Request) {
	var payload BackupSchedule
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid backup schedule payload.")
		return
	}
	payload.ID = firstNonEmpty(payload.ID, generateSecret(6))
	s.mu.Lock()
	defer s.mu.Unlock()
	replaced := false
	for i := range s.modules.BackupSchedules {
		if s.modules.BackupSchedules[i].ID == payload.ID {
			s.modules.BackupSchedules[i] = payload
			replaced = true
			break
		}
	}
	if !replaced {
		s.modules.BackupSchedules = append(s.modules.BackupSchedules, payload)
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Backup schedule saved.", Data: payload})
}

func (s *service) handleBackupScheduleDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.modules.BackupSchedules
	filtered := items[:0]
	deleted := false
	for _, item := range items {
		if item.ID == id {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.BackupSchedules = filtered
	if !deleted {
		writeError(w, http.StatusNotFound, "Backup schedule not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Backup schedule deleted."})
}

func (s *service) handleBackupCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain      string `json:"domain"`
		BackupPath  string `json:"backup_path"`
		RemoteRepo  string `json:"remote_repo"`
		Password    string `json:"password"`
		Incremental bool   `json:"incremental"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid backup payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	now := time.Now().UTC()
	snapshot := BackupSnapshot{
		ID:         generateSecret(8),
		ShortID:    generateSecret(4),
		Time:       now.Format(time.RFC3339),
		Hostname:   "aurapanel-dev",
		Tags:       []string{"website", domain},
		Domain:     domain,
		BackupPath: payload.BackupPath,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.BackupSnapshots = append([]BackupSnapshot{snapshot}, s.modules.BackupSnapshots...)
	s.appendActivityLocked("system", "backup_create", fmt.Sprintf("Backup created for %s.", domain), "")
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("Backup started for %s.", domain), Data: snapshot})
}

func (s *service) handleBackupSnapshots(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string `json:"domain"`
	}
	_ = decodeJSON(r, &payload)
	domain := normalizeDomain(payload.Domain)
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]BackupSnapshot, 0, len(s.modules.BackupSnapshots))
	for _, snapshot := range s.modules.BackupSnapshots {
		if domain == "" || snapshot.Domain == domain {
			items = append(items, snapshot)
		}
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
}

func (s *service) handleBackupRestore(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain     string `json:"domain"`
		SnapshotID string `json:"snapshot_id"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid restore payload.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("Restore scheduled for %s from snapshot %s.", payload.Domain, payload.SnapshotID)})
}

func (s *service) handleDBBackupsList(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.DBBackups})
}

func (s *service) handleDBBackupCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		DBName string `json:"db_name"`
		Engine string `json:"engine"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid DB backup payload.")
		return
	}
	engine := normalizeEngine(payload.Engine)
	filename := fmt.Sprintf("%s-%s.sql.gz", sanitizeDBName(payload.DBName), time.Now().UTC().Format("20060102-150405"))
	record := DBBackupRecord{
		ID:        generateSecret(8),
		Filename:  filename,
		Engine:    map[bool]string{true: "mariadb", false: "postgres"}[engine == "mariadb"],
		Size:      "12 MB",
		CreatedAt: time.Now().UTC().UnixMilli(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.DBBackups = append([]DBBackupRecord{record}, s.modules.DBBackups...)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Database backup created.", Data: record})
}

func (s *service) handleDBBackupDownload(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.modules.DBBackups {
		if item.ID == id || item.Filename == id {
			writeBlob(w, item.Filename, "application/gzip", []byte("-- simulated database backup --\n"))
			return
		}
	}
	writeError(w, http.StatusNotFound, "Database backup not found.")
}

func (s *service) handleDBBackupRestore(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		BackupID string `json:"backup_id"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid DB restore payload.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("Database restore queued for %s.", payload.BackupID)})
}

func (s *service) handleDBBackupDelete(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		BackupID string `json:"backup_id"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid DB backup delete payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.modules.DBBackups
	filtered := items[:0]
	deleted := false
	for _, item := range items {
		if item.ID == payload.BackupID || item.Filename == payload.BackupID {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.DBBackups = filtered
	if !deleted {
		writeError(w, http.StatusNotFound, "Database backup not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Database backup deleted."})
}

func (s *service) handleActivityLog(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.ActivityLogs})
}

func (s *service) handleSSLBindings(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.SSLBindings})
}

func (s *service) handleSSLDetails(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string `json:"domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid SSL details payload.")
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	domain := normalizeDomain(payload.Domain)
	detail := s.modules.SSLCertificates[domain]
	if detail.Domain == "" {
		detail = SSLCertificateDetail{Domain: domain, Status: "missing", Issuer: "-", ExpiryDate: "-", DaysRemaining: 0}
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: detail})
}

func (s *service) handleSSLHostnameIssue(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string `json:"domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid hostname SSL payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.SSLBindings.HostnameSSLDomain = domain
	s.modules.SSLBindings.UpdatedAt = time.Now().UTC().Unix()
	s.recordIssuedCertificateLocked(domain, "Let's Encrypt", false)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("Hostname SSL issued for %s.", domain)})
}

func (s *service) handleSSLMailIssue(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string `json:"domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid mail SSL payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.SSLBindings.MailSSLDomain = domain
	s.modules.SSLBindings.UpdatedAt = time.Now().UTC().Unix()
	s.recordIssuedCertificateLocked(domain, "Let's Encrypt", false)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("Mail SSL issued for %s.", domain)})
}

func (s *service) handleSSLWildcardIssue(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string `json:"domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid wildcard SSL payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordIssuedCertificateLocked("*."+domain, "Let's Encrypt", true)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("Wildcard SSL issued for *.%s.", domain)})
}
