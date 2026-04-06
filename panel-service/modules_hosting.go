package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func sanitizeBackupDestination(item BackupDestination) BackupDestination {
	item.Password = ""
	return item
}

func sanitizeBackupDestinations(items []BackupDestination) []BackupDestination {
	if len(items) == 0 {
		return []BackupDestination{}
	}
	sanitized := make([]BackupDestination, 0, len(items))
	for _, item := range items {
		sanitized = append(sanitized, sanitizeBackupDestination(item))
	}
	return sanitized
}

func isAllowedTransferHomeDir(homeDir string) bool {
	normalized := normalizeVirtualPath(homeDir)
	if normalized == "/" || normalized == "/home" {
		return false
	}
	if strings.Contains(normalized, "..") {
		return false
	}
	return strings.HasPrefix(normalized, "/home/")
}

func (s *service) firstInstalledPHPVersionLocked() string {
	for _, item := range s.modules.PHPVersions {
		if item.Installed {
			return item.Version
		}
	}
	return "8.3"
}

func (s *service) handlePHPVersions(w http.ResponseWriter) {
	versions := discoverPHPVersions()
	s.mu.Lock()
	s.modules.PHPVersions = versions
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: versions})
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

	// Kurulum uzun sürebilir, arkada plan işlemi olarak başlatıyoruz
	go func(v string) {
		err := installPHPVersion(v)
		s.mu.Lock()
		defer s.mu.Unlock()
		if err != nil {
			s.appendActivityLocked("system", "php_install_error", fmt.Sprintf("PHP %s kurulumu başarisiz: %v", v, err), "")
			return
		}
		s.modules.PHPVersions = discoverPHPVersions()
		if _, err := os.Stat(detectPHPIniPath(v)); err != nil {
			_ = writeManagedFile(detectPHPIniPath(v), defaultPHPIni(v))
		}
		s.appendActivityLocked("system", "php_install", fmt.Sprintf("PHP %s installed.", v), "")
		s.saveRuntimeStateLocked()
	}(version)

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("PHP %s kurulumu arka planda başladi. Bu işlem birkaç dakika sürebilir.", version)})
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

	go func(v string) {
		err := removePHPVersion(v)
		s.mu.Lock()
		defer s.mu.Unlock()
		if err != nil {
			s.appendActivityLocked("system", "php_remove_error", fmt.Sprintf("PHP %s kaldirilamadi: %v", v, err), "")
			return
		}
		replacement := ""
		for _, item := range discoverPHPVersions() {
			if item.Version != v && item.Installed {
				replacement = item.Version
				break
			}
		}
		if replacement == "" {
			replacement = "8.3"
		}
		s.modules.PHPVersions = discoverPHPVersions()
		for i := range s.state.Websites {
			if s.state.Websites[i].PHPVersion == v || s.state.Websites[i].PHP == v {
				s.state.Websites[i].PHPVersion = replacement
				s.state.Websites[i].PHP = replacement
			}
		}
		for i := range s.state.Subdomains {
			if s.state.Subdomains[i].PHPVersion == v {
				s.state.Subdomains[i].PHPVersion = replacement
			}
		}
		s.appendActivityLocked("system", "php_remove", fmt.Sprintf("PHP %s removed.", v), "")
		s.saveRuntimeStateLocked()
	}(version)

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("PHP %s kaldirma işlemi arka planda başladi.", version)})
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
	if err := restartPHPRuntime(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
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
	version := strings.TrimSpace(payload.Version)
	content, err := readManagedFile(detectPHPIniPath(version))
	if err != nil {
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
	if err := writeManagedFile(detectPHPIniPath(version), payload.Content); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	s.appendActivityLocked("system", "php_ini_save", fmt.Sprintf("php.ini updated for %s.", version), "")
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("php.ini saved for PHP %s.", version)})
}

func (s *service) handleMariaDBTuningGet(w http.ResponseWriter) {
	configPath := "/etc/mysql/mariadb.conf.d/50-server.cnf"
	if !fileExists(configPath) {
		configPath = "/etc/mysql/my.cnf"
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to read MariaDB config")
		return
	}

	lines := strings.Split(string(content), "\n")
	settings := map[string]string{
		"max_connections":         "151",
		"innodb_buffer_pool_size": "128M",
		"key_buffer_size":         "16M",
		"max_allowed_packet":      "16M",
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if _, exists := settings[key]; exists {
			settings[key] = val
		}
	}

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: settings})
}

func (s *service) handleMariaDBTuningSet(w http.ResponseWriter, r *http.Request) {
	var payload map[string]string
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	configPath := "/etc/mysql/mariadb.conf.d/50-server.cnf"
	if !fileExists(configPath) {
		configPath = "/etc/mysql/my.cnf"
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to read config")
		return
	}

	lines := strings.Split(string(content), "\n")
	updatedLines := make([]string, 0, len(lines))
	keysHandled := make(map[string]bool)
	inMysqldSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[mysqld]") {
			inMysqldSection = true
			updatedLines = append(updatedLines, line)
			continue
		}
		if strings.HasPrefix(trimmed, "[") && inMysqldSection {
			for k, v := range payload {
				if !keysHandled[k] {
					updatedLines = append(updatedLines, fmt.Sprintf("%s = %s", k, v))
					keysHandled[k] = true
				}
			}
			inMysqldSection = false
		}

		if inMysqldSection && !strings.HasPrefix(trimmed, "#") && strings.Contains(trimmed, "=") {
			parts := strings.SplitN(trimmed, "=", 2)
			key := strings.TrimSpace(parts[0])
			if val, exists := payload[key]; exists {
				updatedLines = append(updatedLines, fmt.Sprintf("%s = %s", key, val))
				keysHandled[key] = true
				continue
			}
		}
		updatedLines = append(updatedLines, line)
	}

	if inMysqldSection {
		for k, v := range payload {
			if !keysHandled[k] {
				updatedLines = append(updatedLines, fmt.Sprintf("%s = %s", k, v))
			}
		}
	}

	err = os.WriteFile(configPath, []byte(strings.Join(updatedLines, "\n")), 0644)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to write config")
		return
	}

	go func() {
		_ = exec.Command("systemctl", "restart", "mariadb").Run()
	}()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "MariaDB settings updated. Service is restarting in the background."})
}

// PostgreSQL Tuning APIs
func (s *service) handlePostgresTuningGet(w http.ResponseWriter) {
	var configPath string
	matches, _ := filepath.Glob("/etc/postgresql/*/main/postgresql.conf")
	if len(matches) > 0 {
		configPath = matches[0]
	} else {
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: map[string]string{
			"max_connections":      "100",
			"shared_buffers":       "128MB",
			"work_mem":             "4MB",
			"maintenance_work_mem": "64MB",
		}})
		return
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to read PostgreSQL config")
		return
	}

	lines := strings.Split(string(content), "\n")
	settings := map[string]string{
		"max_connections":      "100",
		"shared_buffers":       "128MB",
		"work_mem":             "4MB",
		"maintenance_work_mem": "64MB",
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "'\"")

		if idx := strings.Index(val, "#"); idx != -1 {
			val = strings.TrimSpace(val[:idx])
		}

		if _, exists := settings[key]; exists {
			settings[key] = val
		}
	}

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: settings})
}

