package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestRequireSecurityConfigFailsWithoutSecret(t *testing.T) {
	t.Setenv("AURAPANEL_DEV_SIMULATION", "")
	t.Setenv("AURAPANEL_JWT_SECRET", "")
	t.Setenv("AURAPANEL_INTERNAL_PROXY_TOKEN", "")

	if err := RequireSecurityConfig(); err == nil {
		t.Fatalf("expected RequireSecurityConfig to fail without JWT secret")
	}
}

func TestRequireSecurityConfigFailsWithoutInternalProxyToken(t *testing.T) {
	t.Setenv("AURAPANEL_DEV_SIMULATION", "")
	t.Setenv("AURAPANEL_JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("AURAPANEL_INTERNAL_PROXY_TOKEN", "")

	if err := RequireSecurityConfig(); err == nil {
		t.Fatalf("expected RequireSecurityConfig to fail without internal proxy token")
	}
}

func TestAuthMiddlewareRejectsMissingToken(t *testing.T) {
	t.Setenv("AURAPANEL_JWT_SECRET", "0123456789abcdef0123456789abcdef")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/websites", nil)
	rec := httptest.NewRecorder()

	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing token, got %d", rec.Code)
	}
}

func TestAuthMiddlewareAcceptsValidToken(t *testing.T) {
	t.Setenv("AURAPANEL_JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("AURAPANEL_JWT_ISSUER", "aurapanel-gateway")
	t.Setenv("AURAPANEL_JWT_AUDIENCE", "aurapanel-ui")

	now := time.Now().UTC()
	claims := gatewayClaims{
		Email: "admin@server.com",
		Name:  "Admin",
		Role:  "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    JwtIssuer(),
			Audience:  jwt.ClaimStrings{JwtAudience()},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenValue, err := token.SignedString([]byte(JwtSecret()))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/websites", nil)
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	rec := httptest.NewRecorder()

	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := GetAuthUser(r.Context()); !ok {
			t.Fatalf("expected auth user in context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for valid token, got %d", rec.Code)
	}
}

func TestAuthMiddlewareAcceptsCookieToken(t *testing.T) {
	t.Setenv("AURAPANEL_JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("AURAPANEL_JWT_ISSUER", "aurapanel-gateway")
	t.Setenv("AURAPANEL_JWT_AUDIENCE", "aurapanel-ui")
	t.Setenv("AURAPANEL_AUTH_COOKIE_NAME", "aurapanel_session")

	now := time.Now().UTC()
	claims := gatewayClaims{
		Email: "admin@server.com",
		Name:  "Admin",
		Role:  "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    JwtIssuer(),
			Audience:  jwt.ClaimStrings{JwtAudience()},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenValue, err := token.SignedString([]byte(JwtSecret()))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/phpmyadmin/index.php", nil)
	req.AddCookie(&http.Cookie{
		Name:  "aurapanel_session",
		Value: tokenValue,
	})
	rec := httptest.NewRecorder()

	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := GetAuthUser(r.Context()); !ok {
			t.Fatalf("expected auth user in context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for valid cookie token, got %d", rec.Code)
	}
}
