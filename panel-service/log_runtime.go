package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type logCatalogEntry struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group"`
}

type siteLogEntry struct {
	Domain string   `json:"domain"`
	Logs   []string `json:"logs"`
}

var staticLogCatalog = []logCatalogEntry{
	{ID: "system_syslog", Name: "System Log (syslog)", Group: "system"},
	{ID: "system_auth", Name: "Auth Log", Group: "system"},
	{ID: "system_dmesg", Name: "Kernel (dmesg)", Group: "system"},
	{ID: "system_journal", Name: "Journal (son 200 satır)", Group: "system"},
	{ID: "web_ols_access", Name: "OLS Access Log", Group: "webserver"},
	{ID: "web_ols_error", Name: "OLS Error Log", Group: "webserver"},
	{ID: "web_nginx_access", Name: "Nginx Access Log", Group: "webserver"},
	{ID: "web_nginx_error", Name: "Nginx Error Log", Group: "webserver"},
	{ID: "panel_service", Name: "Panel Service", Group: "panel"},
	{ID: "panel_gateway", Name: "API Gateway", Group: "panel"},
	{ID: "security_fail2ban", Name: "Fail2Ban", Group: "security"},
	{ID: "db_mariadb", Name: "MariaDB Error Log", Group: "database"},
	{ID: "db_postgresql", Name: "PostgreSQL Log", Group: "database"},
	{ID: "mail_postfix", Name: "Postfix (mail.log)", Group: "mail"},
	{ID: "mail_dovecot", Name: "Dovecot Log", Group: "mail"},
}

func logSourceToPath(id string) string {
	switch id {
	case "system_syslog":
		return "/var/log/syslog"
	case "system_auth":
		return "/var/log/auth.log"
	case "web_ols_access":
		return "/usr/local/lsws/logs/access.log"
	case "web_ols_error":
		return "/usr/local/lsws/logs/error.log"
	case "web_nginx_access":
		return "/var/log/nginx/access.log"
	case "web_nginx_error":
		return "/var/log/nginx/error.log"
	case "security_fail2ban":
		return "/var/log/fail2ban.log"
	case "db_mariadb":
		return "/var/log/mysql/error.log"
	case "mail_postfix":
		return "/var/log/mail.log"
	case "mail_dovecot":
		if fileExists("/var/log/dovecot.log") {
			return "/var/log/dovecot.log"
		}
		return "/var/log/dovecot/dovecot.log"
	default:
		return ""
	}
}

func readLogSource(id string, lines int) ([]string, error) {
	if lines <= 0 {
		lines = 100
	}
	if lines > 1000 {
		lines = 1000
	}

	switch id {
	case "system_dmesg":
		return runLogCommand("dmesg", "-T")
	case "system_journal":
		return runLogCommand("journalctl", "--no-pager", "-n", fmt.Sprintf("%d", lines))
	case "panel_service":
		return runLogCommand("journalctl", "--no-pager", "-u", "aurapanel-service", "-n", fmt.Sprintf("%d", lines))
	case "panel_gateway":
		return runLogCommand("journalctl", "--no-pager", "-u", "aurapanel-api", "-n", fmt.Sprintf("%d", lines))
	case "db_postgresql":
		return readPostgresLog(lines)
	default:
		path := logSourceToPath(id)
		if path == "" {
			return nil, fmt.Errorf("unknown log source: %s", id)
		}
		return tailManagedFile(path, lines)
	}
}

func runLogCommand(name string, args ...string) ([]string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("command failed: %s %v: %w", name, args, err)
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return []string{}, nil
	}
	return strings.Split(text, "\n"), nil
}

func readPostgresLog(lines int) ([]string, error) {
	matches, err := filepath.Glob("/var/log/postgresql/postgresql-*.log")
	if err != nil || len(matches) == 0 {
		matches, err = filepath.Glob("/var/log/postgresql/*.log")
		if err != nil || len(matches) == 0 {
			return runLogCommand("journalctl", "--no-pager", "-u", "postgresql", "-n", fmt.Sprintf("%d", lines))
		}
	}
	return tailManagedFile(matches[len(matches)-1], lines)
}

func collectSiteLogCatalog() []siteLogEntry {
	entries := []siteLogEntry{}
	seen := map[string]bool{}

	dirEntries, err := os.ReadDir("/home")
	if err == nil {
		for _, entry := range dirEntries {
			if !entry.IsDir() {
				continue
			}
			domain := entry.Name()
			if seen[domain] {
				continue
			}
			logs := []string{}
			if _, err := os.Stat(fmt.Sprintf("/home/%s/logs/access.log", domain)); err == nil {
				logs = append(logs, "access")
			}
			if _, err := os.Stat(fmt.Sprintf("/home/%s/logs/error.log", domain)); err == nil {
				logs = append(logs, "error")
			}
			if len(logs) > 0 {
				seen[domain] = true
				entries = append(entries, siteLogEntry{Domain: domain, Logs: logs})
			}
		}
	}

	return entries
}

func (s *service) handleLogCatalog(w http.ResponseWriter) {
	catalog := struct {
		Categories []logCatalogEntry `json:"categories"`
		Sites      []siteLogEntry    `json:"sites"`
	}{
		Categories: staticLogCatalog,
		Sites:      collectSiteLogCatalog(),
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: catalog})
}

func (s *service) handleLogSource(w http.ResponseWriter, r *http.Request) {
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	lines := 100
	if v := r.URL.Query().Get("lines"); v != "" {
		if n, err := parseIntParam(v); err == nil && n > 0 {
			lines = n
		}
	}

	if source == "" {
		writeError(w, http.StatusBadRequest, "source parameter is required")
		return
	}

	// Site log'ları: site_{domain}_{kind}
	if strings.HasPrefix(source, "site_") {
		parts := strings.SplitN(strings.TrimPrefix(source, "site_"), "_", 2)
		if len(parts) != 2 {
			writeError(w, http.StatusBadRequest, "invalid site log source")
			return
		}
		domain := parts[0]
		kind := parts[1]
		if !isValidDomainName(domain) {
			writeError(w, http.StatusBadRequest, "invalid domain")
			return
		}
		logs, err := realSiteLogs(domain, kind)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: logs})
		return
	}

	logs, err := readLogSource(source, lines)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	output := map[string]interface{}{
		"source": source,
		"lines":  len(logs),
		"data":   logs,
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: output})
}

func parseIntParam(value string) (int, error) {
	var n int
	for _, c := range value {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
