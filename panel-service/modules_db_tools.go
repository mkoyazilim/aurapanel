package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type dbToolCredential struct {
	Engine    string
	DBName    string
	Username  string
	Password  string
	Host      string
	Temporary bool
}

type dbToolLaunchSecret struct {
	Tool       string
	Domain     string
	ExpiresAt  time.Time
	Credential dbToolCredential
}

type dbToolTempUser struct {
	Engine    string
	Username  string
	ExpiresAt time.Time
}

func dbToolTempUserKey(engine, username string) string {
	engine = normalizeEngine(engine)
	username = sanitizeDBName(username)
	if engine == "" || username == "" {
		return ""
	}
	return engine + "|" + username
}

func normalizeDBTool(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "phpmyadmin":
		return "phpmyadmin"
	case "pgadmin", "pgadmin4":
		return "pgadmin"
	default:
		return ""
	}
}

func dbEngineForTool(tool string) string {
	switch normalizeDBTool(tool) {
	case "phpmyadmin":
		return "mariadb"
	case "pgadmin":
		return "postgresql"
	default:
		return ""
	}
}

func (s *service) handleDBToolSSO(w http.ResponseWriter, r *http.Request, tool string) {
	tool = normalizeDBTool(tool)
	if tool == "" {
		writeError(w, http.StatusBadRequest, "Unsupported database tool.")
		return
	}

	var payload struct {
		TTLSeconds int    `json:"ttl_seconds"`
		Domain     string `json:"domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid DB tool SSO payload.")
		return
	}

	domain := normalizeDomain(payload.Domain)
	if domain != "" && !s.requireDomainAccess(w, r, domain) {
		return
	}

	ttlSeconds := clampInt(payload.TTLSeconds, 60, 900)
	token := generateSecret(12)
	expiresAt := time.Now().UTC().Add(time.Duration(ttlSeconds) * time.Second)

	issuer := "system"
	if principal, ok := principalFromContext(r.Context()); ok {
		issuer = firstNonEmpty(principal.Email, principal.Username, principal.Name, "system")
	}

	tokenItem := DBToolToken{
		Token:     token,
		Tool:      tool,
		IssuedBy:  issuer,
		Domain:    domain,
		Engine:    dbEngineForTool(tool),
		ExpiresAt: expiresAt,
	}
	launchSecret := dbToolLaunchSecret{}
	hasLaunchSecret := false

	switch tool {
	case "phpmyadmin":
		if domain != "" {
			link, err := s.resolveDomainDBLink(domain, "mariadb")
			if err != nil {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			tempUser, tempPass, tempHost, err := createRuntimeTemporaryDBUser("mariadb", link.DBName, link.DBUser)
			if err != nil {
				writeError(w, http.StatusBadGateway, "Failed to prepare temporary database login user.")
				return
			}
			tokenItem.DBName = link.DBName
			tokenItem.DBUser = link.DBUser
			launchSecret = dbToolLaunchSecret{
				Tool:      tool,
				Domain:    domain,
				ExpiresAt: expiresAt,
				Credential: dbToolCredential{
					Engine:    "mariadb",
					DBName:    link.DBName,
					Username:  tempUser,
					Password:  tempPass,
					Host:      firstNonEmpty(tempHost, "localhost"),
					Temporary: true,
				},
			}
			hasLaunchSecret = true
		}
	case "pgadmin":
		if domain != "" {
			link, err := s.resolveDomainDBLink(domain, "postgresql")
			if err != nil {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			tokenItem.DBName = link.DBName
			tokenItem.DBUser = link.DBUser
		}
		if _, _, err := resolvePGAdminCredentials(); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
	}

	s.mu.Lock()
	if s.modules.DBToolTokens == nil {
		s.modules.DBToolTokens = map[string]DBToolToken{}
	}
	if s.dbToolLaunchSecrets == nil {
		s.dbToolLaunchSecrets = map[string]dbToolLaunchSecret{}
	}
	s.modules.DBToolTokens[token] = tokenItem
	if hasLaunchSecret {
		s.dbToolLaunchSecrets[token] = launchSecret
	}
	s.appendActivityLocked(issuer, "db_tool_launch", fmt.Sprintf("%s launch token issued.", tool), "")
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"url":        fmt.Sprintf("/api/v1/db/tools/%s/sso/consume?token=%s", tool, token),
			"tool":       tool,
			"domain":     domain,
			"expires_at": expiresAt.Format(time.RFC3339),
		},
	})
}

func (s *service) handleDBToolConsume(w http.ResponseWriter, r *http.Request, tool string) {
	tool = normalizeDBTool(tool)
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if tool == "" || token == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("<html><body><h1>Invalid DB tool token</h1></body></html>"))
		return
	}

	now := time.Now().UTC()
	s.mu.Lock()
	item, ok := s.modules.DBToolTokens[token]
	if ok {
		delete(s.modules.DBToolTokens, token)
	}
	secret, hasSecret := s.dbToolLaunchSecrets[token]
	if hasSecret {
		delete(s.dbToolLaunchSecrets, token)
	}
	s.mu.Unlock()

	if !ok || item.Tool != tool || item.ExpiresAt.Before(now) {
		if hasSecret && secret.Credential.Temporary {
			_ = dropRuntimeTemporaryDBUser(secret.Credential.Engine, secret.Credential.Username)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte("<html><body><h1>DB tool token expired</h1></body></html>"))
		return
	}
	s.registerDBToolAccess(item.IssuedBy, serviceClientIP(r), item.ExpiresAt)

	targetURL := resolveDBToolBaseURL(r, tool)
	switch tool {
	case "phpmyadmin":
		if hasSecret && secret.Credential.Password != "" {
			if secret.Credential.Temporary {
				expiresAt := now.Add(defaultDBToolTempUserTTL)
				s.mu.Lock()
				if s.dbToolTempUsers == nil {
					s.dbToolTempUsers = map[string]dbToolTempUser{}
				}
				if key := dbToolTempUserKey(secret.Credential.Engine, secret.Credential.Username); key != "" {
					s.dbToolTempUsers[key] = dbToolTempUser{
						Engine:    secret.Credential.Engine,
						Username:  secret.Credential.Username,
						ExpiresAt: expiresAt,
					}
				}
				s.mu.Unlock()
			}
			writePHPMyAdminAutoLoginPage(w, targetURL, secret.Credential, item.Domain)
			return
		}
	case "pgadmin":
		email, password, err := resolvePGAdminCredentials()
		if err == nil {
			writePGAdminAutoLoginPage(w, targetURL, email, password, item.Domain, item.DBName, item.DBUser)
			return
		}
	}
	http.Redirect(w, r, targetURL, http.StatusFound)
}

func writePHPMyAdminAutoLoginPage(w http.ResponseWriter, targetURL string, credential dbToolCredential, domain string) {
	message := "phpMyAdmin oturumu aciliyor..."
	if domain != "" {
		message = fmt.Sprintf("%s icin phpMyAdmin oturumu aciliyor...", domain)
	}
	loginPath := browserPathFromURL(targetURL)
	writeDBToolAutoLoginPage(w, message, fmt.Sprintf(`
const loginUrl = %s;
const username = %s;
const password = %s;

async function run() {
  const initialRes = await fetch(loginUrl, { credentials: 'include', redirect: 'follow' });
  const initialHtml = await initialRes.text();
  const doc = new DOMParser().parseFromString(initialHtml, 'text/html');
  const userField = doc.querySelector('input[name=\"pma_username\"]');
  if (!userField) {
    window.location.href = loginUrl;
    return;
  }
  const form = userField.closest('form') || doc.querySelector('form[name=\"login_form\"], form[action*=\"index.php\"], form');
  if (!form) {
    throw new Error('phpMyAdmin login form bulunamadi.');
  }

  const action = form.getAttribute('action') || loginUrl;
  const submitUrl = new URL(action, loginUrl).toString();
  const params = new URLSearchParams();
  form.querySelectorAll('input[type=\"hidden\"][name]').forEach((field) => {
    params.set(field.name, field.value || '');
  });
  params.set('pma_username', username);
  params.set('pma_password', password);
  if (!params.has('server')) {
    params.set('server', '1');
  }

  const authRes = await fetch(submitUrl, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: params.toString(),
    redirect: 'follow'
  });
  if (!authRes.ok && authRes.status >= 400) {
    throw new Error('phpMyAdmin otomatik giris istegi basarisiz.');
  }

  window.location.href = loginUrl;
}

run().catch((error) => {
  const el = document.getElementById('status');
  if (el) {
    el.textContent = error?.message || 'Oturum acma basarisiz oldu.';
    el.style.color = '#b91c1c';
  }
});
`, strconv.Quote(loginPath), strconv.Quote(credential.Username), strconv.Quote(credential.Password)))
}

func writePGAdminAutoLoginPage(w http.ResponseWriter, targetURL, email, password, domain, dbName, dbUser string) {
	message := "pgAdmin oturumu aciliyor..."
	if domain != "" {
		message = fmt.Sprintf("%s icin pgAdmin oturumu aciliyor...", domain)
	}
	hint := ""
	if dbName != "" && dbUser != "" {
		hint = fmt.Sprintf("Hedef veritabani: %s (kullanici: %s)", dbName, dbUser)
	}
	targetPath := browserPathFromURL(targetURL)
	writeDBToolAutoLoginPage(w, message, fmt.Sprintf(`
const targetUrl = %s;
const loginUrl = new URL('/pgadmin4/login?next=' + encodeURIComponent('/pgadmin4/'), window.location.origin).toString();
const email = %s;
const password = %s;
const hint = %s;

async function run() {
  const loginPage = await fetch(loginUrl, { credentials: 'include', redirect: 'follow' });
  const html = await loginPage.text();
  const doc = new DOMParser().parseFromString(html, 'text/html');
  const form = doc.querySelector('form');
  if (!form) {
    window.location.href = targetUrl;
    return;
  }

  const action = form.getAttribute('action') || loginUrl;
  const submitUrl = new URL(action, loginUrl).toString();
  const params = new URLSearchParams();
  form.querySelectorAll('input[type=\"hidden\"][name]').forEach((field) => {
    params.set(field.name, field.value || '');
  });
  params.set('email', email);
  params.set('password', password);

  const authRes = await fetch(submitUrl, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: params.toString(),
    redirect: 'follow'
  });
  if (!authRes.ok && authRes.status >= 400) {
    throw new Error('pgAdmin otomatik giris istegi basarisiz.');
  }

  window.location.href = targetUrl;
}

