package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	olsHTTPDConfigPath      = "/usr/local/lsws/conf/httpd_config.conf"
	olsLSWSControlPath      = "/usr/local/lsws/bin/lswsctrl"
	olsManagedVhostPrefix   = "AuraPanel_"
	olsManagedVhostsBegin   = "# AURAPANEL VHOSTS BEGIN"
	olsManagedVhostsEnd     = "# AURAPANEL VHOSTS END"
	olsManagedListenerBegin = "    # AURAPANEL MAPS BEGIN"
	olsManagedListenerEnd   = "    # AURAPANEL MAPS END"
)

type olsManagedSite struct {
	Site    Website
	Config  WebsiteAdvancedConfig
	Aliases []string
}

func (s *service) syncOLSVhostsLocked() error {
	sites := append([]Website(nil), s.state.Websites...)
	advanced := make(map[string]WebsiteAdvancedConfig, len(s.state.AdvancedConfig))
	for key, value := range s.state.AdvancedConfig {
		advanced[key] = value
	}
	aliases := append([]DomainAlias(nil), s.state.Aliases...)
	return syncOLSRuntimeState(sites, advanced, aliases)
}

func syncOLSRuntimeState(sites []Website, advanced map[string]WebsiteAdvancedConfig, aliases []DomainAlias) error {
	if !fileExists(olsHTTPDConfigPath) || !fileExists(olsLSWSControlPath) {
		return fmt.Errorf("openlitespeed runtime is not installed on this host")
	}

	managedSites, err := buildOLSManagedSites(sites, advanced, aliases)
	if err != nil {
		return err
	}

	previousHTTPD, err := os.ReadFile(olsHTTPDConfigPath)
	if err != nil {
		return err
	}
	previousVhostFiles, err := backupOLSManagedVhostFiles()
	if err != nil {
		return err
	}

	desiredDirs := map[string]struct{}{}
	for _, item := range managedSites {
		if err := ensureOLSManagedFilesystem(item); err != nil {
			return err
		}
		vhostDir := olsManagedVhostDir(item.Site.Domain)
		desiredDirs[vhostDir] = struct{}{}
		if err := os.WriteFile(filepath.Join(vhostDir, "vhconf.conf"), []byte(renderOLSVhostConfig(item)), 0o644); err != nil {
			return err
		}
	}

	renderedHTTPD, err := renderOLSHTTPDConfig(string(previousHTTPD), managedSites)
	if err != nil {
		return err
	}
	if err := os.WriteFile(olsHTTPDConfigPath, []byte(renderedHTTPD), 0o640); err != nil {
		return err
	}

	if err := reloadOpenLiteSpeed(); err != nil {
		_ = os.WriteFile(olsHTTPDConfigPath, previousHTTPD, 0o640)
		_ = restoreOLSManagedVhostFiles(previousVhostFiles)
		_ = reloadOpenLiteSpeed()
		return err
	}

	return cleanupStaleOLSVhostDirs(desiredDirs)
}

