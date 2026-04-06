package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
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
	PGAdmin    dbToolPGAdminSession
}

type dbToolPGAdminSession struct {
	Email    string
	Password string
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

	principal, hasPrincipal := principalFromContext(r.Context())
	issuer := "system"
	if hasPrincipal {
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
		} else {
			if !hasPrincipal {
				writeError(w, http.StatusUnauthorized, "Unauthorized.")
				return
			}
			scopeDBs, primaryDB, err := s.resolvePrincipalMariaDBScope(principal)
			if err != nil {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			tempUser, tempPass, tempHost, err := createRuntimeTemporaryDBUser("mariadb", primaryDB, "")
			if err != nil {
				writeError(w, http.StatusBadGateway, "Failed to prepare temporary database login user.")
				return
			}
			if err := grantRuntimeTemporaryMariaDBUserScope(tempUser, scopeDBs); err != nil {
				_ = dropRuntimeTemporaryDBUser("mariadb", tempUser)
				writeError(w, http.StatusBadGateway, "Failed to grant scoped database privileges for temporary login user.")
				return
			}
			tokenItem.DBName = primaryDB
			tokenItem.DBUser = tempUser
			launchSecret = dbToolLaunchSecret{
				Tool:      tool,
				Domain:    domain,
				ExpiresAt: expiresAt,
				Credential: dbToolCredential{
					Engine:    "mariadb",
					DBName:    primaryDB,
					Username:  tempUser,
					Password:  tempPass,
					Host:      firstNonEmpty(tempHost, "localhost"),
					Temporary: true,
				},
			}
			hasLaunchSecret = true
		}
	case "pgadmin":
		if !hasPrincipal {
			writeError(w, http.StatusUnauthorized, "Unauthorized.")
			return
		}

		var (
			scopeDBs  []string
			primaryDB string
		)
		if domain != "" {
			link, err := s.resolveDomainDBLink(domain, "postgresql")
			if err != nil {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			scopeDBs = []string{link.DBName}
			primaryDB = link.DBName
		} else {
			resolvedScope, resolvedPrimary, err := s.resolvePrincipalPostgresScope(principal)
			if err != nil {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			scopeDBs = resolvedScope
			primaryDB = resolvedPrimary
		}

		tempUser, tempPass, tempHost, err := createRuntimeTemporaryDBUser("postgresql", primaryDB, "")
		if err != nil {
			writeError(w, http.StatusBadGateway, "Failed to prepare temporary PostgreSQL login user.")
			return
		}
		if err := grantRuntimeTemporaryPostgresUserScope(tempUser, scopeDBs); err != nil {
			_ = dropRuntimeTemporaryDBUser("postgresql", tempUser)
			writeError(w, http.StatusBadGateway, "Failed to grant scoped PostgreSQL privileges for temporary login user.")
			return
		}

		pgAdminEmail, pgAdminPassword, err := ensurePGAdminScopedCredentials(principal)
		if err != nil {
			_ = dropRuntimeTemporaryDBUser("postgresql", tempUser)
			writeError(w, http.StatusBadGateway, "Failed to prepare scoped pgAdmin login user.")
			return
		}
		credential := dbToolCredential{
			Engine:    "postgresql",
			DBName:    primaryDB,
			Username:  tempUser,
			Password:  tempPass,
			Host:      firstNonEmpty(tempHost, "127.0.0.1"),
			Temporary: true,
		}
		if err := prepareScopedPGAdminServerProfile(pgAdminEmail, credential); err != nil {
			_ = dropRuntimeTemporaryDBUser("postgresql", tempUser)
			writeError(w, http.StatusBadGateway, "Failed to prepare scoped pgAdmin server profile.")
			return
		}

		tokenItem.DBName = primaryDB
		tokenItem.DBUser = tempUser
		launchSecret = dbToolLaunchSecret{
			Tool:       tool,
			Domain:     domain,
			ExpiresAt:  expiresAt,
			Credential: credential,
			PGAdmin: dbToolPGAdminSession{
				Email:    pgAdminEmail,
				Password: pgAdminPassword,
			},
		}
		hasLaunchSecret = true
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

func grantRuntimeTemporaryMariaDBUserScope(username string, dbNames []string) error {
	username = sanitizeDBName(username)
	if username == "" {
		return fmt.Errorf("temporary user is required")
	}
	uniqueDBs := map[string]struct{}{}
	for _, value := range dbNames {
		dbName := sanitizeDBName(value)
		if dbName == "" {
			continue
		}
		uniqueDBs[dbName] = struct{}{}
	}
	if len(uniqueDBs) == 0 {
		return nil
	}
	orderedDBs := make([]string, 0, len(uniqueDBs))
	for dbName := range uniqueDBs {
		orderedDBs = append(orderedDBs, dbName)
	}
	sort.Strings(orderedDBs)
	for _, dbName := range orderedDBs {
		query := fmt.Sprintf(
			"GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost';",
			dbName,
			sqlQuote(username),
		)
		if _, err := mysqlExec(query); err != nil {
			return err
		}
	}
	_, err := mysqlExec("FLUSH PRIVILEGES;")
	return err
}

func grantRuntimeTemporaryPostgresUserScope(username string, dbNames []string) error {
	username = sanitizeDBName(username)
	if username == "" {
		return fmt.Errorf("temporary user is required")
	}
	uniqueDBs := map[string]struct{}{}
	for _, value := range dbNames {
		dbName := sanitizeDBName(value)
		if dbName == "" {
			continue
		}
		uniqueDBs[dbName] = struct{}{}
	}
	if len(uniqueDBs) == 0 {
		return nil
	}

	roleIdent := postgresIdentifier(username)
	orderedDBs := make([]string, 0, len(uniqueDBs))
	for dbName := range uniqueDBs {
		orderedDBs = append(orderedDBs, dbName)
	}
	sort.Strings(orderedDBs)
	for _, dbName := range orderedDBs {
		dbIdent := postgresIdentifier(dbName)
		if _, err := postgresExec(fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s;", dbIdent, roleIdent)); err != nil {
			return err
		}
		_, _ = postgresExecDB(dbName, fmt.Sprintf("GRANT USAGE ON SCHEMA public TO %s;", roleIdent))
		_, _ = postgresExecDB(dbName, fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO %s;", roleIdent))
		_, _ = postgresExecDB(dbName, fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO %s;", roleIdent))
		_, _ = postgresExecDB(dbName, fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON TABLES TO %s;", roleIdent))
		_, _ = postgresExecDB(dbName, fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON SEQUENCES TO %s;", roleIdent))
	}
	return nil
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
	if hasSecret && secret.Credential.Password != "" && secret.Credential.Temporary {
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
	switch tool {
	case "phpmyadmin":
		if hasSecret && secret.Credential.Password != "" {
			writePHPMyAdminAutoLoginPage(w, targetURL, secret.Credential, item.Domain)
			return
		}
	case "pgadmin":
		if hasSecret && strings.TrimSpace(secret.PGAdmin.Email) != "" && strings.TrimSpace(secret.PGAdmin.Password) != "" {
			writePGAdminAutoLoginPage(w, targetURL, secret.PGAdmin.Email, secret.PGAdmin.Password, item.Domain, item.DBName, item.DBUser)
			return
		}
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
const loginPath = %s;
const loginUrl = new URL(loginPath, window.location.origin).toString();
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

func (s *service) resolvePrincipalMariaDBScope(pr servicePrincipal) ([]string, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allowed := map[string]struct{}{}
	for _, item := range s.state.MariaDBs {
		if !s.principalCanAccessDatabaseLocked(pr, item) {
			continue
		}
		dbName := sanitizeDBName(item.Name)
		if dbName == "" {
			continue
		}
		allowed[dbName] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil, "", fmt.Errorf("Bu hesap icin erisilebilir MariaDB veritabani bulunamadi")
	}

	ordered := make([]string, 0, len(allowed))
	for dbName := range allowed {
		ordered = append(ordered, dbName)
	}
	sort.Strings(ordered)
	primary := ordered[0]
	bestLinkedAt := int64(-1)
	for _, link := range s.state.DBLinks {
		if normalizeEngine(link.Engine) != "mariadb" {
			continue
		}
		dbName := sanitizeDBName(link.DBName)
		if dbName == "" {
			continue
		}
		if _, ok := allowed[dbName]; !ok {
			continue
		}
		if link.LinkedAt > bestLinkedAt {
			bestLinkedAt = link.LinkedAt
			primary = dbName
		}
	}
	return ordered, primary, nil
}

func (s *service) resolvePrincipalPostgresScope(pr servicePrincipal) ([]string, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allowed := map[string]struct{}{}
	for _, item := range s.state.PostgresDBs {
		if !s.principalCanAccessDatabaseLocked(pr, item) {
			continue
		}
		dbName := sanitizeDBName(item.Name)
		if dbName == "" {
			continue
		}
		allowed[dbName] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil, "", fmt.Errorf("Bu hesap icin erisilebilir PostgreSQL veritabani bulunamadi")
	}

	ordered := make([]string, 0, len(allowed))
	for dbName := range allowed {
		ordered = append(ordered, dbName)
	}
	sort.Strings(ordered)
	primary := ordered[0]
	bestLinkedAt := int64(-1)
	for _, link := range s.state.DBLinks {
		if normalizeEngine(link.Engine) != "postgresql" {
			continue
		}
		dbName := sanitizeDBName(link.DBName)
		if dbName == "" {
			continue
		}
		if _, ok := allowed[dbName]; !ok {
			continue
		}
		if link.LinkedAt > bestLinkedAt {
			bestLinkedAt = link.LinkedAt
			primary = dbName
		}
	}
	return ordered, primary, nil
}

func ensurePGAdminScopedCredentials(pr servicePrincipal) (string, string, error) {
	email := pgAdminScopedEmailForPrincipal(pr)
	password := generateSecret(20)
	if err := upsertPGAdminInternalUser(email, password); err != nil {
		return "", "", err
	}
	return email, password, nil
}

func pgAdminScopedEmailForPrincipal(pr servicePrincipal) string {
	identity := strings.ToLower(strings.TrimSpace(firstNonEmpty(pr.Email, pr.Username, pr.Name, "user")))
	localPart := sanitizeName(strings.SplitN(identity, "@", 2)[0])
	localPart = strings.Trim(localPart, "_-")
	if localPart == "" {
		localPart = "user"
	}
	if len(localPart) > 24 {
		localPart = localPart[:24]
	}
	hash := sha1.Sum([]byte(identity + "|" + strings.ToLower(strings.TrimSpace(pr.Role))))
	suffix := hex.EncodeToString(hash[:])[:10]
	return fmt.Sprintf("apsso_%s_%s@%s", localPart, suffix, pgAdminScopedEmailDomain())
}

func pgAdminScopedEmailDomain() string {
	explicit := firstNonEmpty(
		strings.TrimSpace(os.Getenv("AURAPANEL_PGADMIN_SCOPED_EMAIL_DOMAIN")),
		strings.TrimSpace(readEnvFileValue(adminServiceEnvPath(), "AURAPANEL_PGADMIN_SCOPED_EMAIL_DOMAIN")),
	)
	if looksLikeDomain(explicit) {
		return strings.ToLower(strings.TrimSpace(explicit))
	}

	defaultEmail := firstNonEmpty(
		strings.TrimSpace(os.Getenv("AURAPANEL_PGADMIN_DEFAULT_EMAIL")),
		strings.TrimSpace(readEnvFileValue(adminServiceEnvPath(), "AURAPANEL_PGADMIN_DEFAULT_EMAIL")),
	)
	if at := strings.LastIndex(defaultEmail, "@"); at > 0 && at+1 < len(defaultEmail) {
		domain := strings.ToLower(strings.TrimSpace(defaultEmail[at+1:]))
		if looksLikeDomain(domain) {
			return domain
		}
	}
	return "aurapanel.info"
}

func looksLikeDomain(value string) bool {
	value = strings.Trim(strings.ToLower(strings.TrimSpace(value)), ".")
	if value == "" {
		return false
	}
	if strings.Contains(value, " ") {
		return false
	}
	return strings.Contains(value, ".")
}

func upsertPGAdminInternalUser(email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	password = strings.TrimSpace(password)
	if email == "" || password == "" {
		return fmt.Errorf("pgAdmin user credentials are required")
	}

	if _, err := runPGAdminSetup("update-user", "--password", password, "--role", "Administrator", "--active", email); err == nil {
		return nil
	}
	if _, err := runPGAdminSetup("add-user", "--role", "Administrator", email, password); err == nil {
		return nil
	}
	if _, err := runPGAdminSetup("update-user", "--password", password, "--role", "Administrator", "--active", email); err == nil {
		return nil
	}
	return fmt.Errorf("pgAdmin scoped user could not be created/updated")
}

func prepareScopedPGAdminServerProfile(email string, credential dbToolCredential) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return fmt.Errorf("pgAdmin user email is required")
	}
	if strings.TrimSpace(credential.Username) == "" || strings.TrimSpace(credential.Password) == "" {
		return fmt.Errorf("temporary PostgreSQL credential is required")
	}

	container := pgAdminContainerName()
	host, port := resolvePGAdminPostgresEndpoint(container)
	passfilePath := fmt.Sprintf("/tmp/aurapanel-%s.pgpass", sanitizeDBName(credential.Username))
	serverJSONPath := fmt.Sprintf("/tmp/aurapanel-%s-servers.json", sanitizeDBName(credential.Username))

	passfileLine := fmt.Sprintf("%s:%d:*:%s:%s\n", host, port, credential.Username, credential.Password)
	passTmpFile, err := os.CreateTemp("", "aurapanel-pgpass-*")
	if err != nil {
		return err
	}
	passTmpPath := passTmpFile.Name()
	if _, writeErr := passTmpFile.WriteString(passfileLine); writeErr != nil {
		passTmpFile.Close()
		_ = os.Remove(passTmpPath)
		return writeErr
	}
	if closeErr := passTmpFile.Close(); closeErr != nil {
		_ = os.Remove(passTmpPath)
		return closeErr
	}
	defer os.Remove(passTmpPath)

	if _, err := runDockerCommandTrimmed(25*time.Second, "cp", passTmpPath, fmt.Sprintf("%s:%s", container, passfilePath)); err != nil {
		return err
	}
	_, _ = runDockerCommandTrimmed(12*time.Second, "exec", container, "chown", "pgadmin:root", passfilePath)
	if _, err := runDockerCommandTrimmed(12*time.Second, "exec", container, "chmod", "600", passfilePath); err != nil {
		return err
	}

	serverPayload := map[string]interface{}{
		"Servers": map[string]interface{}{
			"1": map[string]interface{}{
				"Name":          fmt.Sprintf("AuraPanel (%s)", credential.DBName),
				"Group":         "AuraPanel",
				"Host":          host,
				"Port":          port,
				"MaintenanceDB": credential.DBName,
				"Username":      credential.Username,
				"PassFile":      passfilePath,
				"SSLMode":       "prefer",
			},
		},
	}
	serverBytes, err := json.Marshal(serverPayload)
	if err != nil {
		return err
	}
	serverTmpFile, err := os.CreateTemp("", "aurapanel-pgservers-*.json")
	if err != nil {
		return err
	}
	serverTmpPath := serverTmpFile.Name()
	if _, writeErr := serverTmpFile.Write(serverBytes); writeErr != nil {
		serverTmpFile.Close()
		_ = os.Remove(serverTmpPath)
		return writeErr
	}
	if closeErr := serverTmpFile.Close(); closeErr != nil {
		_ = os.Remove(serverTmpPath)
		return closeErr
	}
	defer os.Remove(serverTmpPath)

	if _, err := runDockerCommandTrimmed(25*time.Second, "cp", serverTmpPath, fmt.Sprintf("%s:%s", container, serverJSONPath)); err != nil {
		return err
	}
	defer runDockerCommandTrimmed(8*time.Second, "exec", container, "rm", "-f", serverJSONPath)

	if _, err := runPGAdminSetup("load-servers", "--replace", "--user", email, serverJSONPath); err != nil {
		return err
	}
	return nil
}

func resolvePGAdminPostgresEndpoint(container string) (string, int) {
	host := firstNonEmpty(
		strings.TrimSpace(os.Getenv("AURAPANEL_PGADMIN_POSTGRES_HOST")),
		strings.TrimSpace(readEnvFileValue(adminServiceEnvPath(), "AURAPANEL_PGADMIN_POSTGRES_HOST")),
		strings.TrimSpace(os.Getenv("AURAPANEL_POSTGRES_HOST")),
		strings.TrimSpace(readEnvFileValue(adminServiceEnvPath(), "AURAPANEL_POSTGRES_HOST")),
	)
	if host == "" {
		if gateway, err := resolvePGAdminContainerGateway(container); err == nil && gateway != "" {
			host = gateway
		}
	}
	if host == "" {
		host = "127.0.0.1"
	}

	port := 5432
	for _, raw := range []string{
		strings.TrimSpace(os.Getenv("AURAPANEL_PGADMIN_POSTGRES_PORT")),
		strings.TrimSpace(readEnvFileValue(adminServiceEnvPath(), "AURAPANEL_PGADMIN_POSTGRES_PORT")),
		strings.TrimSpace(os.Getenv("AURAPANEL_POSTGRES_PORT")),
		strings.TrimSpace(readEnvFileValue(adminServiceEnvPath(), "AURAPANEL_POSTGRES_PORT")),
	} {
		if raw == "" {
			continue
		}
		if value, err := strconv.Atoi(raw); err == nil && value > 0 && value < 65536 {
			port = value
			break
		}
	}
	return host, port
}

func resolvePGAdminContainerGateway(container string) (string, error) {
	output, err := runDockerCommandTrimmed(10*time.Second, "exec", container, "ip", "route")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 {
			continue
		}
		if fields[0] == "default" && fields[1] == "via" {
			candidate := strings.TrimSpace(fields[2])
			if ip := net.ParseIP(candidate); ip != nil {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("docker gateway not found")
}

func runPGAdminSetup(args ...string) (string, error) {
	container := pgAdminContainerName()
	pythonBin := strings.TrimSpace(envOr("AURAPANEL_PGADMIN_SETUP_PYTHON", "/venv/bin/python3"))
	setupPath := strings.TrimSpace(envOr("AURAPANEL_PGADMIN_SETUP_PATH", "/pgadmin4/setup.py"))
	commandArgs := []string{"exec", container, pythonBin, setupPath}
	commandArgs = append(commandArgs, args...)
	return runDockerCommandTrimmed(45*time.Second, commandArgs...)
}

func runDockerCommandTrimmed(timeout time.Duration, args ...string) (string, error) {
	output, err := runCommandCombinedOutputWithTimeout(timeout, "docker", args...)
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed == "" {
			trimmed = err.Error()
		}
		return "", fmt.Errorf("%s", trimmed)
	}
	return trimmed, nil
}

func pgAdminContainerName() string {
	return firstNonEmpty(
		strings.TrimSpace(os.Getenv("AURAPANEL_PGADMIN_CONTAINER_NAME")),
		strings.TrimSpace(readEnvFileValue(adminServiceEnvPath(), "AURAPANEL_PGADMIN_CONTAINER_NAME")),
		"aurapanel-pgadmin",
	)
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
