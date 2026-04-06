package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	olsHTTPDConfigPath      = "/usr/local/lsws/conf/httpd_config.conf"
	olsLSWSControlPath      = "/usr/local/lsws/bin/lswsctrl"
	olsManagedVhostPrefix   = "AuraPanel_"
	olsManagedVhostsBegin   = "# AURAPANEL VHOSTS BEGIN"
	olsManagedVhostsEnd     = "# AURAPANEL VHOSTS END"
	olsManagedListenerBegin = "    # AURAPANEL MAPS BEGIN"
	olsManagedListenerEnd   = "    # AURAPANEL MAPS END"
	olsReloadWaitTimeout    = 10 * time.Second
	olsReloadPollInterval   = 250 * time.Millisecond
)

var olsSleep = time.Sleep

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
		vhostConfPath := filepath.Join(vhostDir, "vhconf.conf")
		if err := os.WriteFile(vhostConfPath, []byte(renderOLSVhostConfig(item)), 0o600); err != nil {
			return err
		}
		if err := ensureOLSManagedVhostOwnership(item.Site.Domain); err != nil {
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

	// Always do a gracefull reload to apply new vhost configs immediately
	if err := reloadOpenLiteSpeed(); err != nil {
		// Rollback if reload fails due to syntax error
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
	siteRoot := filepath.Dir(docroot)
	if err := ensureOLSManagedOwnerAccount(item.Site); err != nil {
		return err
	}
	if err := ensureOLSPathMode("/home", 0o711); err != nil {
		return err
	}
	if err := os.MkdirAll(siteRoot, 0o711); err != nil {
		return err
	}
	if err := os.MkdirAll(docroot, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(olsManagedVhostDir(item.Site.Domain), 0o755); err != nil {
		return err
	}
	if err := seedOLSManagedDocrootFiles(item.Site.Domain, item.Config.RewriteRules); err != nil {
		return err
	}
	return ensureOLSManagedOwnership(item.Site)
}

func seedOLSManagedDocrootFiles(domain, rules string) error {
	docroot := domainDocroot(domain)
	if err := os.MkdirAll(docroot, 0o755); err != nil {
		return err
	}
	return seedOLSManagedDocrootContent(docroot, domain, rules)
}

func seedOLSManagedDocrootContent(docroot, domain, rules string) error {
	// Runtime sync must never mutate live application files.
	// Seed defaults only for truly empty docroots (freshly provisioned websites).
	if !olsDocrootEffectivelyEmpty(docroot) {
		return nil
	}
	if err := writeOLSHTAccessFile(filepath.Join(docroot, ".htaccess"), rules, false); err != nil {
		return err
	}
	indexPath := filepath.Join(docroot, "index.html")
	if fileExists(indexPath) || fileExists(filepath.Join(docroot, "index.php")) {
		return nil
	}
	return os.WriteFile(indexPath, []byte(defaultOLSIndexPlaceholder(domain)), 0o644)
}

func olsDocrootEffectivelyEmpty(docroot string) bool {
	entries, err := os.ReadDir(docroot)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if strings.TrimSpace(entry.Name()) == "" {
			continue
		}
		return false
	}
	return true
}

func defaultOLSIndexPlaceholder(domain string) string {
	domain = normalizeDomain(domain)
	if domain == "" {
		domain = "site"
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="tr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s - Basariyla Kuruldu</title>
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
        <h1>Web Siteniz Hazir!</h1>
        <p><span class="domain">%s</span> alani icin hosting hesabin ve web sunucun basariyla olusturuldu ve su an aktif olarak calisiyor.</p>
        <p>AuraPanel uzerinden dosyalarini yukleyebilir, veritabani olusturabilir veya tek tikla WordPress kurabilirsin.</p>
    </div>
    <div class="footer">
        Powered by <a href="https://aurapanel.com" target="_blank">AuraPanel</a> & OpenLiteSpeed
    </div>
</body>
</html>
`, domain, domain)
}

func writeOLSHTAccess(domain, rules string) error {
	docroot := domainDocroot(domain)
	if err := os.MkdirAll(docroot, 0o755); err != nil {
		return err
	}
	return writeOLSHTAccessFile(filepath.Join(docroot, ".htaccess"), rules, shouldOverwriteOLSHTAccess(rules))
}

func applyWebsiteRewriteRules(domain, rules string) error {
	docroot := domainDocroot(domain)
	if err := os.MkdirAll(docroot, 0o755); err != nil {
		return err
	}
	return writeOLSHTAccessFile(filepath.Join(docroot, ".htaccess"), rules, true)
}

func shouldOverwriteOLSHTAccess(rules string) bool {
	rules = strings.TrimSpace(rules)
	if rules == "" {
		return false
	}
	return !strings.EqualFold(rules, "RewriteEngine On")
}

func writeOLSHTAccessFile(path, rules string, overwrite bool) error {
	if !overwrite && fileExists(path) {
		return nil
	}
	rules = strings.TrimSpace(rules)
	if rules == "" {
		rules = "RewriteEngine On"
	}
	return os.WriteFile(path, []byte(rules+"\n"), 0o644)
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
		case r == '.':
			b.WriteByte('_')
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

func olsSiteLogDir(domain string) string {
	return filepath.Join("/home", normalizeDomain(domain), "logs")
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
	logDir := olsSiteLogDir(item.Site.Domain)
	accessLog := filepath.ToSlash(filepath.Join(logDir, item.Site.Domain+".access_log"))
	errorLog := filepath.ToSlash(filepath.Join(logDir, item.Site.Domain+".error_log"))
	phpErrorLog := filepath.ToSlash(filepath.Join(logDir, item.Site.Domain+".php.error.log"))
	docroot := filepath.ToSlash(domainDocroot(item.Site.Domain))
	siteOwner := siteSystemOwner(item.Site)

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

	// Add explicit acme-challenge routing context to ensure Let's Encrypt validation works even with strict rewrites
	builder.WriteString("context /.well-known/acme-challenge/ {\n")
	builder.WriteString("  location                " + docroot + "/.well-known/acme-challenge/\n")
	builder.WriteString("  allowBrowse             1\n")
	builder.WriteString("  rewrite  {\n")
	builder.WriteString("    enable                0\n")
	builder.WriteString("  }\n")
	builder.WriteString("  addDefaultCharset       off\n")
	builder.WriteString("  phpIniOverride  {\n")
	builder.WriteString("  }\n")
	builder.WriteString("  forceStrict             0\n")
	builder.WriteString("}\n\n")
	builder.WriteString("context /webmail/ {\n")
	builder.WriteString("  location                /usr/local/lsws/Example/html/webmail/\n")
	builder.WriteString("  allowBrowse             1\n")
	builder.WriteString("  rewrite  {\n")
	builder.WriteString("    enable                0\n")
	builder.WriteString("  }\n")
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
	builder.WriteString("  extUser                 " + siteOwner + "\n")
	builder.WriteString("  extGroup                " + siteOwner + "\n")
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
	certPath, keyPath := findCertificatePair(item.Site.Domain)
	builder.WriteString("rewrite  {\n")
	builder.WriteString("  enable                  1\n")
	builder.WriteString("  autoLoadHtaccess        1\n")
	if certPath != "" && keyPath != "" {
		builder.WriteString("  RewriteCond %{HTTPS} !=on\n")
		builder.WriteString("  RewriteRule ^ https://%{HTTP_HOST}%{REQUEST_URI} [R=301,L]\n")
	}
	if !strings.EqualFold(item.Site.Status, "active") {
		builder.WriteString("  RewriteRule ^(.*)$ - [F,L]\n")
	}
	builder.WriteString("}\n\n")
	if certPath != "" && keyPath != "" {
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
	return fmt.Sprintf("/home/%s/:/home/%s/public_html/:/tmp:/var/tmp:/usr/local/lib/php/:/dev/urandom:/usr/local/lsws/Example/html/webmail/:/usr/local/lsws/Example/html/webmail/temp/:/usr/local/lsws/Example/html/webmail/logs/", domain, domain)
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
	lines = append(lines, "    map                      Example *")
	lines = append(lines, olsManagedListenerEnd)
	return strings.Join(lines, "\n")
}

func siteSystemOwner(site Website) string {
	return firstNonEmpty(sanitizeName(site.Owner), sanitizeName(site.User), "admin")
}

func systemNoLoginShell() string {
	for _, candidate := range []string{"/usr/sbin/nologin", "/sbin/nologin", "/bin/false"} {
		if fileExists(candidate) {
			return candidate
		}
	}
	return "/bin/false"
}

func ensureOLSManagedOwnerAccount(site Website) error {
	owner := siteSystemOwner(site)
	if owner == "" || owner == "root" {
		return nil
	}
	if !systemGroupExists(owner) {
		if output, err := exec.Command("groupadd", "--system", owner).CombinedOutput(); err != nil && !systemGroupExists(owner) {
			return fmt.Errorf("website owner group %s could not be created: %s", owner, strings.TrimSpace(string(output)))
		}
	}
	if systemUserExists(owner) {
		return nil
	}
	args := []string{"--system", "-M", "-s", systemNoLoginShell(), "-g", owner, owner}
	if output, err := exec.Command("useradd", args...).CombinedOutput(); err != nil && !systemUserExists(owner) {
		return fmt.Errorf("website owner user %s could not be created: %s", owner, strings.TrimSpace(string(output)))
	}
	return nil
}

func ensureOLSManagedOwnership(site Website) error {
	owner := siteSystemOwner(site)
	if owner == "" || owner == "root" {
		return nil
	}
	runtimeGroup := olsSharedRuntimeGroup()
	if runtimeGroup == "" {
		return fmt.Errorf("failed to determine OpenLiteSpeed runtime group (nogroup/nobody)")
	}

	docroot := domainDocroot(site.Domain)
	siteRoot := filepath.Dir(docroot)
	logDir := olsSiteLogDir(site.Domain)
	if !fileExists(docroot) {
		return nil
	}
	if err := ensureOLSPathMode(siteRoot, 0o711); err != nil {
		return err
	}
	if err := runOLSChown(siteRoot, owner, owner, false); err != nil {
		return err
	}
	if err := runOLSChown(docroot, owner, owner, true); err != nil {
		return err
	}
	if err := runOLSChown(docroot, owner, runtimeGroup, false); err != nil {
		return err
	}
	if err := ensureOLSPathMode(docroot, 0o750); err != nil {
		return err
	}
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return err
	}
	if err := runOLSChown(logDir, "root", runtimeGroup, true); err != nil {
		return err
	}
	if err := ensureOLSPathMode(logDir, 0o750); err != nil {
		return err
	}

	htaccessPath := filepath.Join(docroot, ".htaccess")
	if fileExists(htaccessPath) {
		if err := runOLSChown(htaccessPath, owner, owner, false); err != nil {
			return err
		}
	}
	indexPath := filepath.Join(docroot, "index.html")
	if fileExists(indexPath) {
		if err := runOLSChown(indexPath, owner, owner, false); err != nil {
			return err
		}
	}
	return nil
}

func ensureOLSManagedVhostOwnership(domain string) error {
	vhostDir := olsManagedVhostDir(domain)
	return ensureOLSManagedVhostDirOwnership(vhostDir)
}

func ensureOLSManagedVhostDirOwnership(vhostDir string) error {
	if !fileExists(vhostDir) {
		return nil
	}
	ownerUser := "lsadm"
	if !systemUserExists(ownerUser) {
		ownerUser = "root"
	}
	group := "lsadm"
	if !systemGroupExists(group) {
		group = "root"
	}
	if err := runOLSChown(vhostDir, ownerUser, group, true); err != nil {
		return err
	}
	if err := ensureOLSPathMode(vhostDir, 0o750); err != nil {
		return err
	}
	vhostConf := filepath.Join(vhostDir, "vhconf.conf")
	if fileExists(vhostConf) {
		if err := runOLSChown(vhostConf, ownerUser, group, false); err != nil {
			return err
		}
		if err := os.Chmod(vhostConf, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func ensureOLSPathMode(path string, mode os.FileMode) error {
	if !fileExists(path) {
		return nil
	}
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("chmod failed for %s: %w", path, err)
	}
	return nil
}

func runOLSChown(path, owner, group string, recursive bool) error {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(owner) == "" {
		return nil
	}
	spec := owner
	if strings.TrimSpace(group) != "" {
		spec = owner + ":" + group
	}
	args := []string{}
	if recursive {
		args = append(args, "-R")
	}
	args = append(args, spec, path)
	output, err := exec.Command("chown", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("chown %s failed for %s: %s", spec, path, strings.TrimSpace(string(output)))
	}
	return nil
}

func olsSharedRuntimeGroup() string {
	for _, group := range []string{"nogroup", "nobody"} {
		if systemGroupExists(group) {
			return group
		}
	}
	return ""
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

func currentOpenLiteSpeedPID() string {
	for _, path := range []string{"/tmp/lshttpd/lshttpd.pid", "/run/openlitespeed.pid", "/var/run/openlitespeed.pid"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if pid := strings.TrimSpace(string(raw)); pid != "" {
			return pid
		}
	}
	return ""
}

func openLiteSpeedRunning() bool {
	if state, ok := detectSystemdStatus("lshttpd.service", "lsws.service", "openlitespeed.service"); ok {
		switch strings.ToLower(strings.TrimSpace(state)) {
		case "active", "activating", "reloading":
			return true
		}
	}
	_, err := commandOutputTrimmed(olsLSWSControlPath, "status")
	return err == nil
}

func waitForOpenLiteSpeedTransition(previousPID string, pidReader func() string, isRunning func() bool, sleep func(time.Duration), timeout, interval time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		currentPID := strings.TrimSpace(pidReader())
		if currentPID != "" && currentPID != previousPID && isRunning() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		sleep(interval)
	}
}

func reloadOpenLiteSpeedWithHooks(runCommand func(string, ...string) (string, error), pidReader func() string, isRunning func() bool, sleep func(time.Duration)) error {
	previousPID := strings.TrimSpace(pidReader())
	_, reloadErr := runCommand(olsLSWSControlPath, "reload")
	if reloadErr == nil {
		return nil
	}

	if waitForOpenLiteSpeedTransition(previousPID, pidReader, isRunning, sleep, olsReloadWaitTimeout, olsReloadPollInterval) {
		return nil
	}

	_, restartErr := runCommand(olsLSWSControlPath, "restart")
	if restartErr == nil {
		return nil
	}

	if waitForOpenLiteSpeedTransition(previousPID, pidReader, isRunning, sleep, olsReloadWaitTimeout, olsReloadPollInterval) {
		return nil
	}

	return fmt.Errorf("openlitespeed reload failed: %v (restart failed: %v)", reloadErr, restartErr)
}

func reloadOpenLiteSpeed() error {
	return reloadOpenLiteSpeedWithHooks(commandOutputTrimmed, currentOpenLiteSpeedPID, openLiteSpeedRunning, olsSleep)
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
		vhostDir := filepath.Dir(path)
		if err := os.MkdirAll(vhostDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			return err
		}
		if err := ensureOLSManagedVhostDirOwnership(vhostDir); err != nil {
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
