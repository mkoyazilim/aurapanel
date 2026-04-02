package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDDoSGuardMiddlewareDisabledBypassesLimiter(t *testing.T) {
	t.Setenv("AURAPANEL_DDOS_ENABLED", "0")
	t.Setenv("AURAPANEL_DDOS_PROFILE", "off")

	calls := 0
	handler := DDoSGuardMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/status/metrics", nil)
	req1.RemoteAddr = "198.51.100.10:1234"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/status/metrics", nil)
	req2.RemoteAddr = "198.51.100.10:4321"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec1.Code != http.StatusNoContent || rec2.Code != http.StatusNoContent {
		t.Fatalf("expected disabled guard to pass through, got %d and %d", rec1.Code, rec2.Code)
	}
	if calls != 2 {
		t.Fatalf("expected downstream handler to be called twice, got %d", calls)
	}
}

func TestDDoSGuardMiddlewareGlobalLimit(t *testing.T) {
	t.Setenv("AURAPANEL_DDOS_ENABLED", "1")
	t.Setenv("AURAPANEL_DDOS_PROFILE", "standard")
	t.Setenv("AURAPANEL_DDOS_GLOBAL_RPS", "1")
	t.Setenv("AURAPANEL_DDOS_GLOBAL_BURST", "1")
	t.Setenv("AURAPANEL_DDOS_AUTH_RPS", "100")
	t.Setenv("AURAPANEL_DDOS_AUTH_BURST", "100")

	handler := DDoSGuardMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	lastCode := http.StatusNoContent
	for idx := 0; idx < 6; idx++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/status/services", nil)
		req.RemoteAddr = "198.51.100.20:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		lastCode = rec.Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("expected request burst to be rate-limited, got %d", lastCode)
	}
}

func TestDDoSGuardMiddlewareAuthLimit(t *testing.T) {
	t.Setenv("AURAPANEL_DDOS_ENABLED", "1")
	t.Setenv("AURAPANEL_DDOS_PROFILE", "standard")
	t.Setenv("AURAPANEL_DDOS_GLOBAL_RPS", "100")
	t.Setenv("AURAPANEL_DDOS_GLOBAL_BURST", "100")
	t.Setenv("AURAPANEL_DDOS_AUTH_RPS", "1")
	t.Setenv("AURAPANEL_DDOS_AUTH_BURST", "1")

	handler := DDoSGuardMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	lastCode := http.StatusNoContent
	for idx := 0; idx < 3; idx++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "198.51.100.30:4567"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		lastCode = rec.Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("expected auth burst to be rate-limited, got %d", lastCode)
	}
}
