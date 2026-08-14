package api

import (
	"net/http"
)

func (s *Server) handleMailList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")

	accounts, err := s.deps.Store.ListMailAccounts(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, accounts)
}

func (s *Server) handleMailCreate(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")

	var req struct {
		Domain    string `json:"domain"`
		LocalPart string `json:"local_part"`
		Password  string `json:"password"`
		QuotaMB   int    `json:"quota_mb"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	if err := s.deps.Mail.CreateAccount(r.Context(), siteID, req.Domain, req.LocalPart, req.Password, req.QuotaMB); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMailDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	email := r.PathValue("email")

	if err := s.deps.Mail.DeleteAccount(r.Context(), email); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
