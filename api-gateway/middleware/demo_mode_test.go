package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withDemoAuthUser(req *http.Request, email, role string) *http.Request {
	ctx := context.WithValue(req.Context(), authUserContextKey, AuthUser{
		Email: email,
		Name:  "Demo",
		Role:  role,
	})
	return req.WithContext(ctx)
}

func TestDemoModeMiddlewareAllowsReadRequestsForDemoUser(t *testing.T) {
	t.Setenv("AURAPANEL_DEMO_MODE", "true")
	t.Setenv("AURAPANEL_DEMO_EMAIL", "demo@aurapanel.info")

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := DemoModeMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/websites", nil)
	req = withDemoAuthUser(req, "demo@aurapanel.info", roleAdmin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestDemoModeMiddlewareBlocksMutationForDemoUser(t *testing.T) {
	t.Setenv("AURAPANEL_DEMO_MODE", "true")
	t.Setenv("AURAPANEL_DEMO_EMAIL", "demo@aurapanel.info")

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := DemoModeMiddleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/vhost", nil)
	req = withDemoAuthUser(req, "demo@aurapanel.info", roleAdmin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestDemoModeMiddlewareAllowsConfiguredReadOnlyPostEndpoints(t *testing.T) {
	t.Setenv("AURAPANEL_DEMO_MODE", "true")
	t.Setenv("AURAPANEL_DEMO_EMAIL", "demo@aurapanel.info")

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := DemoModeMiddleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/list", nil)
	req = withDemoAuthUser(req, "demo@aurapanel.info", roleAdmin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestDemoModeMiddlewareBlocksDBToolsForDemoUser(t *testing.T) {
	t.Setenv("AURAPANEL_DEMO_MODE", "true")
	t.Setenv("AURAPANEL_DEMO_EMAIL", "demo@aurapanel.info")

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := DemoModeMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/phpmyadmin/index.php", nil)
	req = withDemoAuthUser(req, "demo@aurapanel.info", roleAdmin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestDemoModeMiddlewareAllowsCustomAllowlistPOSTPath(t *testing.T) {
	t.Setenv("AURAPANEL_DEMO_MODE", "true")
	t.Setenv("AURAPANEL_DEMO_EMAIL", "demo@aurapanel.info")
	t.Setenv("AURAPANEL_DEMO_POST_ALLOWLIST", "/api/v1/custom/read")

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := DemoModeMiddleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/custom/read", nil)
	req = withDemoAuthUser(req, "demo@aurapanel.info", roleAdmin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestDemoModeMiddlewareDoesNotRestrictNonDemoUsers(t *testing.T) {
	t.Setenv("AURAPANEL_DEMO_MODE", "true")
	t.Setenv("AURAPANEL_DEMO_EMAIL", "demo@aurapanel.info")

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := DemoModeMiddleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/vhost", nil)
	req = withDemoAuthUser(req, "admin@aurapanel.info", roleAdmin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestDemoModeMiddlewareSkipsChecksWhenDisabled(t *testing.T) {
	t.Setenv("AURAPANEL_DEMO_MODE", "false")
	t.Setenv("AURAPANEL_DEMO_EMAIL", "demo@aurapanel.info")

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := DemoModeMiddleware(next)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/vhost", nil)
	req = withDemoAuthUser(req, "demo@aurapanel.info", roleAdmin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestDemoModeMiddlewareSupportsMultipleDemoAccounts(t *testing.T) {
	t.Setenv("AURAPANEL_DEMO_MODE", "true")
	t.Setenv("AURAPANEL_DEMO_EMAILS", "demo@aurapanel.info, demo.reseller@aurapanel.info, demo.user@aurapanel.info")

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := DemoModeMiddleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/vhost", nil)
	req = withDemoAuthUser(req, "demo.reseller@aurapanel.info", roleReseller)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for listed demo account, got %d", rec.Code)
	}
}
