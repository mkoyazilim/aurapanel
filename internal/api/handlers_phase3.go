package api

import (
	"net/http"
)

func (s *Server) handlePhase3Servers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	servers, err := s.deps.Store.ListServers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, servers)
}


