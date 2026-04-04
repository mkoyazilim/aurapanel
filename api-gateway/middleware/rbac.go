package middleware

import (
	"net/http"
	"strings"
)

const (
	roleAdmin    = "admin"
	roleReseller = "reseller"
	roleUser     = "user"

	permissionUsersManage      = "users.manage"
	permissionPackagesManage   = "packages.manage"
	permissionResellerManage   = "reseller.manage"
	permissionWebsitesManage   = "websites.manage"
	permissionDNSManage        = "dns.manage"
	permissionDatabasesManage  = "databases.manage"
	permissionMailManage       = "mail.manage"
	permissionFTPManage        = "ftp.manage"
	permissionSFTPManage       = "sftp.manage"
	permissionBackupsManage    = "backups.manage"
	permissionAppsManage       = "apps.manage"
	permissionRuntimeManage    = "runtime.manage"
	permissionFilesManage      = "files.manage"
	permissionTerminalManage   = "terminal.manage"
	permissionPHPManage        = "php.manage"
	permissionSSLManage        = "ssl.manage"
	permissionSecurityManage   = "security.manage"
	permissionMonitoringRead   = "monitoring.read"
	permissionLogsRead         = "logs.read"
	permissionCronManage       = "cron.manage"
	permissionCloudflareManage = "cloudflare.manage"
	permissionMigrationManage  = "migration.manage"
	permissionCloudlinuxManage = "cloudlinux.manage"
	permissionMinioManage      = "minio.manage"
	permissionActivityRead     = "activity.read"
	permissionOpsManage        = "ops.manage"
	permissionPanelManage      = "panel.manage"
	permissionAIManage         = "ai.manage"
)

type permissionRule struct {
	prefix      string
	permissions []string
}

var permissionRules = []permissionRule{
	{prefix: "/api/v1/users", permissions: []string{permissionUsersManage}},
	{prefix: "/api/v1/acl", permissions: []string{permissionUsersManage}},
	{prefix: "/api/v1/packages", permissions: []string{permissionPackagesManage, permissionUsersManage}},
	{prefix: "/api/v1/reseller", permissions: []string{permissionResellerManage}},
	{prefix: "/api/v1/vhost", permissions: []string{permissionWebsitesManage}},
	{prefix: "/api/v1/websites", permissions: []string{permissionWebsitesManage}},
	{prefix: "/api/v1/analytics", permissions: []string{permissionWebsitesManage}},
	{prefix: "/api/v1/dns", permissions: []string{permissionDNSManage}},
	{prefix: "/api/v1/db/backup", permissions: []string{permissionBackupsManage}},
	{prefix: "/api/v1/db", permissions: []string{permissionDatabasesManage}},
	{prefix: "/api/v1/mail", permissions: []string{permissionMailManage}},
	{prefix: "/api/v1/ftp", permissions: []string{permissionFTPManage}},
	{prefix: "/api/v1/sftp", permissions: []string{permissionSFTPManage}},
	{prefix: "/api/v1/backup", permissions: []string{permissionBackupsManage}},
	{prefix: "/api/v1/apps", permissions: []string{permissionAppsManage}},
	{prefix: "/api/v1/wordpress", permissions: []string{permissionAppsManage}},
	{prefix: "/api/v1/plugins", permissions: []string{permissionAppsManage}},
	{prefix: "/api/v1/docker", permissions: []string{permissionRuntimeManage}},
	{prefix: "/api/v1/runtime", permissions: []string{permissionRuntimeManage}},
	{prefix: "/api/v1/files", permissions: []string{permissionFilesManage}},
	{prefix: "/api/v1/terminal", permissions: []string{permissionTerminalManage}},
	{prefix: "/api/v1/php", permissions: []string{permissionPHPManage}},
	{prefix: "/api/v1/ssl", permissions: []string{permissionSSLManage}},
	{prefix: "/api/v1/security", permissions: []string{permissionSecurityManage}},
	{prefix: "/api/v1/status", permissions: []string{permissionMonitoringRead}},
	{prefix: "/api/v1/monitor/logs", permissions: []string{permissionLogsRead}},
	{prefix: "/api/v1/monitor/cron", permissions: []string{permissionCronManage}},
	{prefix: "/api/v1/monitor", permissions: []string{permissionMonitoringRead}},
	{prefix: "/api/v1/cloudflare", permissions: []string{permissionCloudflareManage}},
	{prefix: "/api/v1/migration", permissions: []string{permissionMigrationManage}},
	{prefix: "/api/v1/cloudlinux", permissions: []string{permissionCloudlinuxManage}},
	{prefix: "/api/v1/minio", permissions: []string{permissionMinioManage}},
	{prefix: "/api/v1/activity", permissions: []string{permissionActivityRead}},
	{prefix: "/api/v1/ops", permissions: []string{permissionOpsManage}},
	{prefix: "/api/v1/federated", permissions: []string{permissionOpsManage}},
	{prefix: "/api/v1/panel", permissions: []string{permissionPanelManage}},
	{prefix: "/api/v1/update", permissions: []string{permissionPanelManage}},
	{prefix: "/api/v1/system", permissions: []string{permissionPanelManage}},
	{prefix: "/api/v1/ai", permissions: []string{permissionAIManage}},
	{prefix: "/phpmyadmin", permissions: []string{permissionDatabasesManage}},
	{prefix: "/pgadmin4", permissions: []string{permissionDatabasesManage}},
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case roleAdmin:
		return roleAdmin
	case roleReseller:
		return roleReseller
	default:
		return roleUser
	}
}

