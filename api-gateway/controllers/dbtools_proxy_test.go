package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aurapanel/api-gateway/middleware"
	"github.com/golang-jwt/jwt/v5"
)

func TestDBToolsProxyRequiresAndForwardsCookieAuth(t *testing.T) {
	var gotPath string
	var gotAuthEmail string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuthEmail = r.Header.Get("X-Aura-Auth-Email")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	t.Setenv("AURAPANEL_DBTOOLS_UPSTREAM_URL", upstream.URL)
	t.Setenv("AURAPANEL_INTERNAL_PROXY_TOKEN", "internal-token")
	t.Setenv("AURAPANEL_JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("AURAPANEL_JWT_ISSUER", "aurapanel-gateway")
	t.Setenv("AURAPANEL_JWT_AUDIENCE", "aurapanel-ui")
	t.Setenv("AURAPANEL_AUTH_COOKIE_NAME", "aurapanel_session")

	proxy, err := NewDBToolsProxy()
	if err != nil {
		t.Fatalf("failed to init db tools proxy: %v", err)
	}
	protected := middleware.AuthMiddleware(proxy)

	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email":    "admin@server.com",
		"name":     "Admin",
		"role":     "admin",
		"username": "admin",
		"iss":      middleware.JwtIssuer(),
		"aud":      middleware.JwtAudience(),
		"iat":      now.Unix(),
		"nbf":      now.Add(-1 * time.Minute).Unix(),
		"exp":      now.Add(1 * time.Hour).Unix(),
	})
	tokenValue, err := token.SignedString([]byte(middleware.JwtSecret()))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/phpmyadmin/index.php", nil)
	req.AddCookie(&http.Cookie{Name: "aurapanel_session", Value: tokenValue})
	rec := httptest.NewRecorder()

	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 from proxied request, got %d", rec.Code)
	}
	if gotPath != "/phpmyadmin/index.php" {
		t.Fatalf("expected upstream path /phpmyadmin/index.php, got %s", gotPath)
	}
	if gotAuthEmail != "admin@server.com" {
		t.Fatalf("expected forwarded auth email, got %q", gotAuthEmail)
	}
}
