package api

import (

	"net/http"

	"github.com/mkoyazilim/aurapanel/internal/auth"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

func (s *Server) handleUsersList(w http.ResponseWriter, r *http.Request) {
	u, role, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	var users []store.User
	var err error

	if role == "admin" {
		users, err = s.deps.Store.ListUsers(r.Context(), 0) // all
	} else if role == "reseller" {
		users, err = s.deps.Store.ListUsers(r.Context(), u.ID) // only sub-users
	} else {
		writeErr(w, http.StatusForbidden, "erişim reddedildi")
		return
	}

	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Remove passwords from response
	for i := range users {
		users[i].PasswordHash = ""
	}

	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	u, role, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	if role == "user" {
		writeErr(w, http.StatusForbidden, "erişim reddedildi")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	// Reseller can only create users, admin can create resellers or users
	if role == "reseller" {
		req.Role = "user"
	} else if req.Role == "" {
		req.Role = "user"
	}

	roleID, err := s.deps.Store.GetRoleIDByName(r.Context(), req.Role)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "geçersiz rol")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "şifre şifrelenemedi")
		return
	}

	newUser := store.User{
		Username:     req.Username,
		PasswordHash: hash,
		RoleID:       roleID,
		Status:       "active",
	}

	if role == "reseller" {
		newUser.ParentID.Valid = true
		newUser.ParentID.Int64 = u.ID
	}

	if _, err := s.deps.Store.InsertUser(r.Context(), newUser); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (s *Server) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	_, role, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	if role == "user" {
		writeErr(w, http.StatusForbidden, "erişim reddedildi")
		return
	}

	// Delete implementation
	// Needs to check if target user belongs to the reseller
	writeJSON(w, http.StatusNotImplemented, map[string]string{"status": "not_implemented"})
}