func pathMatchesPrefix(path, prefix string) bool {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return false
	}
	normalizedPrefix := strings.TrimSuffix(prefix, "/")
	return path == normalizedPrefix || strings.HasPrefix(path, normalizedPrefix+"/")
}

func isRestrictedNonAdminPath(path string) bool {
	return pathMatchesPrefix(path, "/api/v1/ai") ||
		pathMatchesPrefix(path, "/api/v1/security/ssh-keys") ||
		pathMatchesPrefix(path, "/api/v1/websites/custom-ssl") ||
		pathMatchesPrefix(path, "/api/v1/websites/vhost-config")
}

func resellerAllowed(path string) bool {
	if isRestrictedNonAdminPath(path) {
		return false
	}

	allowedPrefixes := []string{
		"/api/v1/auth/me",
		"/api/v1/vhost",
		"/api/v1/websites",
		"/api/v1/dns",
		"/api/v1/db",
		"/api/v1/mail",
		"/api/v1/ftp",
		"/api/v1/sftp",
		"/api/v1/backup",
		"/api/v1/apps",
		"/api/v1/wordpress",
		"/api/v1/files",
		"/api/v1/php",
		"/api/v1/ssl",
		"/api/v1/monitor/cron",
		"/api/v1/monitor/logs/site",
		"/api/v1/security/status",
		"/api/v1/security/firewall",
		"/api/v1/security/2fa",
		"/api/v1/security/immutable/status",
		"/api/v1/security/ebpf/events",
		"/api/v1/security/malware",
		"/api/v1/status/metrics",
		"/api/v1/status/services",
		"/api/v1/status/processes",
		"/api/v1/status/update",
		"/api/v1/analytics/website-traffic",
	}

	for _, prefix := range allowedPrefixes {
		if pathMatchesPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func userAllowed(method, path string) bool {
	if isRestrictedNonAdminPath(path) {
		return false
	}

	if path == "/api/v1/auth/me" || path == "/api/v1/status/metrics" || path == "/api/v1/status/services" || path == "/api/v1/status/update" {
		return true
	}

	if pathMatchesPrefix(path, "/api/v1/files") {
		return true
	}

	// Personal security actions.
	if pathMatchesPrefix(path, "/api/v1/security/2fa") ||
		pathMatchesPrefix(path, "/api/v1/security/status") ||
		pathMatchesPrefix(path, "/api/v1/security/immutable/status") ||
		pathMatchesPrefix(path, "/api/v1/security/ebpf/events") {
		return true
	}

	if method == http.MethodGet || method == http.MethodHead {
		return pathMatchesPrefix(path, "/api/v1/vhost/list") ||
			pathMatchesPrefix(path, "/api/v1/websites/aliases") ||
			pathMatchesPrefix(path, "/api/v1/websites/advanced-config") ||
			pathMatchesPrefix(path, "/api/v1/monitor/logs/site") ||
			pathMatchesPrefix(path, "/api/v1/analytics/website-traffic")
	}

	return false
}

func isAuthorized(role, method, path string) bool {
	return isAuthorizedWithPermissions(role, nil, method, path)
}

func normalizePermissionSet(permissions []string) map[string]struct{} {
	set := make(map[string]struct{}, len(permissions))
	for _, item := range permissions {
		key := strings.TrimSpace(item)
		if key == "" {
			continue
		}
		set[key] = struct{}{}
	}
	return set
}

func requiredPermissionsForPath(path string) ([]string, bool) {
	if path == "/api/v1/auth/me" || path == "/api/v1/auth/logout" {
		return []string{}, true
	}
	for _, rule := range permissionRules {
		if pathMatchesPrefix(path, rule.prefix) {
			return rule.permissions, true
		}
	}
	return nil, false
}

func isAuthorizedWithPermissions(role string, permissions []string, method, path string) bool {
	switch normalizeRole(role) {
	case roleAdmin:
		return true
	}

	if len(permissions) > 0 {
		required, mapped := requiredPermissionsForPath(path)
		if !mapped {
			return false
		}
		if len(required) == 0 {
			return true
		}
		permissionSet := normalizePermissionSet(permissions)
		if _, ok := permissionSet["*"]; ok {
			return true
		}
		for _, requiredPermission := range required {
			if _, ok := permissionSet[requiredPermission]; ok {
				return true
			}
		}
		return false
	}

	switch normalizeRole(role) {
	case roleReseller:
		return resellerAllowed(path)
	case roleUser:
		return userAllowed(method, path)
	}
	return false
}

// RBACMiddleware enforces endpoint-level role permissions after authentication.
func RBACMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetAuthUser(r.Context())
		if !ok {
			WriteError(w, r, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "Unauthorized")
			return
		}

		if !isAuthorizedWithPermissions(user.Role, user.Permissions, r.Method, r.URL.Path) {
			WriteError(w, r, http.StatusForbidden, "AUTH_FORBIDDEN", "Role is not allowed for this endpoint")
			return
		}

		next.ServeHTTP(w, r)
	})
}