func (s *service) handlePostgresTuningSet(w http.ResponseWriter, r *http.Request) {
	var payload map[string]string
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	var configPath string
	matches, _ := filepath.Glob("/etc/postgresql/*/main/postgresql.conf")
	if len(matches) > 0 {
		configPath = matches[0]
	} else {
		writeError(w, http.StatusInternalServerError, "PostgreSQL config file not found. Ensure PostgreSQL is installed.")
		return
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to read config")
		return
	}

	lines := strings.Split(string(content), "\n")
	updatedLines := make([]string, 0, len(lines))
	keysHandled := make(map[string]bool)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		for k, v := range payload {
			if strings.HasPrefix(trimmed, "#"+k) || strings.HasPrefix(trimmed, "# "+k) {
				if !keysHandled[k] {
					updatedLines = append(updatedLines, fmt.Sprintf("%s = '%s'", k, v))
					keysHandled[k] = true
					continue
				}
			}
		}

		if !strings.HasPrefix(trimmed, "#") && strings.Contains(trimmed, "=") {
			parts := strings.SplitN(trimmed, "=", 2)
			key := strings.TrimSpace(parts[0])
			if val, exists := payload[key]; exists {
				updatedLines = append(updatedLines, fmt.Sprintf("%s = '%s'", key, val))
				keysHandled[key] = true
				continue
			}
		}
		updatedLines = append(updatedLines, line)
	}

	for k, v := range payload {
		if !keysHandled[k] {
			updatedLines = append(updatedLines, fmt.Sprintf("%s = '%s'", k, v))
		}
	}

	err = os.WriteFile(configPath, []byte(strings.Join(updatedLines, "\n")), 0644)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to write config")
		return
	}

	go func() {
		_ = exec.Command("systemctl", "restart", "postgresql").Run()
	}()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "PostgreSQL settings updated. Service is restarting in the background."})
}

func (s *service) handleWebsiteAdvancedConfigGet(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	if domain != "" {
		if !isValidDomainName(domain) {
			writeError(w, http.StatusBadRequest, "Invalid domain.")
			return
		}
		if !s.requireDomainAccess(w, r, domain) {
			return
		}
	}
	config := WebsiteAdvancedConfig{}
	if domain != "" {
		s.mu.RLock()
		existing, ok := s.state.AdvancedConfig[domain]
		s.mu.RUnlock()
		if ok {
			config = existing
		} else {
			config = defaultWebsiteAdvancedConfig()
		}
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: config})
}

// Mail Tuning APIs (Postfix & Dovecot)
func (s *service) handleMailTuningGet(w http.ResponseWriter) {
	settings := map[string]string{
		"message_size_limit":                  "10485760", // 10MB default
		"mailbox_size_limit":                  "51200000", // 50MB default
		"smtpd_client_connection_count_limit": "50",
	}

	// Read Postfix main.cf
	if content, err := os.ReadFile("/etc/postfix/main.cf"); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if _, exists := settings[key]; exists {
				settings[key] = val
			}
		}
	}

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: settings})
}

func (s *service) handleMailTuningSet(w http.ResponseWriter, r *http.Request) {
	var payload map[string]string
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	// Apply to Postfix using postconf
	for k, v := range payload {
		if k == "message_size_limit" || k == "mailbox_size_limit" || k == "smtpd_client_connection_count_limit" {
			_ = exec.Command("postconf", "-e", fmt.Sprintf("%s=%s", k, v)).Run()
		}
	}

	go func() {
		_ = exec.Command("systemctl", "restart", "postfix").Run()
	}()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Mail server settings updated. Postfix is restarting in the background."})
}

func (s *service) handleWebsiteCustomSSLGet(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	if !isValidDomainName(domain) {
		writeError(w, http.StatusBadRequest, "Invalid domain.")
		return
	}
	if !s.requireDomainAccess(w, r, domain) {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	current := s.state.CustomSSL[domain]
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: WebsiteCustomSSL{
			CertPEM: current.CertPEM,
			KeyPEM:  "",
		},
	})
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
	if !isValidDomainName(domain) {
		writeError(w, http.StatusBadRequest, "Invalid domain.")
		return
	}
	if !s.requireDomainAccess(w, r, domain) {
		return
	}
	if err := storeCustomCertificate(domain, payload.CertPEM, payload.KeyPEM); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.CustomSSL[domain] = WebsiteCustomSSL{CertPEM: payload.CertPEM, KeyPEM: payload.KeyPEM}
	if payload.CertPEM != "" && payload.KeyPEM != "" {
		s.modules.SSLCertificates[domain] = inspectCertificate(domain)
	}
	if err := s.syncOLSVhostsLocked(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.appendActivityLocked("system", "ssl_custom", fmt.Sprintf("Custom SSL stored for %s.", domain), "")
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("Custom SSL saved for %s.", domain)})
}

