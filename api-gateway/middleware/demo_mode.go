package middleware

import (
	"net/http"
	"os"
	"strings"
)

const demoModeErrorCode = "DEMO_READ_ONLY"

var defaultDemoAllowedPOSTPaths = map[string]struct{}{
	"/api/v1/auth/logout":            {},
	"/api/v1/files/list":             {},
	"/api/v1/files/read":             {},
	"/api/v1/php/ini/get":            {},
	"/api/v1/ssl/details":            {},
	"/api/v1/cloudflare/zones":       {},
	"/api/v1/cloudflare/server-auth": {},
	"/api/v1/cloudflare/dns/list":    {},
	"/api/v1/cloudflare/analytics":   {},
	"/api/v1/wordpress/scan":         {},
}

func demoModeEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("AURAPANEL_DEMO_MODE")))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func demoAccountEmail() string {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("AURAPANEL_DEMO_EMAIL")))
	if value == "" {
		return "demo@aurapanel.info"
	}
	return value
}

func demoAccountEmails() map[string]struct{} {
	emails := map[string]struct{}{}

	raw := strings.TrimSpace(os.Getenv("AURAPANEL_DEMO_EMAILS"))
	if raw != "" {
		for _, item := range strings.Split(raw, ",") {
			email := strings.ToLower(strings.TrimSpace(item))
			if email == "" {
				continue
			}
			emails[email] = struct{}{}
		}
	}

	if len(emails) == 0 {
		emails[demoAccountEmail()] = struct{}{}
	}

	return emails
}

func isDemoAccount(user AuthUser) bool {
	email := strings.ToLower(strings.TrimSpace(user.Email))
	if email == "" {
		return false
	}
	_, ok := demoAccountEmails()[email]
	return ok
}

func isDemoDBToolPath(path string) bool {
	return pathMatchesPrefix(path, "/phpmyadmin") || pathMatchesPrefix(path, "/pgadmin4")
}

func demoAllowedPOSTPaths() map[string]struct{} {
	allowed := make(map[string]struct{}, len(defaultDemoAllowedPOSTPaths))
	for path := range defaultDemoAllowedPOSTPaths {
		allowed[path] = struct{}{}
	}

	custom := strings.TrimSpace(os.Getenv("AURAPANEL_DEMO_POST_ALLOWLIST"))
	if custom == "" {
		return allowed
	}
	for _, item := range strings.Split(custom, ",") {
		path := strings.TrimSpace(item)
		if path == "" {
			continue
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		allowed[path] = struct{}{}
	}
	return allowed
}

func isDemoAllowedPOST(path string) bool {
	_, ok := demoAllowedPOSTPaths()[path]
	return ok
}

func denyDemoWrite(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, http.StatusForbidden, demoModeErrorCode, "Demo account is read-only. Write operations are disabled.")
}

// DemoModeMiddleware keeps demo users in strict read-only mode.
func DemoModeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !demoModeEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		user, ok := GetAuthUser(r.Context())
		if !ok {
			WriteError(w, r, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "Unauthorized")
			return
		}
		if !isDemoAccount(user) {
			next.ServeHTTP(w, r)
			return
		}

		path := strings.TrimSpace(r.URL.Path)
		if isDemoDBToolPath(path) {
			denyDemoWrite(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		case http.MethodPost:
			if isDemoAllowedPOST(path) {
				next.ServeHTTP(w, r)
				return
			}
			denyDemoWrite(w, r)
			return
		default:
			denyDemoWrite(w, r)
			return
		}
	})
}
