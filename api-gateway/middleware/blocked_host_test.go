package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestBlockedHostMiddlewareBlocksConfiguredHost(t *testing.T) {
	t.Setenv("AURAPANEL_BLOCKED_HOSTS", "demo.aurapanel.com")
	blockedHostsInit = sync.Once{}
	cachedBlockedHost = nil

	nextCalled := false
	handler := BlockedHostMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "demo.aurapanel.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for blocked host, got %d", rec.Code)
	}
	if nextCalled {
		t.Fatalf("next handler should not be called for blocked host")
	}
}

func TestBlockedHostMiddlewareAllowsOtherHost(t *testing.T) {
	t.Setenv("AURAPANEL_BLOCKED_HOSTS", "demo.aurapanel.com")
	blockedHostsInit = sync.Once{}
	cachedBlockedHost = nil

	nextCalled := false
	handler := BlockedHostMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "panel.aurapanel.info"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for allowed host, got %d", rec.Code)
	}
	if !nextCalled {
		t.Fatalf("next handler should be called for allowed host")
	}
}