func (s *service) handleWebsiteOpenBasedirSet(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain      string `json:"domain"`
		Enabled     *bool  `json:"enabled"`
		OpenBasedir *bool  `json:"open_basedir"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid open_basedir payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	if !isValidDomainName(domain) {
		writeError(w, http.StatusBadRequest, "Invalid domain.")
		return
	}
	if !s.requireDomainAccess(w, r, domain) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultSiteArtifactsLocked(domain)
	cfg := s.state.AdvancedConfig[domain]
	enabled := false
	switch {
	case payload.Enabled != nil:
		enabled = *payload.Enabled
	case payload.OpenBasedir != nil:
		enabled = *payload.OpenBasedir
	}
	cfg.OpenBasedir = enabled
	s.state.AdvancedConfig[domain] = cfg
	if err := s.syncOLSVhostsLocked(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.saveRuntimeStateLocked(); err != nil {
		writeError(w, http.StatusInternalServerError, "open_basedir update could not be persisted.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Open Basedir updated.", Data: cfg})
}

func (s *service) handleWebsiteRewriteSet(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain       string `json:"domain"`
		Rules        string `json:"rules"`
		RewriteRules string `json:"rewrite_rules"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid rewrite payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	if !isValidDomainName(domain) {
		writeError(w, http.StatusBadRequest, "Invalid domain.")
		return
	}
	if !s.requireDomainAccess(w, r, domain) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultSiteArtifactsLocked(domain)
	resolvedRules := firstNonEmpty(payload.Rules, payload.RewriteRules)
	if err := validateWebsiteRewriteRules(domain, resolvedRules); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg := s.state.AdvancedConfig[domain]
	previousRules := cfg.RewriteRules
	cfg.RewriteRules = resolvedRules
	s.state.AdvancedConfig[domain] = cfg
	if err := applyWebsiteRewriteRules(domain, resolvedRules); err != nil {
		cfg.RewriteRules = previousRules
		s.state.AdvancedConfig[domain] = cfg
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.syncOLSVhostsLocked(); err != nil {
		cfg.RewriteRules = previousRules
		s.state.AdvancedConfig[domain] = cfg
		_ = applyWebsiteRewriteRules(domain, previousRules)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.saveRuntimeStateLocked(); err != nil {
		cfg.RewriteRules = previousRules
		s.state.AdvancedConfig[domain] = cfg
		_ = applyWebsiteRewriteRules(domain, previousRules)
		writeError(w, http.StatusInternalServerError, "rewrite update could not be persisted.")
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Rewrite rules updated.", Data: cfg})
}

func (s *service) handleWebsiteVhostConfigSet(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain      string `json:"domain"`
		Content     string `json:"content"`
		VhostConfig string `json:"vhost_config"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid vhost config payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	if !isValidDomainName(domain) {
		writeError(w, http.StatusBadRequest, "Invalid domain.")
		return
	}
	if !s.requireDomainAccess(w, r, domain) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultSiteArtifactsLocked(domain)
	resolvedContent := firstNonEmpty(payload.Content, payload.VhostConfig)
	cfg := s.state.AdvancedConfig[domain]
	cfg.VhostConfig = resolvedContent
	s.state.AdvancedConfig[domain] = cfg
	if err := s.syncOLSVhostsLocked(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.saveRuntimeStateLocked(); err != nil {
		writeError(w, http.StatusInternalServerError, "vhost config update could not be persisted.")
		return
	}
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
	owner := s.resolveRequestedOwner(r, payload.Owner)
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
	if err := s.enforceOwnerDomainsLimitLocked(owner); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	s.state.Websites = append(s.state.Websites, Website{
		Domain:        fqdn,
		Owner:         owner,
		User:          owner,
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
	if err := s.syncOLSVhostsLocked(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
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
	if !isValidDomainName(domain) || !isValidDomainName(alias) {
		writeError(w, http.StatusBadRequest, "Domain and alias are required.")
		return
	}
	if !s.requireDomainAccess(w, r, domain) {
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
	if err := s.syncOLSVhostsLocked(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Alias added."})
}

func (s *service) handleAliasDelete(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	alias := normalizeDomain(r.URL.Query().Get("alias"))
	if !isValidDomainName(domain) || !isValidDomainName(alias) {
		writeError(w, http.StatusBadRequest, "Domain and alias are required.")
		return
	}
	if !s.requireDomainAccess(w, r, domain) {
		return
	}
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
	if err := s.syncOLSVhostsLocked(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Alias deleted."})
}

func (s *service) handleWebsiteTraffic(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	if !isValidDomainName(domain) {
		writeError(w, http.StatusBadRequest, "Domain is required.")
		return
	}
	if !s.requireDomainAccess(w, r, domain) {
		return
	}
	hours := clampInt(queryInt(r, "hours", 24), 1, 168)
	data, err := collectWebsiteTraffic(domain, hours)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data:   data,
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
		Domain   string `json:"domain"`
		ServerIP string `json:"server_ip"`
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
	serverIP := strings.TrimSpace(payload.ServerIP)
	if net.ParseIP(serverIP) == nil {
		serverIP = detectPrimaryIPv4()
		if strings.TrimSpace(serverIP) == "" {
			serverIP = "127.0.0.1"
		}
	}
	s.ensureDNSArtifactsLocked(domain, true)
	s.upsertDNSRecordLocked(domain, DNSRecord{RecordType: "A", Name: "@", Content: serverIP, TTL: 3600})
	s.upsertDNSRecordLocked(domain, DNSRecord{RecordType: "A", Name: "www", Content: serverIP, TTL: 3600})
	s.upsertDNSRecordLocked(domain, DNSRecord{RecordType: "A", Name: "ftp", Content: serverIP, TTL: 3600})
	s.upsertDNSRecordLocked(domain, DNSRecord{RecordType: "A", Name: "panel", Content: serverIP, TTL: 3600})
	s.upsertDNSRecordLocked(domain, DNSRecord{RecordType: "A", Name: "mail", Content: serverIP, TTL: 3600})
	s.recalcDNSZoneLocked(domain)
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
	s.ensureDNSArtifactsLocked(domain, true)
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

func (s *service) handleMailboxesList(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}
	s.mu.RLock()
	quotaByAddress := map[string]int{}
	ownerByAddress := map[string]string{}
	for _, mailbox := range s.modules.Mailboxes {
		quotaByAddress[mailbox.Address] = mailbox.QuotaMB
		ownerByAddress[strings.ToLower(strings.TrimSpace(mailbox.Address))] = sanitizeName(mailbox.Owner)
	}
	s.mu.RUnlock()
	items := loadSystemMailboxes(quotaByAddress)
	if principal.Role != "admin" {
		ids := principalAliases(principal)
		s.mu.RLock()
		if user := s.findUserByEmailLocked(principal.Email); user != nil {
			ids[sanitizeName(user.Username)] = struct{}{}
		}
		s.mu.RUnlock()
		filtered := make([]Mailbox, 0, len(items))
		for _, item := range items {
			allowedDomain := false
			if normalizeDomain(item.Domain) != "" {
				s.mu.RLock()
				allowedDomain = s.canAccessDomainLocked(principal, item.Domain)
				s.mu.RUnlock()
			}
			if allowedDomain {
				filtered = append(filtered, item)
				continue
			}
			if owner := ownerByAddress[strings.ToLower(strings.TrimSpace(item.Address))]; owner != "" {
				if _, ok := ids[owner]; ok {
					filtered = append(filtered, item)
				}
			}
		}
		items = filtered
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
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
	if strings.TrimSpace(payload.Password) == "" {
		writeError(w, http.StatusBadRequest, "Mailbox password is required.")
		return
	}
	owner := s.resolveRequestedOwner(r, payload.Owner)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.enforceOwnerEmailsLimitLocked(owner); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
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
		Owner:   owner,
		QuotaMB: maxInt(payload.QuotaMB, 256),
		UsedMB:  0,
	})
	if err := upsertSystemMailbox(address, payload.Password); err != nil {
		s.modules.Mailboxes = s.modules.Mailboxes[:len(s.modules.Mailboxes)-1]
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
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
	if err := deleteSystemMailbox(address); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
	if err := updateSystemMailboxPassword(payload.Address, payload.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Mailbox password updated."})
}

func (s *service) handleMailForwardsList(w http.ResponseWriter) {
	s.mu.RLock()
	items := append([]MailForward(nil), s.modules.MailForwards...)
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
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
	if err := upsertSystemForward(payload.Domain, payload.Source, payload.Target); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
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
	if err := deleteSystemForward(payload.Domain, payload.Source); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
	if err := setSystemCatchAll(domain, payload.Target, payload.Enabled); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
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
	baseURL := s.resolveWebmailBaseURL(r)
	if strings.Contains(baseURL, "?") {
		baseURL += "&"
	} else {
		baseURL += "?"
	}
	http.Redirect(w, r, fmt.Sprintf("%s_task=login&_action=login&_user=%s&_autologin_token=%s", baseURL, url.QueryEscape(item.Address), url.QueryEscape(token)), http.StatusFound)
}

func (s *service) resolveWebmailBaseURL(r *http.Request) string {
	baseURL := strings.TrimSpace(os.Getenv("AURAPANEL_WEBMAIL_BASE_URL"))
	if baseURL != "" {
		return baseURL
	}

	s.mu.RLock()
	websiteDomain := ""
	for _, site := range s.state.Websites {
		domain := normalizeDomain(site.Domain)
		if domain == "" {
			continue
		}
		if certPath, keyPath := findCertificatePair(domain); certPath != "" && keyPath != "" {
			websiteDomain = domain
			break
		}
	}
	hostnameSSLDomain := normalizeDomain(s.modules.SSLBindings.HostnameSSLDomain)
	s.mu.RUnlock()
	if websiteDomain != "" {
		return fmt.Sprintf("https://%s/webmail/index.php", websiteDomain)
	}
	if hostnameSSLDomain != "" {
		return fmt.Sprintf("https://%s/webmail/index.php", hostnameSSLDomain)
	}

	host := forwardedHeaderValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" {
		return "/webmail/index.php"
	}

	originalHost := host
	if parsedHost, _, err := net.SplitHostPort(host); err == nil && parsedHost != "" {
		host = parsedHost
	}

	scheme := forwardedHeaderValue(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "https"
	}
	if strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1" {
		scheme = "http"
		host = originalHost
	}

	return fmt.Sprintf("%s://%s/webmail/index.php", scheme, host)
}

func forwardedHeaderValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, ","); idx >= 0 {
		value = value[:idx]
	}
	return strings.TrimSpace(value)
}

func (s *service) handleMailWebmailVerify(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	s.mu.Lock()
	item, ok := s.modules.WebmailTokens[token]
	if ok {
		delete(s.modules.WebmailTokens, token)
	}
	s.mu.Unlock()
	if !ok || item.ExpiresAt.Before(time.Now().UTC()) {
		writeError(w, http.StatusUnauthorized, "Token invalid or expired")
		return
	}
	masterPass := strings.TrimSpace(os.Getenv("AURAPANEL_MAIL_MASTER_PASS"))
	if masterPass == "" {
		masterPass = readEnvFileValue("/etc/aurapanel/aurapanel.env", "AURAPANEL_MAIL_MASTER_PASS")
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"address":     item.Address,
		"master_pass": masterPass,
	})
}

func (s *service) transferAccountsLocked(kind string) *[]TransferAccount {
	if kind == "sftp" {
		return &s.modules.SFTPUsers
	}
	return &s.modules.FTPUsers
}

func (s *service) normalizeTransferAccountLocked(kind string, account TransferAccount) TransferAccount {
	account.Username = sanitizeName(account.Username)
	account.Domain = normalizeDomain(account.Domain)
	account.HomeDir = normalizeVirtualPath(account.HomeDir)
	if account.Domain == "" {
		account.Domain = inferTransferDomainFromHomeDir(account.HomeDir)
	}
	if kind == "ftp" && account.Domain != "" && sanitizeName(account.Username) == primaryFTPUsernameForDomain(account.Domain) {
		account.Primary = true
	}
	return account
}

func (s *service) transferOwnerHint(kind string, account TransferAccount) string {
	if kind != "ftp" {
		return ""
	}
	domain := normalizeDomain(account.Domain)
	if domain == "" {
		domain = inferTransferDomainFromHomeDir(account.HomeDir)
	}
	if domain == "" {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.websiteOwnerForDomainLocked(domain)
}

func (s *service) syncRuntimeTransferAccountsLocked(kind string) ([]TransferAccount, error) {
	source, err := runtimeTransferAccounts(kind)
	if err != nil {
		return nil, err
	}
	source = mergeTransferMetadata(source, *s.transferAccountsLocked(kind))
	for i := range source {
		source[i] = s.normalizeTransferAccountLocked(kind, source[i])
	}
	*s.transferAccountsLocked(kind) = source
	return source, nil
}

func (s *service) transferAccountAccessibleToPrincipal(principal servicePrincipal, account TransferAccount) bool {
	if principal.Role == "admin" {
		return true
	}
	domain := normalizeDomain(account.Domain)
	if domain != "" {
		s.mu.RLock()
		allowed := s.canAccessDomainLocked(principal, domain)
		s.mu.RUnlock()
		if allowed {
			return true
		}
	}
	if strings.TrimSpace(account.HomeDir) != "" {
		return s.nonAdminCanAccessManagedFilePath(principal, account.HomeDir)
	}
	return false
}

func (s *service) handleTransferList(w http.ResponseWriter, r *http.Request, kind string) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	s.mu.Lock()
	source, err := s.syncRuntimeTransferAccountsLocked(kind)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	items := make([]TransferAccount, 0, len(source))
	for _, item := range source {
		if domain != "" && normalizeDomain(item.Domain) != domain {
			continue
		}
		if principal.Role != "admin" && !s.transferAccountAccessibleToPrincipal(principal, item) {
			continue
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
}

func (s *service) handleTransferCreate(w http.ResponseWriter, r *http.Request, kind string) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}
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
	account := TransferAccount{
		Username:  sanitizeName(payload.Username),
		Domain:    normalizeDomain(payload.Domain),
		HomeDir:   normalizeVirtualPath(payload.HomeDir),
		CreatedAt: time.Now().UTC().Unix(),
	}
	s.mu.Lock()
	account = s.normalizeTransferAccountLocked(kind, account)
	s.mu.Unlock()
	if !isAllowedTransferHomeDir(account.HomeDir) {
		writeError(w, http.StatusBadRequest, "Home directory must be under /home/<account>/...")
		return
	}
	if kind == "ftp" && account.Primary {
		writeError(w, http.StatusConflict, "Primary FTP username is reserved. Use password reset for primary account.")
		return
	}
	if principal.Role != "admin" && !s.transferAccountAccessibleToPrincipal(principal, account) {
		writeError(w, http.StatusForbidden, "Access denied for this transfer home directory.")
		return
	}
	ownerHint := s.transferOwnerHint(kind, account)
	if err := createRuntimeTransferAccount(kind, account.Username, payload.Password, account.HomeDir, ownerHint); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.transferAccountsLocked(kind)
	*items = append(removeTransferAccountByUsername(*items, account.Username), account)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: strings.ToUpper(kind) + " account created.", Data: account})
}

func (s *service) handleTransferPassword(w http.ResponseWriter, r *http.Request, kind string) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}
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
	key := sanitizeName(payload.Username)
	if principal.Role != "admin" {
		s.mu.Lock()
		source, err := s.syncRuntimeTransferAccountsLocked(kind)
		s.mu.Unlock()
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		target := TransferAccount{}
		found := false
		for _, item := range source {
			if sanitizeName(item.Username) == key {
				target = item
				found = true
				break
			}
		}
		if !found {
			writeError(w, http.StatusNotFound, "Transfer account not found.")
			return
		}
		if !s.transferAccountAccessibleToPrincipal(principal, target) {
			writeError(w, http.StatusForbidden, "Access denied for this transfer account.")
			return
		}
	}
	if err := updateRuntimeTransferPassword(kind, key, payload.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.transferAccountsLocked(kind)
	for i := range *items {
		if (*items)[i].Username == key {
			writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: strings.ToUpper(kind) + " password updated."})
			return
		}
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: strings.ToUpper(kind) + " password updated."})
}

func (s *service) handleTransferDelete(w http.ResponseWriter, r *http.Request, kind string) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}
	var payload struct {
		Username string `json:"username"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid transfer delete payload.")
		return
	}
	key := sanitizeName(payload.Username)
	s.mu.Lock()
	source, err := s.syncRuntimeTransferAccountsLocked(kind)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	target := TransferAccount{}
	found := false
	for _, item := range source {
		if sanitizeName(item.Username) == key {
			target = item
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "Transfer account not found.")
		return
	}
	if principal.Role != "admin" && !s.transferAccountAccessibleToPrincipal(principal, target) {
		writeError(w, http.StatusForbidden, "Access denied for this transfer account.")
		return
	}
	if kind == "ftp" && target.Primary {
		writeError(w, http.StatusForbidden, "Primary FTP account cannot be deleted. Only password reset is allowed.")
		return
	}
	if err := deleteRuntimeTransferAccount(kind, key); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.transferAccountsLocked(kind)
	*items = removeTransferAccountByUsername(*items, key)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: strings.ToUpper(kind) + " account deleted."})
}

