package api

import (
	"net/http"

	"github.com/mkoyazilim/aurapanel/internal/store"
)

func (s *Server) handleCloudflareGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")

	settings, err := s.deps.Store.GetCloudflareSettings(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if settings == nil {
		settings = &store.CloudflareSettings{
			SiteID: siteID,
		}
	}

	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleCloudflareSave(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")

	var req store.CloudflareSettings
	if !decodeBody(w, r, &req) {
		return
	}
	req.SiteID = siteID

	if err := s.deps.Store.SaveCloudflareSettings(r.Context(), &req); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleCloudflarePurge(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")

	if err := s.deps.Cloudflare.PurgeCache(r.Context(), siteID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
