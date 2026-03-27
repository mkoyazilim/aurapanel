package controllers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPanelStaticHandlerServesIndexAndAssets(t *testing.T) {
	tmp := t.TempDir()
	index := []byte("<html><body>panel</body></html>")
	js := []byte("console.log('ok')")

	if err := os.WriteFile(filepath.Join(tmp, "index.html"), index, 0o644); err != nil {
		t.Fatalf("failed to write index: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "assets"), 0o755); err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "assets", "app.js"), js, 0o644); err != nil {
		t.Fatalf("failed to write js: %v", err)
	}

	t.Setenv("AURAPANEL_PANEL_DIST", tmp)
	handler := PanelStaticHandler()

	recRoot := httptest.NewRecorder()
	handler.ServeHTTP(recRoot, httptest.NewRequest(http.MethodGet, "/", nil))
	if recRoot.Code != http.StatusOK {
		t.Fatalf("expected 200 for /, got %d", recRoot.Code)
	}
	if got := recRoot.Header().Get("Cache-Control"); got == "" {
		t.Fatalf("expected Cache-Control on index response")
	}

	recAsset := httptest.NewRecorder()
	handler.ServeHTTP(recAsset, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if recAsset.Code != http.StatusOK {
		t.Fatalf("expected 200 for asset, got %d", recAsset.Code)
	}

	recSpa := httptest.NewRecorder()
	handler.ServeHTTP(recSpa, httptest.NewRequest(http.MethodGet, "/websites/example.com", nil))
	if recSpa.Code != http.StatusOK {
		t.Fatalf("expected 200 for SPA fallback, got %d", recSpa.Code)
	}
	if got := recSpa.Header().Get("Cache-Control"); got == "" {
		t.Fatalf("expected Cache-Control on SPA fallback response")
	}
}

func TestPanelStaticHandlerRejectsAPIPath(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("failed to write index: %v", err)
	}
	t.Setenv("AURAPANEL_PANEL_DIST", tmp)
	handler := PanelStaticHandler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for /api/* path, got %d", rec.Code)
	}
}