// Pure-FTPd Tuning APIs
func (s *service) handleFTPTuningGet(w http.ResponseWriter) {
	settings := map[string]string{
		"PassivePortRange": "30000 30049", // default
		"TLS":              "2",           // 2=TLS required, 1=TLS optional, 0=No TLS
		"MaxClientsNumber": "50",
	}

	// Read Pure-FTPd conf files which are stored as individual files per setting in /etc/pure-ftpd/conf/
	for key := range settings {
		path := filepath.Join("/etc/pure-ftpd/conf", key)
		if content, err := os.ReadFile(path); err == nil {
			settings[key] = strings.TrimSpace(string(content))
		}
	}

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: settings})
}

func (s *service) handleFTPTuningSet(w http.ResponseWriter, r *http.Request) {
	var payload map[string]string
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	for key, value := range payload {
		// Basic validation to prevent arbitrary file writes
		if key == "PassivePortRange" || key == "TLS" || key == "MaxClientsNumber" {
			path := filepath.Join("/etc/pure-ftpd/conf", key)
			_ = os.WriteFile(path, []byte(strings.TrimSpace(value)+"\n"), 0644)
		}
	}

	go func() {
		_ = exec.Command("systemctl", "restart", "pure-ftpd").Run()
	}()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "FTP settings updated. Service is restarting in the background."})
}