run().catch((error) => {
  const el = document.getElementById('status');
  if (el) {
    el.textContent = error?.message || 'Oturum acma basarisiz oldu.';
    el.style.color = '#b91c1c';
  }
  const hintEl = document.getElementById('hint');
  if (hintEl && hint) {
    hintEl.textContent = hint;
  }
});
`, strconv.Quote(targetPath), strconv.Quote(email), strconv.Quote(password), strconv.Quote(hint)))
}

func writeDBToolAutoLoginPage(w http.ResponseWriter, message, script string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="tr">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>AuraPanel DB Tool SSO</title>
  <style>
    body{font-family:Arial,sans-serif;background:#0f172a;color:#e2e8f0;display:flex;justify-content:center;align-items:center;min-height:100vh;margin:0}
    .card{background:#111827;border:1px solid #1f2937;border-radius:10px;padding:24px;max-width:560px;width:90%%}
    .title{font-size:20px;margin:0 0 10px 0}
    .muted{color:#94a3b8;margin:0}
  </style>
</head>
<body>
  <div class="card">
    <h1 class="title">DB Tool SSO</h1>
    <p id="status" class="muted">%s</p>
    <p id="hint" class="muted"></p>
  </div>
  <script>%s</script>
</body>
</html>`, message, script)
}

