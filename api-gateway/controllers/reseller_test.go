package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResellerListVhostsForwardsQueryToService(t *testing.T) {
	var gotPath string
	var gotMethod string
	var gotProxyToken string

	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		gotMethod = r.Method
		gotProxyToken = r.Header.Get("X-Aura-Proxy-Token")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   []interface{}{},
		})
	}))
	defer service.Close()

	t.Setenv("AURAPANEL_SERVICE_URL", service.URL)
	t.Setenv("AURAPANEL_INTERNAL_PROXY_TOKEN", "internal-token")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reseller/vhost/list?search=alice&page=2&per_page=999", nil)
	rec := httptest.NewRecorder()

	ResellerListVhosts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("expected method GET, got %s", gotMethod)
	}
	if gotPath != "/api/v1/vhost/list?page=2&per_page=200&search=alice" {
		t.Fatalf("unexpected forwarded path: %s", gotPath)
	}
	if gotProxyToken != "internal-token" {
		t.Fatalf("expected proxy token to be forwarded")
	}
}