func (s *service) handleCronJobsList(w http.ResponseWriter) {
	jobs, err := runtimeCronJobs()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.mu.Lock()
	s.modules.CronJobs = jobs
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: jobs})
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
	payload.User = firstNonEmpty(strings.TrimSpace(payload.User), "root")
	payload.ID = sanitizeName(firstNonEmpty(payload.ID, generateSecret(6)))
	if err := createRuntimeCronJob(payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.CronJobs = append(removeCronJobByID(s.modules.CronJobs, payload.ID), payload)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Cron job created.", Data: payload})
}

func (s *service) handleCronJobDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "Cron job id is required.")
		return
	}
	if err := deleteRuntimeCronJob(id); err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "Cron job not found.")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.CronJobs = removeCronJobByID(s.modules.CronJobs, id)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Cron job deleted."})
}

func (s *service) handleOLSTuningGet(w http.ResponseWriter) {
	s.mu.RLock()
	pending := s.modules.OLSTuningPending
	staged := s.modules.OLSConfig
	s.mu.RUnlock()
	if pending {
		runtimeErr := ""
		if _, err := runtimeOLSTuningConfig(); err != nil {
			runtimeErr = err.Error()
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Status: "success",
			Data:   olsTuningResponseData(staged, true, runtimeErr),
		})
		return
	}

	cfg, err := runtimeOLSTuningConfig()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.mu.Lock()
	s.modules.OLSConfig = cfg
	s.modules.OLSTuningPending = false
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: olsTuningResponseData(cfg, false, "")})
}

func (s *service) handleOLSTuningSet(w http.ResponseWriter, r *http.Request, apply bool) {
	var payload OLSTuningConfig
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid OLS tuning payload.")
		return
	}

	s.mu.Lock()
	s.modules.OLSConfig = payload
	s.modules.OLSTuningPending = true
	s.mu.Unlock()

	if !apply {
		writeJSON(w, http.StatusOK, apiResponse{
			Status:  "success",
			Message: "OpenLiteSpeed tuning saved. Apply changes to reload the runtime.",
			Data:    olsTuningResponseData(payload, true, ""),
		})
		return
	}

	if err := applyOLSTuningConfig(payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	s.modules.OLSConfig = payload
	s.modules.OLSTuningPending = false
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: "OpenLiteSpeed tuning applied.",
		Data:    olsTuningResponseData(payload, false, ""),
	})
}

func olsTuningResponseData(cfg OLSTuningConfig, pending bool, runtimeErr string) map[string]interface{} {
	data := map[string]interface{}{
		"max_connections":           cfg.MaxConnections,
		"max_ssl_connections":       cfg.MaxSSLConnections,
		"conn_timeout_secs":         cfg.ConnTimeoutSecs,
		"keep_alive_timeout_secs":   cfg.KeepAliveTimeoutSecs,
		"max_keep_alive_requests":   cfg.MaxKeepAliveRequests,
		"gzip_compression":          cfg.GzipCompression,
		"static_cache_enabled":      cfg.StaticCacheEnabled,
		"static_cache_max_age_secs": cfg.StaticCacheMaxAgeSec,
		"pending":                   pending,
	}
	if runtimeErr != "" {
		data["runtime_error"] = runtimeErr
	}
	return data
}

func managedPathWithinRoot(path, root string) bool {
	normalizedPath := filepath.Clean(strings.TrimSpace(path))
	normalizedRoot := filepath.Clean(strings.TrimSpace(root))
	if normalizedPath == "." || normalizedRoot == "." || normalizedRoot == "" {
		return false
	}
	return normalizedPath == normalizedRoot || strings.HasPrefix(normalizedPath, normalizedRoot+string(os.PathSeparator))
}

func (s *service) nonAdminOwnedFileRoots(principal servicePrincipal) map[string]struct{} {
	roots := map[string]struct{}{}
	if principal.Role == "admin" {
		return roots
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, site := range s.state.Websites {
		if !s.principalOwnsWebsiteLocked(principal, site) {
			continue
		}
		domain := normalizeDomain(site.Domain)
		if domain == "" {
			continue
		}
		root := filepath.Clean(filepath.Join("/home", domain))
		roots[root] = struct{}{}
	}

	return roots
}

func (s *service) nonAdminCanAccessManagedFilePath(principal servicePrincipal, path string) bool {
	if principal.Role == "admin" {
		return true
	}
	resolved, err := resolveManagedPath(path)
	if err != nil {
		return false
	}
	roots := s.nonAdminOwnedFileRoots(principal)
	if len(roots) == 0 {
		return false
	}
	for root := range roots {
		if managedPathWithinRoot(resolved, root) {
			return true
		}
	}
	return false
}

func filterHomeEntriesForRoots(entries []virtualFileEntry, roots map[string]struct{}) []virtualFileEntry {
	if len(entries) == 0 || len(roots) == 0 {
		return []virtualFileEntry{}
	}
	allowedNames := map[string]struct{}{}
	for root := range roots {
		name := strings.ToLower(strings.TrimSpace(filepath.Base(root)))
		if name == "" || name == "." || name == "/" {
			continue
		}
		allowedNames[name] = struct{}{}
	}
	filtered := make([]virtualFileEntry, 0, len(entries))
	for _, item := range entries {
		if !item.IsDir {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(item.Name))
		if _, ok := allowedNames[name]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (s *service) resolveFilePathForPrincipal(principal servicePrincipal, path string) (string, int, error) {
	resolved, err := resolveManagedPath(path)
	if err != nil {
		return "", http.StatusBadRequest, err
	}
	if principal.Role == "admin" {
		return resolved, http.StatusOK, nil
	}
	if !s.nonAdminCanAccessManagedFilePath(principal, resolved) {
		return "", http.StatusForbidden, fmt.Errorf("Access denied for this path.")
	}
	return resolved, http.StatusOK, nil
}

func (s *service) handleFilesList(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		var payload struct {
			Path string `json:"path"`
		}
		if r.Method == http.MethodPost && decodeJSON(r, &payload) == nil {
			path = strings.TrimSpace(payload.Path)
		}
	}
	if path == "" {
		path = "/home"
	}

	resolvedPath, err := resolveManagedPath(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if principal.Role != "admin" {
		roots := s.nonAdminOwnedFileRoots(principal)
		if len(roots) == 0 {
			writeError(w, http.StatusForbidden, "No owned websites available for file access.")
			return
		}
		if resolvedPath == filepath.Clean("/home") {
			items, listErr := listManagedEntries(resolvedPath)
			if listErr != nil {
				writeError(w, http.StatusBadRequest, listErr.Error())
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{
				Status: "success",
				Data:   filterHomeEntriesForRoots(items, roots),
			})
			return
		}
		if !s.nonAdminCanAccessManagedFilePath(principal, resolvedPath) {
			writeError(w, http.StatusForbidden, "Access denied for this path.")
			return
		}
	}

	items, err := listManagedEntries(resolvedPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
}

func (s *service) handleFileRead(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	var payload struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid file read payload.")
		return
	}
	resolvedPath, status, err := s.resolveFilePathForPrincipal(principal, payload.Path)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	content, err := readManagedFile(resolvedPath)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: content})
}

func (s *service) handleFileWrite(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	var payload struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid file write payload.")
		return
	}
	resolvedPath, status, err := s.resolveFilePathForPrincipal(principal, payload.Path)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	if err := writeManagedFile(resolvedPath, payload.Content); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "File written."})
}

