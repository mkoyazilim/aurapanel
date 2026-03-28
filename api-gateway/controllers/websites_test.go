package controllers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListWebsitesForwardsToServiceContract(t *testing.T) {
	var gotPath string
	var gotAuth string

	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   []interface{}{},
		})
	}))
	defer service.Close()

	t.Setenv("AURAPANEL_SERVICE_URL", service.URL)
	req := httptest.NewRequest(http.MethodGet, "/api/websites", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	ListWebsites(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotPath != "/api/v1/vhost/list" {
		t.Fatalf("expected /api/v1/vhost/list, got %s", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("authorization header was not forwarded")
	}
}

func TestCreateWebsiteForwardsPayloadToServiceContract(t *testing.T) {
	var gotPath string
	var gotMethod string
	var gotBody string

	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "success",
		})
	}))
	defer service.Close()

	t.Setenv("AURAPANEL_SERVICE_URL", service.URL)

	payload := `{"domain":"example.com","owner":"admin","php_version":"8.3"}`
	req := httptest.NewRequest(http.MethodPost, "/api/websites", strings.NewReader(payload))
	rec := httptest.NewRecorder()

	CreateWebsite(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected method POST, got %s", gotMethod)
	}
	if gotPath != "/api/v1/vhost" {
		t.Fatalf("expected /api/v1/vhost, got %s", gotPath)
	}
	if gotBody != payload {
		t.Fatalf("payload mismatch: %s", gotBody)
	}
}