func resolvePGAdminCredentials() (string, string, error) {
	email := firstNonEmpty(
		strings.TrimSpace(os.Getenv("AURAPANEL_PGADMIN_DEFAULT_EMAIL")),
		strings.TrimSpace(readEnvFileValue(adminServiceEnvPath(), "AURAPANEL_PGADMIN_DEFAULT_EMAIL")),
	)
	password := firstNonEmpty(
		strings.TrimSpace(os.Getenv("AURAPANEL_PGADMIN_DEFAULT_PASSWORD")),
		strings.TrimSpace(readEnvFileValue(adminServiceEnvPath(), "AURAPANEL_PGADMIN_DEFAULT_PASSWORD")),
	)
	if email == "" || password == "" {
		return "", "", fmt.Errorf("pgAdmin default login bilgisi bulunamadi. AURAPANEL_PGADMIN_DEFAULT_EMAIL/PASSWORD tanimlanmali")
	}
	return email, password, nil
}

func (s *service) resolveDomainDBLink(domain, engine string) (WebsiteDBLink, error) {
	domain = normalizeDomain(domain)
	engine = normalizeEngine(engine)
	if domain == "" {
		return WebsiteDBLink{}, fmt.Errorf("domain is required")
	}
	if engine == "" {
		return WebsiteDBLink{}, fmt.Errorf("database engine is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	selected := WebsiteDBLink{}
	found := false
	for _, item := range s.state.DBLinks {
		if normalizeDomain(item.Domain) != domain || normalizeEngine(item.Engine) != engine {
			continue
		}
		if !found || item.LinkedAt > selected.LinkedAt {
			selected = item
			found = true
		}
	}
	if found {
		selected.Engine = engine
		selected.DBHost = normalizeDBHost(firstNonEmpty(selected.DBHost, "localhost"))
		selected.DBName = sanitizeDBName(selected.DBName)
		selected.DBUser = sanitizeDBName(selected.DBUser)
		if selected.DBName != "" && selected.DBUser != "" {
			return selected, nil
		}
	}

	var dbs []DatabaseRecord
	var users []DatabaseUser
	if engine == "mariadb" {
		dbs = s.state.MariaDBs
		users = s.state.MariaUsers
	} else {
		dbs = s.state.PostgresDBs
		users = s.state.PostgresUsers
	}

	for _, db := range dbs {
		if normalizeDomain(db.SiteDomain) != domain {
			continue
		}
		dbName := sanitizeDBName(db.Name)
		if dbName == "" {
			continue
		}
		for _, user := range users {
			if sanitizeDBName(user.LinkedDBName) != dbName {
				continue
			}
			dbUser := sanitizeDBName(user.Username)
			if dbUser == "" {
				continue
			}
			return WebsiteDBLink{
				Domain:   domain,
				Engine:   engine,
				DBName:   dbName,
				DBUser:   dbUser,
				DBHost:   normalizeDBHost(firstNonEmpty(user.Host, "localhost")),
				LinkedAt: time.Now().UTC().Unix(),
			}, nil
		}
	}

	toolName := "phpMyAdmin"
	if engine == "postgresql" {
		toolName = "pgAdmin"
	}
	return WebsiteDBLink{}, fmt.Errorf("%s icin %s veritabani baglantisi bulunamadi", domain, toolName)
}

func resolveDBToolBaseURL(r *http.Request, tool string) string {
	tool = normalizeDBTool(tool)
	if tool == "" {
		return "/"
	}

	baseURL := ""
	defaultPath := ""
	switch tool {
	case "phpmyadmin":
		baseURL = strings.TrimSpace(os.Getenv("AURAPANEL_PHPMYADMIN_BASE_URL"))
		defaultPath = "/phpmyadmin/index.php"
	case "pgadmin":
		baseURL = strings.TrimSpace(os.Getenv("AURAPANEL_PGADMIN_BASE_URL"))
		defaultPath = "/pgadmin4/"
	}
	if baseURL == "" {
		baseURL = defaultPath
	}

	lower := strings.ToLower(baseURL)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return baseURL
	}

	origin := servicePublicOrigin(r)
	if origin == "" {
		if strings.HasPrefix(baseURL, "/") {
			return baseURL
		}
		return "/" + strings.TrimLeft(baseURL, "/")
	}
	if strings.HasPrefix(baseURL, "/") {
		return origin + baseURL
	}
	return origin + "/" + strings.TrimLeft(baseURL, "/")
}

func servicePublicOrigin(r *http.Request) string {
	if panelEdgeSingleDomainEnabled() {
		if edgeDomain := panelEdgeDomain(); edgeDomain != "" {
			return fmt.Sprintf("https://%s", edgeDomain)
		}
	}

	host := forwardedHeaderValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" {
		return ""
	}

	originalHost := host
	if parsedHost, _, err := net.SplitHostPort(host); err == nil && parsedHost != "" {
		if !isLoopbackHost(parsedHost) {
			// DB tools are exposed via web stack (80/443), not gateway API port.
			host = parsedHost
		}
	}

	scheme := forwardedHeaderValue(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	targetHost := host
	if isLoopbackHost(host) {
		targetHost = originalHost
	}

	return fmt.Sprintf("%s://%s", scheme, targetHost)
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func browserPathFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "/"
	}
	if strings.HasPrefix(raw, "/") {
		return raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "/" + strings.TrimLeft(raw, "/")
	}
	if !parsed.IsAbs() {
		path := parsed.String()
		if strings.HasPrefix(path, "/") {
			return path
		}
		return "/" + strings.TrimLeft(path, "/")
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}
	return path
}