func (s *service) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	const maxUploadBytes = 250 << 20 // 250 MB

	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		errText := strings.ToLower(err.Error())
		switch {
		case strings.Contains(errText, "request body too large"), strings.Contains(errText, "too large"):
			writeError(w, http.StatusRequestEntityTooLarge, "Upload is too large for current limits.")
		case strings.Contains(errText, "multipart"), strings.Contains(errText, "boundary"):
			writeError(w, http.StatusBadRequest, "Upload could not be parsed. Use multipart/form-data upload.")
		default:
			writeError(w, http.StatusBadRequest, "Invalid multipart upload payload.")
		}
		return
	}

	targetPath := strings.TrimSpace(r.FormValue("path"))
	if targetPath == "" {
		targetPath = "/home"
	}

	destDir, status, err := s.resolveFilePathForPrincipal(principal, targetPath)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}

	if stat, statErr := os.Stat(destDir); statErr != nil || !stat.IsDir() {
		writeError(w, http.StatusBadRequest, "Upload target must be an existing directory.")
		return
	}

	fileHeaders := r.MultipartForm.File["files"]
	if len(fileHeaders) == 0 {
		fileHeaders = r.MultipartForm.File["file"]
	}
	if len(fileHeaders) == 0 {
		writeError(w, http.StatusBadRequest, "No files selected for upload.")
		return
	}

	uploaded := 0
	for _, header := range fileHeaders {
		name := strings.TrimSpace(filepath.Base(header.Filename))
		if name == "" {
			continue
		}

		src, openErr := header.Open()
		if openErr != nil {
			writeError(w, http.StatusBadRequest, "Unable to read uploaded file.")
			return
		}

		destPath, resolveErr := resolveManagedPath(filepath.Join(destDir, name))
		if resolveErr != nil {
			_ = src.Close()
			writeError(w, http.StatusBadRequest, resolveErr.Error())
			return
		}

		dst, createErr := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if createErr != nil {
			_ = src.Close()
			writeError(w, http.StatusBadRequest, "Unable to create destination file.")
			return
		}

		if _, copyErr := io.Copy(dst, src); copyErr != nil {
			_ = dst.Close()
			_ = src.Close()
			writeError(w, http.StatusBadRequest, "Failed to save uploaded file.")
			return
		}

		_ = dst.Close()
		_ = src.Close()
		applyManagedPathOwnershipFromParent(destPath)
		uploaded++
	}

	if uploaded == 0 {
		writeError(w, http.StatusBadRequest, "No valid files selected for upload.")
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: fmt.Sprintf("%d file(s) uploaded.", uploaded),
	})
}

