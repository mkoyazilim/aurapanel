package controllers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func panelDistPath() string {
	raw := strings.TrimSpace(os.Getenv("AURAPANEL_PANEL_DIST"))
	if raw == "" {
		return "/opt/aurapanel/frontend/dist"
	}
	return raw
}

func isAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/")
}

// PanelStaticHandler serves compiled frontend assets and falls back to index.html for SPA routes.
func PanelStaticHandler() http.Handler {
	dist := panelDistPath()
	fileServer := http.FileServer(http.Dir(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPIPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}

		cleanPath := filepath.Clean(r.URL.Path)
		if cleanPath == "." || cleanPath == "/" {
			http.ServeFile(w, r, filepath.Join(dist, "index.html"))
			return
		}

		target := filepath.Join(dist, strings.TrimPrefix(cleanPath, "/"))
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, filepath.Join(dist, "index.html"))
	})
}
