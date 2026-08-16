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

func (s *Server) handleMailStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	status, err := s.deps.Mail.GetMailStatus(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleMailSetup(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	if err := s.deps.Mail.SetupMailServer(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMailDKIM(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	var req struct {
		Domain string `json:"domain"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	record, err := s.deps.Mail.GenerateDKIM(r.Context(), req.Domain)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"dkim_record": record})
}

func (s *Server) handleMailDKIMGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	domain := r.URL.Query().Get("domain")
	if domain == "" {
		writeErr(w, http.StatusBadRequest, "domain query parameter required")
		return
	}

	record, err := s.deps.Mail.GetDKIMRecord(r.Context(), domain)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"dkim_record": record})
}

func (s *Server) handleMailPasswordChange(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	email := r.PathValue("email")

	var req struct {
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	if err := s.deps.Mail.ChangePassword(r.Context(), email, req.Password); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