func (s *service) handleFileRename(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	var payload struct {
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid rename payload.")
		return
	}
	oldPath, status, err := s.resolveFilePathForPrincipal(principal, payload.OldPath)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	newPath, status, err := s.resolveFilePathForPrincipal(principal, payload.NewPath)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	if err := renameManagedPath(oldPath, newPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Path renamed."})
}

func (s *service) handleFileChmod(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	var payload struct {
		Path string `json:"path"`
		Mode string `json:"mode"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid chmod payload.")
		return
	}
	resolvedPath, status, err := s.resolveFilePathForPrincipal(principal, payload.Path)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	if err := setManagedPermissions(resolvedPath, payload.Mode); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Permissions updated."})
}

func (s *service) handleFileTrash(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	var payload struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid trash payload.")
		return
	}
	resolvedPath, status, err := s.resolveFilePathForPrincipal(principal, payload.Path)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	if err := trashManagedPath(resolvedPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Item moved to trash."})
}

func (s *service) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	var payload struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid delete payload.")
		return
	}
	resolvedPath, status, err := s.resolveFilePathForPrincipal(principal, payload.Path)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	if err := deleteManagedPath(resolvedPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Item deleted."})
}

func (s *service) handleFileCompress(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	var payload struct {
		Format   string   `json:"format"`
		DestPath string   `json:"dest_path"`
		Sources  []string `json:"sources"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid compress payload.")
		return
	}
	resolvedDest, status, err := s.resolveFilePathForPrincipal(principal, payload.DestPath)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	resolvedSources := make([]string, 0, len(payload.Sources))
	for _, source := range payload.Sources {
		resolvedSource, sourceStatus, resolveErr := s.resolveFilePathForPrincipal(principal, source)
		if resolveErr != nil {
			writeError(w, sourceStatus, resolveErr.Error())
			return
		}
		resolvedSources = append(resolvedSources, resolvedSource)
	}
	if err := compressManagedFiles(resolvedDest, resolvedSources, payload.Format); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Archive created."})
}

func (s *service) handleFileExtract(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	var payload struct {
		ArchivePath string `json:"archive_path"`
		DestDir     string `json:"dest_dir"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid extract payload.")
		return
	}
	resolvedArchive, status, err := s.resolveFilePathForPrincipal(principal, payload.ArchivePath)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	resolvedDest, status, err := s.resolveFilePathForPrincipal(principal, payload.DestDir)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	if err := extractManagedArchive(resolvedArchive, resolvedDest); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Archive extracted."})
}

func (s *service) handleFileCreateDir(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	var payload struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid create directory payload.")
		return
	}
	resolvedPath, status, err := s.resolveFilePathForPrincipal(principal, payload.Path)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	if err := createManagedDir(resolvedPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Directory created."})
}

func backupSnapshotTimestamp(item BackupSnapshot) int64 {
	if item.CreatedAt > 0 {
		return item.CreatedAt
	}
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(item.Time)); err == nil {
		return parsed.UTC().UnixMilli()
	}
	return 0
}

func (s *service) backupDestinationByIDLocked(id string) (BackupDestination, bool) {
	lookup := strings.TrimSpace(id)
	if lookup == "" {
		return BackupDestination{}, false
	}
	for _, item := range s.modules.BackupDestinations {
		if item.ID == lookup {
			return item, true
		}
	}
	return BackupDestination{}, false
}

func (s *service) enforceBackupRetentionLocked(domain string, keep int) (int, error) {
	targetDomain := normalizeDomain(domain)
	if targetDomain == "" {
		return 0, nil
	}
	keep = normalizeBackupRetentionKeep(keep)
	domainItems := make([]BackupSnapshot, 0, len(s.modules.BackupSnapshots))
	for _, snapshot := range s.modules.BackupSnapshots {
		if snapshot.Domain == targetDomain {
			domainItems = append(domainItems, snapshot)
		}
	}
	if len(domainItems) <= keep {
		return 0, nil
	}

	sort.Slice(domainItems, func(i, j int) bool {
		return backupSnapshotTimestamp(domainItems[i]) > backupSnapshotTimestamp(domainItems[j])
	})
	pruned := domainItems[keep:]
	prunedIDs := make(map[string]struct{}, len(pruned))
	var firstErr error
	for _, item := range pruned {
		prunedIDs[item.ID] = struct{}{}
		path := filepath.Clean(strings.TrimSpace(item.BackupPath))
		if path == "" || !isSiteBackupPathAllowed(path, targetDomain) {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = fmt.Errorf("retention cleanup failed for %s: %w", filepath.Base(path), err)
		}
	}

	filtered := s.modules.BackupSnapshots[:0]
	removed := 0
	for _, snapshot := range s.modules.BackupSnapshots {
		if snapshot.Domain == targetDomain {
			if _, ok := prunedIDs[snapshot.ID]; ok {
				removed++
				continue
			}
		}
		filtered = append(filtered, snapshot)
	}
	s.modules.BackupSnapshots = filtered
	return removed, firstErr
}

func (s *service) handleBackupDestinationsGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: sanitizeBackupDestinations(s.modules.BackupDestinations)})
}

func (s *service) handleBackupDestinationSet(w http.ResponseWriter, r *http.Request) {
	var payload BackupDestination
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid backup destination payload.")
		return
	}
	payload.ID = firstNonEmpty(payload.ID, generateSecret(6))
	payload.Name = strings.TrimSpace(payload.Name)
	payload.RemoteRepo = strings.TrimSpace(payload.RemoteRepo)
	payload.RetentionKeep = normalizeBackupRetentionKeep(payload.RetentionKeep)
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
	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: "Backup destination saved.",
		Data:    sanitizeBackupDestination(payload),
	})
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
	payload.Domain = normalizeDomain(payload.Domain)
	if payload.Domain == "" {
		writeError(w, http.StatusBadRequest, "Domain is required for backup schedule.")
		return
	}
	payload.BackupPath = strings.TrimSpace(payload.BackupPath)
	if _, err := resolveSiteBackupTargetDir(payload.Domain, payload.BackupPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.ID = firstNonEmpty(payload.ID, generateSecret(6))
	payload.Incremental = false
	payload.RetentionKeep = normalizeBackupRetentionKeep(payload.RetentionKeep)
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
		Domain        string `json:"domain"`
		DestinationID string `json:"destination_id"`
		BackupPath    string `json:"backup_path"`
		RemoteRepo    string `json:"remote_repo"`
		Password      string `json:"password"`
		Incremental   bool   `json:"incremental"`
		RetentionKeep int    `json:"retention_keep"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid backup payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	if domain == "" {
		writeError(w, http.StatusBadRequest, "Domain is required.")
		return
	}
	payload.Incremental = false

	retentionKeep := payload.RetentionKeep
	s.mu.RLock()
	if destination, ok := s.backupDestinationByIDLocked(payload.DestinationID); ok {
		if retentionKeep <= 0 && destination.RetentionKeep > 0 {
			retentionKeep = destination.RetentionKeep
		}
	}
	s.mu.RUnlock()
	if retentionKeep <= 0 {
		retentionKeep = backupRetentionKeepFromEnv()
	}
	retentionKeep = normalizeBackupRetentionKeep(retentionKeep)

	snapshot, err := createRuntimeSiteBackup(domain, payload.BackupPath, false)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot.Domain = domain
	snapshot.DestinationID = strings.TrimSpace(payload.DestinationID)
	snapshot.RetentionKeep = retentionKeep
	snapshot.Incremental = false
	s.modules.BackupSnapshots = append([]BackupSnapshot{snapshot}, s.modules.BackupSnapshots...)
	prunedCount, retentionErr := s.enforceBackupRetentionLocked(domain, retentionKeep)

	s.appendActivityLocked("system", "backup_create", fmt.Sprintf("Backup created for %s.", domain), "")

	message := fmt.Sprintf("Backup completed for %s.", domain)
	if prunedCount > 0 {
		message = fmt.Sprintf("%s Retention policy pruned %d old snapshot(s).", message, prunedCount)
	}
	if retentionErr != nil {
		message = fmt.Sprintf("%s Retention warning: %s", message, retentionErr.Error())
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: message,
		Data: map[string]interface{}{
			"snapshot":     snapshot,
			"snapshot_id":  firstNonEmpty(snapshot.ShortID, snapshot.ID),
			"pruned_count": prunedCount,
		},
	})
}

func (s *service) handleBackupUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		errText := strings.ToLower(strings.TrimSpace(err.Error()))
		switch {
		case strings.Contains(errText, "multipart"), strings.Contains(errText, "boundary"):
			writeError(w, http.StatusBadRequest, "Backup upload could not be parsed. Use multipart/form-data upload.")
		case strings.Contains(errText, "request body too large"), strings.Contains(errText, "too large"):
			writeError(w, http.StatusRequestEntityTooLarge, "Backup upload is too large for current limits.")
		default:
			writeError(w, http.StatusBadRequest, "Backup upload could not be parsed.")
		}
		return
	}

	domain := normalizeDomain(r.FormValue("domain"))
	if domain == "" {
		writeError(w, http.StatusBadRequest, "Domain is required.")
		return
	}
	destinationID := strings.TrimSpace(r.FormValue("destination_id"))
	backupPath := strings.TrimSpace(r.FormValue("backup_path"))
	retentionKeep, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("retention_keep")))

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Backup archive is required.")
		return
	}
	defer file.Close()

	snapshot, err := createRuntimeSiteBackupFromArchive(domain, backupPath, header.Filename, file)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.RLock()
	if destination, ok := s.backupDestinationByIDLocked(destinationID); ok {
		if retentionKeep <= 0 && destination.RetentionKeep > 0 {
			retentionKeep = destination.RetentionKeep
		}
	}
	s.mu.RUnlock()
	if retentionKeep <= 0 {
		retentionKeep = backupRetentionKeepFromEnv()
	}
	retentionKeep = normalizeBackupRetentionKeep(retentionKeep)

	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot.Domain = domain
	snapshot.DestinationID = destinationID
	snapshot.RetentionKeep = retentionKeep
	s.modules.BackupSnapshots = append([]BackupSnapshot{snapshot}, s.modules.BackupSnapshots...)
	prunedCount, retentionErr := s.enforceBackupRetentionLocked(domain, retentionKeep)
	s.appendActivityLocked("system", "backup_upload", fmt.Sprintf("Backup uploaded for %s from %s.", domain, header.Filename), "")

	message := fmt.Sprintf("Backup uploaded for %s.", domain)
	if prunedCount > 0 {
		message = fmt.Sprintf("%s Retention policy pruned %d old snapshot(s).", message, prunedCount)
	}
	if retentionErr != nil {
		message = fmt.Sprintf("%s Retention warning: %s", message, retentionErr.Error())
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: message,
		Data: map[string]interface{}{
			"snapshot":     snapshot,
			"snapshot_id":  firstNonEmpty(snapshot.ShortID, snapshot.ID),
			"pruned_count": prunedCount,
		},
	})
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
	sort.Slice(items, func(i, j int) bool {
		return backupSnapshotTimestamp(items[i]) > backupSnapshotTimestamp(items[j])
	})
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
}