func buildOLSManagedSites(sites []Website, advanced map[string]WebsiteAdvancedConfig, aliases []DomainAlias) ([]olsManagedSite, error) {
	out := make([]olsManagedSite, 0, len(sites))
	for _, site := range sites {
		domain := normalizeDomain(site.Domain)
		if domain == "" {
			continue
		}
		cfg := advanced[domain]
		cfg.VhostConfig = sanitizeOLSOverride(domain, cfg.VhostConfig)
		site.Domain = domain
		if _, err := resolveOLSPHPBinary(site.PHPVersion); err != nil {
			return nil, fmt.Errorf("%s icin PHP runtime hazir degil: %w", domain, err)
		}
		out = append(out, olsManagedSite{
			Site:    site,
			Config:  cfg,
			Aliases: olsAliasNames(domain, aliases),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Site.Domain < out[j].Site.Domain
	})
	return out, nil
}

func sanitizeOLSOverride(domain, content string) string {
	content = strings.TrimSpace(content)
	defaultLine := fmt.Sprintf("vhDomain %s", normalizeDomain(domain))
	if content == "" || strings.EqualFold(content, defaultLine) {
		return ""
	}
	return content
}

func olsAliasNames(domain string, aliases []DomainAlias) []string {
	items := []string{domain, "www." + domain}
	for _, alias := range aliases {
		if normalizeDomain(alias.Domain) != domain {
			continue
		}
		items = append(items, normalizeDomain(alias.Alias))
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = normalizeDomain(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func ensureOLSManagedFilesystem(item olsManagedSite) error {
	docroot := domainDocroot(item.Site.Domain)
	if err := os.MkdirAll(docroot, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join("/usr/local/lsws/logs"), 0o750); err != nil {
		return err
	}
	if err := os.MkdirAll(olsManagedVhostDir(item.Site.Domain), 0o755); err != nil {
		return err
	}
	if err := writeOLSHTAccess(item.Site.Domain, item.Config.RewriteRules); err != nil {
		return err
	}
	indexPath := filepath.Join(docroot, "index.html")
	if !fileExists(indexPath) {
		placeholder := fmt.Sprintf(`<!DOCTYPE html>
<html lang="tr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s - Başarıyla Kuruldu</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background-color: #0f172a; color: #f8fafc; margin: 0; padding: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; min-height: 100vh; text-align: center; }
        .container { background-color: #1e293b; padding: 3rem; border-radius: 1.5rem; box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.5), 0 8px 10px -6px rgba(0, 0, 0, 0.5); border: 1px solid #334155; max-width: 600px; width: 90%%; }
        h1 { color: #f97316; font-size: 2.25rem; margin-top: 0; margin-bottom: 1rem; }
        p { color: #94a3b8; font-size: 1.125rem; line-height: 1.75; margin-bottom: 2rem; }
        .domain { font-weight: bold; color: #38bdf8; }
        .footer { margin-top: 3rem; color: #64748b; font-size: 0.875rem; }
        .footer a { color: #f97316; text-decoration: none; }
        .footer a:hover { text-decoration: underline; }
        .badge { display: inline-block; padding: 0.25rem 0.75rem; border-radius: 9999px; background-color: rgba(249, 115, 22, 0.1); border: 1px solid rgba(249, 115, 22, 0.2); color: #f97316; font-size: 0.875rem; font-weight: 600; margin-bottom: 1.5rem; }
    </style>
</head>
<body>
    <div class="container">
        <div class="badge">AuraPanel Web Server</div>
        <h1>Web Siteniz Hazır!</h1>
        <p><span class="domain">%s</span> alanı için hosting hesabınız ve web sunucunuz başarıyla oluşturuldu ve şu an aktif olarak çalışıyor.</p>
        <p>AuraPanel üzerinden dosyalarınızı yükleyebilir, veritabanı oluşturabilir veya tek tıkla WordPress kurabilirsiniz.</p>
    </div>
    <div class="footer">
        Powered by <a href="https://aurapanel.com" target="_blank">AuraPanel</a> & OpenLiteSpeed
    </div>
</body>
</html>
`, item.Site.Domain, item.Site.Domain)
		if err := os.WriteFile(indexPath, []byte(placeholder), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeOLSHTAccess(domain, rules string) error {
	docroot := domainDocroot(domain)
	if err := os.MkdirAll(docroot, 0o755); err != nil {
		return err
	}
	rules = strings.TrimSpace(rules)
	if rules == "" {
		rules = "RewriteEngine On"
	}
	return os.WriteFile(filepath.Join(docroot, ".htaccess"), []byte(rules+"\n"), 0o644)
}

func olsManagedVhostName(domain string) string {
	domain = normalizeDomain(domain)
	var b strings.Builder
	b.WriteString(olsManagedVhostPrefix)
	for _, r := range domain {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

func olsManagedVhostDir(domain string) string {
	return filepath.Join("/usr/local/lsws/conf/vhosts", olsManagedVhostName(domain))
}

func olsManagedVhostConfigRelative(domain string) string {
	return filepath.ToSlash(filepath.Join("conf/vhosts", olsManagedVhostName(domain), "vhconf.conf"))
}

func olsManagedSocket(domain string) string {
	return "uds://tmp/lshttpd/" + olsManagedVhostName(domain) + ".sock"
}

func resolveOLSPHPBinary(version string) (string, error) {
	token := phpVersionPackageToken(version)
	candidates := []string{
		fmt.Sprintf("/usr/local/lsws/lsphp%s/bin/lsphp", token),
		fmt.Sprintf("/usr/local/lsws/lsphp%s/bin/lsphp%s", token, token),
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	for _, item := range discoverPHPVersions() {
		token = phpVersionPackageToken(item.Version)
		candidate := fmt.Sprintf("/usr/local/lsws/lsphp%s/bin/lsphp", token)
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("uygun lsphp binary bulunamadi")
}

func renderOLSVhostConfig(item olsManagedSite) string {
	phpBinary, _ := resolveOLSPHPBinary(item.Site.PHPVersion)
	socketName := olsManagedVhostName(item.Site.Domain) + "_lsphp"
	accessLog := filepath.ToSlash(filepath.Join("/usr/local/lsws/logs", item.Site.Domain+".access.log"))
	errorLog := filepath.ToSlash(filepath.Join("/usr/local/lsws/logs", item.Site.Domain+".error.log"))
	phpErrorLog := filepath.ToSlash(filepath.Join("/usr/local/lsws/logs", item.Site.Domain+".php.error.log"))
	docroot := filepath.ToSlash(domainDocroot(item.Site.Domain))

	var builder strings.Builder
	builder.WriteString("# AuraPanel managed OpenLiteSpeed vhost config\n")
	builder.WriteString("docRoot                   " + docroot + "\n")
	builder.WriteString("enableGzip                1\n\n")
	builder.WriteString("index  {\n")
	builder.WriteString("  useServer               0\n")
	builder.WriteString("  indexFiles              index.php, index.html\n")
	builder.WriteString("  autoIndex               0\n")
	builder.WriteString("}\n\n")
	builder.WriteString("errorlog " + errorLog + "{\n")
	builder.WriteString("  useServer               0\n")
	builder.WriteString("  logLevel                NOTICE\n")
	builder.WriteString("  rollingSize             10M\n")
	builder.WriteString("}\n\n")
	builder.WriteString("accessLog " + accessLog + "{\n")
	builder.WriteString("  useServer               0\n")
	builder.WriteString("  logReferer              1\n")
	builder.WriteString("  logUserAgent            1\n")
	builder.WriteString("  keepDays                30\n")
	builder.WriteString("  rollingSize             10M\n")
	builder.WriteString("}\n\n")
	builder.WriteString("extProcessor " + socketName + "{\n")
	builder.WriteString("  type                    lsapi\n")
	builder.WriteString("  address                 " + olsManagedSocket(item.Site.Domain) + "\n")
	builder.WriteString("  maxConns                10\n")
	builder.WriteString("  env                     PHP_LSAPI_CHILDREN=10\n")
	builder.WriteString("  env                     LSAPI_AVOID_FORK=200M\n")
	builder.WriteString("  initTimeout             60\n")
	builder.WriteString("  retryTimeout            0\n")
	builder.WriteString("  persistConn             1\n")
	builder.WriteString("  pcKeepAliveTimeout      1\n")
	builder.WriteString("  respBuffer              0\n")
	builder.WriteString("  autoStart               1\n")
	builder.WriteString("  path                    " + filepath.ToSlash(phpBinary) + "\n")
	builder.WriteString("  backlog                 100\n")
	builder.WriteString("  instances               1\n")
	builder.WriteString("  extMaxIdleTime          300\n")
	builder.WriteString("}\n\n")
	builder.WriteString("scriptHandler {\n")
	builder.WriteString("  add lsapi:" + socketName + " php\n")
	builder.WriteString("  add lsapi:" + socketName + " phtml\n")
	builder.WriteString("}\n\n")
	builder.WriteString("phpIniOverride  {\n")
	builder.WriteString("  php_admin_flag log_errors On\n")
	builder.WriteString("  php_admin_value error_log \"" + phpErrorLog + "\"\n")
	if item.Config.OpenBasedir {
		builder.WriteString("  php_admin_value open_basedir \"" + olsOpenBasedirValue(item.Site.Domain) + "\"\n")
	}
	builder.WriteString("}\n\n")
	builder.WriteString("rewrite  {\n")
	builder.WriteString("  enable                  1\n")
	builder.WriteString("  autoLoadHtaccess        1\n")
	if !strings.EqualFold(item.Site.Status, "active") {
		builder.WriteString("  RewriteRule ^(.*)$ - [F,L]\n")
	}
	builder.WriteString("}\n\n")
	if certPath, keyPath := findCertificatePair(item.Site.Domain); certPath != "" && keyPath != "" {
		builder.WriteString("vhssl  {\n")
		builder.WriteString("  keyFile                 " + filepath.ToSlash(keyPath) + "\n")
		builder.WriteString("  certFile                " + filepath.ToSlash(certPath) + "\n")
		builder.WriteString("  certChain               1\n")
		builder.WriteString("}\n\n")
	}
	if extra := strings.TrimSpace(item.Config.VhostConfig); extra != "" {
		builder.WriteString("# AuraPanel custom vhost override\n")
		builder.WriteString(extra)
		builder.WriteString("\n")
	}
	return builder.String()
}

func olsOpenBasedirValue(domain string) string {
	domain = normalizeDomain(domain)
	return fmt.Sprintf("/home/%s/:/home/%s/public_html/:/tmp:/var/tmp:/usr/local/lib/php/:/dev/urandom", domain, domain)
}

func renderOLSHTTPDConfig(current string, sites []olsManagedSite) (string, error) {
	managedVhosts := renderOLSManagedVhostBlocks(sites)
	withVhosts := replaceOrInsertManagedBlock(current, olsManagedVhostsBegin, olsManagedVhostsEnd, managedVhosts, "module cache {")
	withDefault, err := replaceOLSListenerMaps(withVhosts, "Default", renderOLSManagedListenerMapBlock(sites))
	if err != nil {
		return "", err
	}
	withSSL, err := replaceOLSListenerMaps(withDefault, "AuraPanelSSL", renderOLSManagedListenerMapBlock(sites))
	if err != nil {
		return "", err
	}
	return withSSL, nil
}

func renderOLSManagedVhostBlocks(sites []olsManagedSite) string {
	lines := []string{olsManagedVhostsBegin}
	for _, item := range sites {
		vhostName := olsManagedVhostName(item.Site.Domain)
		vhostAliases := make([]string, 0, len(item.Aliases))
		for _, alias := range item.Aliases {
			if alias != item.Site.Domain {
				vhostAliases = append(vhostAliases, alias)
			}
		}
		lines = append(lines,
			fmt.Sprintf("virtualHost %s{", vhostName),
			fmt.Sprintf("    vhRoot                   /home/%s/", item.Site.Domain),
			"    allowSymbolLink          1",
			"    enableScript             1",
			"    restrained               1",
			"    setUIDMode               0",
			"    chrootMode               0",
			fmt.Sprintf("    docRoot                  %s", filepath.ToSlash(domainDocroot(item.Site.Domain))),
			fmt.Sprintf("    vhDomain                 %s", item.Site.Domain),
			fmt.Sprintf("    adminEmails              %s", firstNonEmpty(strings.TrimSpace(item.Site.Email), fmt.Sprintf("webmaster@%s", item.Site.Domain))),
			fmt.Sprintf("    configFile               %s", olsManagedVhostConfigRelative(item.Site.Domain)),
		)
		if len(vhostAliases) > 0 {
			lines = append(lines, fmt.Sprintf("    vhAliases                %s", strings.Join(vhostAliases, ", ")))
		}
		lines = append(lines, "}", "")
	}
	lines = append(lines, olsManagedVhostsEnd)
	return strings.Join(lines, "\n")
}

func renderOLSManagedListenerMapBlock(sites []olsManagedSite) string {
	lines := []string{olsManagedListenerBegin}
	for _, item := range sites {
		lines = append(lines, fmt.Sprintf("    map                      %s %s", olsManagedVhostName(item.Site.Domain), strings.Join(item.Aliases, ", ")))
	}
	lines = append(lines, olsManagedListenerEnd)
	return strings.Join(lines, "\n")
}

func replaceOrInsertManagedBlock(current, beginMarker, endMarker, replacement, anchor string) string {
	beginIndex := strings.Index(current, beginMarker)
	endIndex := strings.Index(current, endMarker)
	if beginIndex >= 0 && endIndex > beginIndex {
		endIndex += len(endMarker)
		return current[:beginIndex] + replacement + current[endIndex:]
	}
	anchorIndex := strings.Index(current, anchor)
	if anchorIndex >= 0 {
		return current[:anchorIndex] + replacement + "\n\n" + current[anchorIndex:]
	}
	if strings.HasSuffix(current, "\n") {
		return current + "\n" + replacement + "\n"
	}
	return current + "\n\n" + replacement + "\n"
}

func replaceOLSListenerMaps(current, listenerName, replacement string) (string, error) {
	token := "listener " + listenerName + "{"
	start := strings.Index(current, token)
	if start < 0 {
		return current, nil
	}
	openBrace := strings.Index(current[start:], "{")
	if openBrace < 0 {
		return "", fmt.Errorf("%s listener block is invalid", listenerName)
	}
	openBrace += start
	closeBrace, err := findMatchingBrace(current, openBrace)
	if err != nil {
		return "", err
	}
	section := current[start : closeBrace+1]
	section = replaceOrInsertManagedBlock(section, olsManagedListenerBegin, olsManagedListenerEnd, replacement, "\n}")
	return current[:start] + section + current[closeBrace+1:], nil
}

func findMatchingBrace(content string, openIndex int) (int, error) {
	depth := 0
	for idx := openIndex; idx < len(content); idx++ {
		switch content[idx] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return idx, nil
			}
		}
	}
	return -1, fmt.Errorf("configuration brace matching failed")
}

func reloadOpenLiteSpeed() error {
	_, err := commandOutputTrimmed(olsLSWSControlPath, "reload")
	if err == nil {
		return nil
	}
	_, restartErr := commandOutputTrimmed(olsLSWSControlPath, "restart")
	if restartErr == nil {
		return nil
	}
	return fmt.Errorf("openlitespeed reload failed: %v", err)
}

func backupOLSManagedVhostFiles() (map[string][]byte, error) {
	pattern := filepath.Join("/usr/local/lsws/conf/vhosts", olsManagedVhostPrefix+"*", "vhconf.conf")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	backups := make(map[string][]byte, len(matches))
	for _, match := range matches {
		raw, err := os.ReadFile(match)
		if err != nil {
			return nil, err
		}
		backups[match] = raw
	}
	return backups, nil
}

func restoreOLSManagedVhostFiles(backups map[string][]byte) error {
	pattern := filepath.Join("/usr/local/lsws/conf/vhosts", olsManagedVhostPrefix+"*", "vhconf.conf")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, match := range matches {
		if _, ok := backups[match]; ok {
			continue
		}
		_ = os.Remove(match)
		_ = os.Remove(filepath.Dir(match))
	}
	for path, content := range backups {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func cleanupStaleOLSVhostDirs(desiredDirs map[string]struct{}) error {
	pattern := filepath.Join("/usr/local/lsws/conf/vhosts", olsManagedVhostPrefix+"*")
	dirs, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		if _, ok := desiredDirs[dir]; ok {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	return nil
}

func runtimeOLSTuningConfig() (OLSTuningConfig, error) {
	if !fileExists(olsHTTPDConfigPath) {
		return OLSTuningConfig{}, fmt.Errorf("openlitespeed config bulunamadi")
	}
	raw, err := os.ReadFile(olsHTTPDConfigPath)
	if err != nil {
		return OLSTuningConfig{}, err
	}
	content := string(raw)
	tuningBlock, err := extractOLSConfigBlock(content, "tuning{")
	if err != nil {
		return OLSTuningConfig{}, err
	}
	cacheBlock, _ := extractOLSConfigBlock(content, "module cache {")
	cfg := OLSTuningConfig{
		MaxConnections:       10000,
		MaxSSLConnections:    10000,
		ConnTimeoutSecs:      300,
		KeepAliveTimeoutSecs: 5,
		MaxKeepAliveRequests: 10000,
		GzipCompression:      true,
		StaticCacheEnabled:   false,
		StaticCacheMaxAgeSec: 3600,
	}
	parseOLSDirectiveInt(tuningBlock, "maxConnections", &cfg.MaxConnections)
	parseOLSDirectiveInt(tuningBlock, "maxSSLConnections", &cfg.MaxSSLConnections)
	parseOLSDirectiveInt(tuningBlock, "connTimeout", &cfg.ConnTimeoutSecs)
	parseOLSDirectiveInt(tuningBlock, "keepAliveTimeout", &cfg.KeepAliveTimeoutSecs)
	parseOLSDirectiveInt(tuningBlock, "maxKeepAliveReq", &cfg.MaxKeepAliveRequests)
	parseOLSDirectiveBool(tuningBlock, "enableGzipCompress", &cfg.GzipCompression)
	parseOLSDirectiveBool(cacheBlock, "enableCache", &cfg.StaticCacheEnabled)
	parseOLSDirectiveInt(cacheBlock, "expireInSeconds", &cfg.StaticCacheMaxAgeSec)
	return cfg, nil
}

func applyOLSTuningConfig(cfg OLSTuningConfig) error {
	if !fileExists(olsHTTPDConfigPath) {
		return fmt.Errorf("openlitespeed config bulunamadi")
	}
	previous, err := os.ReadFile(olsHTTPDConfigPath)
	if err != nil {
		return err
	}
	content, err := replaceOLSBlockDirectives(string(previous), "tuning{", map[string]string{
		"maxConnections":    strconv.Itoa(maxInt(cfg.MaxConnections, 1)),
		"maxSSLConnections": strconv.Itoa(maxInt(cfg.MaxSSLConnections, 1)),
		"connTimeout":       strconv.Itoa(maxInt(cfg.ConnTimeoutSecs, 1)),
		"keepAliveTimeout":  strconv.Itoa(maxInt(cfg.KeepAliveTimeoutSecs, 1)),
		"maxKeepAliveReq":   strconv.Itoa(maxInt(cfg.MaxKeepAliveRequests, 1)),
		"enableGzipCompress": map[bool]string{
			true:  "1",
			false: "0",
		}[cfg.GzipCompression],
	})
	if err != nil {
		return err
	}
	content, err = replaceOLSBlockDirectives(content, "module cache {", map[string]string{
		"enableCache": map[bool]string{
			true:  "1",
			false: "0",
		}[cfg.StaticCacheEnabled],
		"expireInSeconds": strconv.Itoa(maxInt(cfg.StaticCacheMaxAgeSec, 0)),
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(olsHTTPDConfigPath, []byte(content), 0o640); err != nil {
		return err
	}
	if err := reloadOpenLiteSpeed(); err != nil {
		_ = os.WriteFile(olsHTTPDConfigPath, previous, 0o640)
		_ = reloadOpenLiteSpeed()
		return err
	}
	return nil
}

func extractOLSConfigBlock(content, token string) (string, error) {
	start := strings.Index(content, token)
	if start < 0 {
		return "", fmt.Errorf("%s block bulunamadi", token)
	}
	openBrace := strings.Index(content[start:], "{")
	if openBrace < 0 {
		return "", fmt.Errorf("%s block gecersiz", token)
	}
	openBrace += start
	closeBrace, err := findMatchingBrace(content, openBrace)
	if err != nil {
		return "", err
	}
	return content[start : closeBrace+1], nil
}

func parseOLSDirectiveInt(block, key string, target *int) {
	for _, line := range strings.Split(block, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != key {
			continue
		}
		if value, err := strconv.Atoi(fields[1]); err == nil {
			*target = value
		}
		return
	}
}

func parseOLSDirectiveBool(block, key string, target *bool) {
	for _, line := range strings.Split(block, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != key {
			continue
		}
		*target = fields[1] == "1"
		return
	}
}

func replaceOLSBlockDirectives(content, token string, directives map[string]string) (string, error) {
	start := strings.Index(content, token)
	if start < 0 {
		return "", fmt.Errorf("%s block bulunamadi", token)
	}
	openBrace := strings.Index(content[start:], "{")
	if openBrace < 0 {
		return "", fmt.Errorf("%s block gecersiz", token)
	}
	openBrace += start
	closeBrace, err := findMatchingBrace(content, openBrace)
	if err != nil {
		return "", err
	}
	block := content[start : closeBrace+1]
	lines := strings.Split(block, "\n")
	seen := map[string]bool{}
	for idx, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		if value, ok := directives[fields[0]]; ok {
			lines[idx] = fmt.Sprintf("    %-25s %s", fields[0], value)
			seen[fields[0]] = true
		}
	}
	insertAt := len(lines) - 1
	extra := make([]string, 0, len(directives))
	for key, value := range directives {
		if seen[key] {
			continue
		}
		extra = append(extra, fmt.Sprintf("    %-25s %s", key, value))
	}
	sort.Strings(extra)
	if len(extra) > 0 {
		lines = append(lines[:insertAt], append(extra, lines[insertAt:]...)...)
	}
	updated := strings.Join(lines, "\n")
	return content[:start] + updated + content[closeBrace+1:], nil
}
