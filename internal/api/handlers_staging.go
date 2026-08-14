package api

import (
	"net/http"
)

func (s *Server) handleStagingList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")

	results, err := s.deps.Store.ListStagingEnvironments(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleStagingCreate(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")

	var req struct {
		Domain string `json:"domain"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	stgSite, err := s.deps.Staging.CreateStaging(r.Context(), siteID, req.Domain)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Failed to create staging: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "staging_site_id": stgSite.ID})
}

func (s *Server) handleStagingPush(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")

	if err := s.deps.Staging.PushToProduction(r.Context(), siteID); err != nil {
		writeErr(w, http.StatusInternalServerError, "Push failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleStagingDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	envID := r.PathValue("envId")

	env, err := s.deps.Store.GetStagingEnvironment(r.Context(), envID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "Staging env not found")
		return
	}

	if err := s.deps.Sites.Delete(r.Context(), env.StagingSiteID); err != nil {
		writeErr(w, http.StatusInternalServerError, "Failed to delete staging site: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