func (s *service) handleBackupRestore(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain     string `json:"domain"`
		SnapshotID string `json:"snapshot_id"`
		DryRun     bool   `json:"dry_run"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid restore payload.")
		return
	}
	s.mu.RLock()
	var snapshot BackupSnapshot
	found := false
	for _, item := range s.modules.BackupSnapshots {
		if item.ID == payload.SnapshotID || item.ShortID == payload.SnapshotID {
			snapshot = item
			found = true
			break
		}
	}
	s.mu.RUnlock()
	if !found {
		writeError(w, http.StatusNotFound, "Backup snapshot not found.")
		return
	}
	targetDomain := normalizeDomain(firstNonEmpty(payload.Domain, snapshot.Domain))
	if payload.DryRun {
		preview, err := previewRuntimeSiteRestore(snapshot, targetDomain)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.mu.Lock()
		s.appendActivityLocked("system", "backup_restore_dry_run", fmt.Sprintf("Restore dry-run completed for %s from snapshot %s.", targetDomain, payload.SnapshotID), "")
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, apiResponse{
			Status:  "success",
			Message: fmt.Sprintf("Restore dry-run completed for %s from snapshot %s.", targetDomain, payload.SnapshotID),
			Data:    preview,
		})
		return
	}
	if err := restoreRuntimeSiteBackup(snapshot, targetDomain); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	s.appendActivityLocked("system", "backup_restore", fmt.Sprintf("Restore completed for %s from snapshot %s.", targetDomain, payload.SnapshotID), "")
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("Restore completed for %s from snapshot %s.", targetDomain, payload.SnapshotID)})
}

func (s *service) handleDBBackupsList(w http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := listRuntimeDBBackups(s.modules.DBBackups)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.modules.DBBackups = records
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: records})
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
	record, err := createRuntimeDBBackup(engine, payload.DBName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record.Engine = engine
	s.modules.DBBackups = append([]DBBackupRecord{record}, s.modules.DBBackups...)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Database backup created.", Data: record})
}

func (s *service) handleDBBackupDownload(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.modules.DBBackups {
		if item.ID == id || item.Filename == id {
			content, err := os.ReadFile(resolveDBBackupPath(item))
			if err != nil {
				writeError(w, http.StatusNotFound, "Database backup file not found.")
				return
			}
			writeBlob(w, item.Filename, "application/gzip", content)
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
	s.mu.RLock()
	var record DBBackupRecord
	found := false
	for _, item := range s.modules.DBBackups {
		if item.ID == payload.BackupID || item.Filename == payload.BackupID {
			record = item
			found = true
			break
		}
	}
	s.mu.RUnlock()
	if !found {
		writeError(w, http.StatusNotFound, "Database backup not found.")
		return
	}
	if err := restoreRuntimeDBBackup(record); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("Database restore completed for %s.", record.DBName)})
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
	var target *DBBackupRecord
	for i := range items {
		if items[i].ID == payload.BackupID || items[i].Filename == payload.BackupID {
			target = &items[i]
			break
		}
	}
	if target == nil {
		writeError(w, http.StatusNotFound, "Database backup not found.")
		return
	}
	if err := deleteRuntimeDBBackup(*target); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filtered := items[:0]
	for _, item := range items {
		if item.ID == target.ID || item.Filename == target.Filename {
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.DBBackups = filtered
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
	domain := normalizeDomain(payload.Domain)
	detail := inspectCertificate(domain)
	s.mu.Lock()
	s.modules.SSLCertificates[domain] = detail
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: detail})
}

func (s *service) handleSSLHostnameIssue(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain  string `json:"domain"`
		Email   string `json:"email,omitempty"`
		Webroot string `json:"webroot,omitempty"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid hostname SSL payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	if domain == "" {
		writeError(w, http.StatusBadRequest, "Hostname domain is required.")
		return
	}

	webroot := strings.TrimSpace(payload.Webroot)
	if webroot == "" {
		webroot = "/usr/local/lsws/Example/html"
	}
	_ = os.MkdirAll(webroot, 0o755)

	if err := issueLetsEncryptCertificate([]string{domain}, webroot, false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Bind to OpenLiteSpeed
	certPath, keyPath := findCertificatePair(domain)
	if certPath != "" && keyPath != "" {
		certData, _ := os.ReadFile(certPath)
		keyData, _ := os.ReadFile(keyPath)
		_ = os.WriteFile("/usr/local/lsws/admin/conf/webadmin.crt", certData, 0o644)
		_ = os.WriteFile("/usr/local/lsws/admin/conf/webadmin.key", keyData, 0o600)
		_ = reloadOpenLiteSpeed()
	}

	s.mu.Lock()
	s.modules.SSLBindings.HostnameSSLDomain = domain
	s.modules.SSLBindings.UpdatedAt = time.Now().UTC().Unix()
	s.modules.SSLCertificates[domain] = inspectCertificate(domain)
	s.mu.Unlock()

	s.saveRuntimeState()

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("Hostname SSL issued for %s.", domain)})
}

func (s *service) handleSSLMailIssue(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain  string `json:"domain"`
		Email   string `json:"email,omitempty"`
		Webroot string `json:"webroot,omitempty"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid mail SSL payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	if domain == "" {
		writeError(w, http.StatusBadRequest, "Mail hostname is required.")
		return
	}

	webroot := strings.TrimSpace(payload.Webroot)
	if webroot == "" {
		webroot = "/usr/local/lsws/Example/html"
	}
	_ = os.MkdirAll(webroot, 0o755)

	if err := issueLetsEncryptCertificate([]string{domain}, webroot, false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Bind to Postfix and Dovecot
	certPath, keyPath := findCertificatePair(domain)
	if certPath != "" && keyPath != "" {
		// Postfix
		_ = exec.Command("postconf", "-e", fmt.Sprintf("smtpd_tls_cert_file=%s", certPath)).Run()
		_ = exec.Command("postconf", "-e", fmt.Sprintf("smtpd_tls_key_file=%s", keyPath)).Run()

		// Dovecot
		dovecotConf := "/etc/dovecot/conf.d/10-ssl.conf"
		if fileExists(dovecotConf) {
			content, err := os.ReadFile(dovecotConf)
			if err == nil {
				lines := strings.Split(string(content), "\n")
				for i, line := range lines {
					if strings.HasPrefix(strings.TrimSpace(line), "ssl_cert ") || strings.HasPrefix(strings.TrimSpace(line), "ssl_cert=") {
						lines[i] = fmt.Sprintf("ssl_cert = <%s", certPath)
					}
					if strings.HasPrefix(strings.TrimSpace(line), "ssl_key ") || strings.HasPrefix(strings.TrimSpace(line), "ssl_key=") {
						lines[i] = fmt.Sprintf("ssl_key = <%s", keyPath)
					}
				}
				_ = os.WriteFile(dovecotConf, []byte(strings.Join(lines, "\n")), 0o644)
			}
		}

		go func() {
			_ = exec.Command("systemctl", "restart", "postfix").Run()
			_ = exec.Command("systemctl", "restart", "dovecot").Run()
		}()
	}

	s.mu.Lock()
	s.modules.SSLBindings.MailSSLDomain = domain
	s.modules.SSLBindings.UpdatedAt = time.Now().UTC().Unix()
	s.modules.SSLCertificates[domain] = inspectCertificate(domain)
	s.mu.Unlock()

	s.saveRuntimeState()

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("Mail SSL issued for %s.", domain)})
}

func (s *service) handleSSLWildcardIssue(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain  string `json:"domain"`
		Email   string `json:"email,omitempty"`
		Webroot string `json:"webroot,omitempty"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid wildcard SSL payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	if domain == "" {
		writeError(w, http.StatusBadRequest, "Wildcard domain is required.")
		return
	}

	webroot := strings.TrimSpace(payload.Webroot)
	if webroot == "" {
		webroot = "/usr/local/lsws/Example/html"
	}
	_ = os.MkdirAll(webroot, 0o755)

	if err := issueLetsEncryptCertificate([]string{domain, "*." + domain}, webroot, true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	s.modules.SSLCertificates["*."+domain] = inspectCertificate(domain)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("Wildcard SSL issued for *.%s.", domain)})
}
