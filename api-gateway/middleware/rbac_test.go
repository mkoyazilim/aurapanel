package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withAuthUser(req *http.Request, role string) *http.Request {
	ctx := context.WithValue(req.Context(), authUserContextKey, AuthUser{
		Email: "test@example.com",
		Name:  "Test",
		Role:  role,
	})
	return req.WithContext(ctx)
}

func TestRBACMiddlewareAllowsAdmin(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RBACMiddleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/create", nil)
	req = withAuthUser(req, roleAdmin)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRBACMiddlewareAllowsResellerOnUsersRoute(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RBACMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/list", nil)
	req = withAuthUser(req, roleReseller)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRBACMiddlewareAllowsUserFileOps(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RBACMiddleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/list", nil)
	req = withAuthUser(req, roleUser)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRBACMiddlewareBlocksUserSSHKeyEndpoints(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RBACMiddleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/ssh-keys", nil)
	req = withAuthUser(req, roleUser)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRBACMiddlewareBlocksResellerCustomSSLRead(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RBACMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/websites/custom-ssl", nil)
	req = withAuthUser(req, roleReseller)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRBACMiddlewareBlocksResellerAITools(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RBACMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/tools/status", nil)
	req = withAuthUser(req, roleReseller)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}
