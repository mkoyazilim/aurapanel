package api

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/auth"
	"github.com/mkoyazilim/aurapanel/internal/fm"
	"github.com/mkoyazilim/aurapanel/internal/site"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// --- Health/Status ---

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	sites, err := s.deps.Store.ListSites(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	drifts, _ := s.deps.Store.ListOpenDriftEvents(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"db":          "ok",
		"site_count":  len(sites),
		"open_drifts": len(drifts),
		"time":        time.Now().UTC().Format(time.RFC3339),
	})
}

// --- Auth ---

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		TOTP     string `json:"totp"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	ip := clientIP(r)
	key := ip + "|" + req.Username
	if !s.throttle.Check(key) {
		writeErr(w, http.StatusTooManyRequests, "çok fazla başarısız deneme; bekleyin")
		return
	}

	u, err := s.deps.Store.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok := false
	if u != nil {
		ok, _ = auth.VerifyPassword(req.Password, u.PasswordHash)
	}
	if u == nil || !ok {
		s.throttle.Record(key)
		s.deps.Audit.Write(r.Context(), audit.Event{Action: "auth.login", IP: ip, Result: "failed"})
		writeErr(w, http.StatusUnauthorized, "kullanıcı adı veya şifre hatalı")
		return
	}

	// TOTP zorunluysa doğrula.
	if u.TOTPEnabled {
		secret, err := s.deps.Cipher.Decrypt(u.TOTPSecretEnc.String)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "TOTP sırrı çözümlenemedi")
			return
		}
		totpOK, err := auth.VerifyTOTP(string(secret), req.TOTP)
		if err != nil || !totpOK {
			s.throttle.Record(key)
			s.deps.Audit.Write(r.Context(), audit.Event{Action: "auth.login", IP: ip, Result: "failed",
				Extra: map[string]any{"reason": "totp"}})
			writeErr(w, http.StatusUnauthorized, "TOTP kodu hatalı")
			return
		}
	}

	s.throttle.Reset(key)
	sessionID, csrf, err := s.deps.Sessions.Create(r.Context(), u.ID, ip, r.UserAgent())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: sessionID, Path: "/",
		HttpOnly: true, Secure: false, SameSite: http.SameSiteStrictMode,
		Expires: time.Now().Add(12 * time.Hour),
	})
	s.deps.Store.SetUserLastLogin(r.Context(), u.ID)
	s.deps.Audit.Write(r.Context(), audit.Event{
		Action: "auth.login", Target: u.Username, IP: ip, Result: "success",
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"username":             u.Username,
		"must_change_password": u.MustChangePassword,
		"csrf_token":           csrf,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if sid, ok := r.Context().Value(ctxSessionID).(string); ok && sid != "" {
		s.deps.Sessions.Destroy(r.Context(), sid)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, _ := userFromCtx(r.Context())
	role, _ := s.deps.Store.GetRoleName(r.Context(), u.RoleID)
	writeJSON(w, http.StatusOK, map[string]any{
		"id": u.ID, "username": u.Username, "role": role,
		"must_change_password": u.MustChangePassword, "totp_enabled": u.TOTPEnabled,
	})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	u, ok := userFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "kimlik doğrulaması yok")
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if len(req.NewPassword) < 12 || len(req.NewPassword) > 128 {
		writeErr(w, http.StatusBadRequest, "yeni şifre 12..128 karakter olmalı")
		return
	}
	okOld, _ := auth.VerifyPassword(req.OldPassword, u.PasswordHash)
	if !okOld {
		writeErr(w, http.StatusUnauthorized, "mevcut şifre hatalı")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.deps.Store.UpdateUserPassword(r.Context(), u.ID, hash); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{Action: "auth.password_change", Target: u.Username, Result: "success"})

	// OLS WebAdmin tek giriş çifti (ARCHITECTURE §9.10): şifre değişince
	// WebAdmin'e helper üzerinden senkronla. Panel şifresi otoritedir —
	// senkron hatası panel değişikliğini GERİ ALMAZ (kilitlenme riski),
	// audit'e "failed" olarak işlenir.
	if s.deps.Priv != nil {
		if _, err := s.deps.Priv.Call(r.Context(), "ols.webadmin_credentials", map[string]any{
			"username": u.Username,
			"password": req.NewPassword,
		}); err != nil {
			s.deps.Log.Warn("OLS WebAdmin senkronu başarısız", "error", err)
			s.deps.Audit.Write(r.Context(), audit.Event{Action: "ols.webadmin_sync",
				Target: u.Username, Result: "failed", Extra: map[string]any{"error": err.Error()}})
		} else {
			s.deps.Audit.Write(r.Context(), audit.Event{Action: "ols.webadmin_sync",
				Target: u.Username, Result: "success"})
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMFAStart(w http.ResponseWriter, r *http.Request) {
	u, ok := userFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "kimlik doğrulaması yok")
		return
	}
	secret, url, err := auth.GenerateTOTP("AuraPanel", u.Username)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Sır henüz KAYDEDİLMEZ — enable aşamasında doğrulanınca kaydedilir.
	writeJSON(w, http.StatusOK, map[string]any{"secret": secret, "otpauth_url": url})
}

func (s *Server) handleMFAEnable(w http.ResponseWriter, r *http.Request) {
	u, ok := userFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "kimlik doğrulaması yok")
		return
	}
	var req struct {
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	totpOK, err := auth.VerifyTOTP(req.Secret, req.Code)
	if err != nil || !totpOK {
		writeErr(w, http.StatusBadRequest, "TOTP kodu hatalı")
		return
	}
	enc, err := s.deps.Cipher.Encrypt([]byte(req.Secret))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.deps.Store.SetUserTOTPSecret(r.Context(), u.ID, enc); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{Action: "auth.mfa.enable", Target: u.Username, Result: "success"})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMFADisable(w http.ResponseWriter, r *http.Request) {
	u, ok := userFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "kimlik doğrulaması yok")
		return
	}
	if err := s.deps.Store.SetUserTOTPSecret(r.Context(), u.ID, ""); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{Action: "auth.mfa.disable", Target: u.Username, Result: "success"})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handlePATCreate(w http.ResponseWriter, r *http.Request) {
	u, ok := userFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "kimlik doğrulaması yok")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Name == "" {
		req.Name = "token"
	}
	token := auth.NewPAT()
	if _, err := s.deps.Store.InsertPATToken(r.Context(), store.PATToken{
		UserID: u.ID, Name: req.Name, TokenHash: auth.HashPAT(token),
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.deps.Audit.Write(r.Context(), audit.Event{Action: "auth.pat.create", Target: u.Username, Result: "success"})
	// Ham token yalnızca BİR KEZ döner.
	writeJSON(w, http.StatusCreated, map[string]string{"token": token, "name": req.Name})
}

func (s *Server) handlePATList(w http.ResponseWriter, r *http.Request) {
	u, ok := userFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "kimlik doğrulaması yok")
		return
	}
	tokens, err := s.deps.Store.ListPATTokensByUser(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := []map[string]any{}
	for _, t := range tokens {
		out = append(out, map[string]any{
			"id": t.ID, "name": t.Name, "created_at": t.CreatedAt,
			"expires_at": t.ExpiresAt, "last_used_at": t.LastUsedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePATDelete(w http.ResponseWriter, r *http.Request) {
	u, ok := userFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "kimlik doğrulaması yok")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "geçersiz id")
		return
	}
	tokens, _ := s.deps.Store.ListPATTokensByUser(r.Context(), u.ID)
	found := false
	for _, t := range tokens {
		if t.ID == id {
			found = true
			break
		}
	}
	if !found {
		writeErr(w, http.StatusNotFound, "token yok")
		return
	}
	if err := s.deps.Store.DeletePATToken(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Siteler ---

func (s *Server) handleSitesList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	sites, err := s.deps.Sites.ListSites(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sites)
}

func (s *Server) handleSiteCreate(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		Domain     string      `json:"domain"`
		Aliases    []string    `json:"aliases"`
		PHPVersion string      `json:"php_version"`
		Limits     site.Limits `json:"limits"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.PHPVersion == "" {
		req.PHPVersion = "8.3"
	}
	id, err := s.deps.Sites.Create(r.Context(), site.CreateRequest{
		Domain: req.Domain, Aliases: req.Aliases, PHPVersion: req.PHPVersion, Limits: req.Limits,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"site_id": id})
}

func (s *Server) handleSiteDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if err := s.deps.Sites.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSiteLimits(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req site.Limits
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.Sites.UpdateLimits(r.Context(), r.PathValue("id"), req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSiteFeatures(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req map[string]bool
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.Sites.SetFeatureFlags(r.Context(), r.PathValue("id"), req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Dosyalar ---

func (s *Server) handleFilesList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	entries, err := s.deps.Files.List(r.Context(), r.PathValue("id"), r.URL.Query().Get("path"))
	if err != nil {
		writeFileErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleFileRead(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	content, hash, mtime, err := s.deps.Files.Read(r.Context(), r.PathValue("id"), r.URL.Query().Get("path"))
	if err != nil {
		writeFileErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"content_b64": base64.StdEncoding.EncodeToString(content),
		"sha256":      hash, "modified_at": mtime,
	})
}

func (s *Server) handleFileWrite(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		ContentB64    string `json:"content_b64"`
		ExpectedHash  string `json:"expected_hash"`
		ExpectedMTime string `json:"expected_mtime"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	content, err := base64.StdEncoding.DecodeString(req.ContentB64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "content_b64 geçersiz")
		return
	}
	err = s.deps.Files.Write(r.Context(), r.PathValue("id"), r.URL.Query().Get("path"),
		content, req.ExpectedHash, req.ExpectedMTime)
	if err != nil {
		writeFileErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleFileMkdir(w http.ResponseWriter, r *http.Request) {
	s.fileSimple(w, r, func(ctxID, path string) error {
		return s.deps.Files.Mkdir(r.Context(), ctxID, path)
	}, "path")
}

func (s *Server) handleFileRename(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.Files.Rename(r.Context(), r.PathValue("id"), req.From, req.To); err != nil {
		writeFileErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleFileCopy(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.Files.Copy(r.Context(), r.PathValue("id"), req.From, req.To); err != nil {
		writeFileErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	// Trash bağlıysa çöpe, değilse doğrudan sil (W9.2 notu).
	if s.deps.Trash != nil {
		if _, err := s.deps.Trash.MoveToTrash(r.Context(), r.PathValue("id"), req.Path); err != nil {
			writeFileErr(w, err)
			return
		}
	} else if err := s.deps.Files.Remove(r.Context(), r.PathValue("id"), req.Path); err != nil {
		writeFileErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleFileSymlink(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		Target string `json:"target"`
		Link   string `json:"link"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.Files.Symlink(r.Context(), r.PathValue("id"), req.Target, req.Link); err != nil {
		writeFileErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) fileSimple(w http.ResponseWriter, r *http.Request, fn func(siteID, path string) error, field string) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := fn(r.PathValue("id"), req.Path); err != nil {
		writeFileErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// writeFileErr, fm hatalarını HTTP kodlarına eşler (FILE_MANAGER §15).
func writeFileErr(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		return
	case err == fm.ErrOutsideRoot || err == fm.ErrInvalidPath:
		writeErr(w, http.StatusForbidden, err.Error())
	case err == fm.ErrConflict:
		writeErr(w, http.StatusConflict, err.Error())
	case err == fm.ErrRateLimited:
		writeErr(w, http.StatusTooManyRequests, err.Error())
	default:
		writeErr(w, http.StatusBadRequest, err.Error())
	}
}

// --- Upload ---

func (s *Server) handleUploadInit(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		SiteID    string `json:"site_id"`
		Dir       string `json:"dir"`
		FileName  string `json:"file_name"`
		TotalSize int64  `json:"total_size"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	id, err := s.deps.Uploads.Init(req.SiteID, req.Dir, req.FileName, req.TotalSize)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"upload_id": id})
}

func (s *Server) handleUploadChunk(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		UploadID string `json:"upload_id"`
		Index    int    `json:"index"`
		DataB64  string `json:"data_b64"`
		SHA256   string `json:"sha256"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.DataB64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "data_b64 geçersiz")
		return
	}
	n, err := s.deps.Uploads.Chunk(req.UploadID, req.Index, data, req.SHA256)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"bytes": n})
}

func (s *Server) handleUploadFinalize(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		UploadID string `json:"upload_id"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	target, err := s.deps.Uploads.Finalize(r.Context(), req.UploadID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"target": target})
}

func (s *Server) handleUploadAbort(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		UploadID string `json:"upload_id"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.Uploads.Abort(req.UploadID); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Arşiv ---

func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		Action  string   `json:"action"`
		Format  string   `json:"format"`
		Target  string   `json:"target"`
		Sources []string `json:"sources"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	siteID := r.PathValue("id")
	var err error
	switch req.Action {
	case "create":
		err = s.deps.Archive.Create(r.Context(), siteID, req.Target, req.Format, req.Sources)
	case "extract":
		parts := strings.SplitN(req.Target, "|", 2) // "archive|dest"
		if len(parts) != 2 {
			writeErr(w, http.StatusBadRequest, "target 'arşiv|hedef' biçiminde olmalı")
			return
		}
		err = s.deps.Archive.Extract(r.Context(), siteID, parts[0], parts[1], req.Format)
	default:
		writeErr(w, http.StatusBadRequest, "bilinmeyen işlem")
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleTrashEmpty(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.deps.Trash == nil {
		writeErr(w, http.StatusNotImplemented, "trash bağlı değil")
		return
	}
	if err := s.deps.Trash.Empty(r.Context(), r.PathValue("id")); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- PHP ---

func (s *Server) handlePHPVersions(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	vs, err := s.deps.PHP.ListVersions(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, vs)
}

func (s *Server) handlePHPSwitch(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		Version string `json:"version"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.PHP.SwitchVersion(r.Context(), r.PathValue("id"), req.Version); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Veritabanları ---

func (s *Server) handleDBList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	dbs, err := s.deps.DB.ListDatabases(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dbs)
}

func (s *Server) handleDBCreate(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.DB.CreateDatabase(r.Context(), r.PathValue("id"), req.Name); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (s *Server) handleDBDrop(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if err := s.deps.DB.DropDatabase(r.Context(), r.PathValue("id"), r.PathValue("name")); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDBUserList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	users, err := s.deps.DB.ListUsers(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleDBUserCreate(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	pw, err := s.deps.DB.CreateUser(r.Context(), r.PathValue("id"), req.Username, req.Password)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"password": pw})
}

func (s *Server) handleDBUserDrop(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if err := s.deps.DB.DropUser(r.Context(), r.PathValue("id"), r.PathValue("name")); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDBUserPassword(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	pw, err := s.deps.DB.ResetPassword(r.Context(), r.PathValue("id"), r.PathValue("name"), req.Password)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"password": pw})
}

func (s *Server) handleDBGrant(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		Username string `json:"username"`
		Database string `json:"database"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.DB.Grant(r.Context(), r.PathValue("id"), req.Username, req.Database); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminerOpen(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		SiteID     string `json:"site_id"`
		DatabaseID int64  `json:"database_id"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	token, err := s.deps.DB.OpenAdminer(r.Context(), req.SiteID, req.DatabaseID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// --- SSL ---

func (s *Server) handleSSLEnableLE(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if err := s.deps.SSL.EnableLetsEncrypt(r.Context(), r.PathValue("id")); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSSLCustom(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		CertPEM string `json:"cert_pem"`
		KeyPEM  string `json:"key_pem"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.SSL.InstallCustom(r.Context(), r.PathValue("id"), []byte(req.CertPEM), []byte(req.KeyPEM)); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSSLDisable(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if err := s.deps.SSL.DisableSSL(r.Context(), r.PathValue("id")); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Yedekler ---

func (s *Server) handleBackupRun(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		Kind string `json:"kind"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	name, err := s.deps.Backups.Run(r.Context(), r.PathValue("id"), req.Kind)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name})
}

func (s *Server) handleBackupList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	list, err := s.deps.Store.ListBackupsBySite(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// --- Drift ---

func (s *Server) handleDriftList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	events, err := s.deps.Store.ListOpenDriftEvents(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) handleDriftScan(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	n, err := s.deps.DriftScan.Scan(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"found": n})
}

func (s *Server) handleDriftRepair(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		SiteID string `json:"site_id"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.DriftFix.Repair(r.Context(), req.SiteID); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDriftAutoRepair(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.DriftScan.SetAutoRepair(r.Context(), req.Enabled); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Güncelleme merkezi (W14) ---

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.deps.Updates == nil {
		writeErr(w, http.StatusNotImplemented, "güncelleme servisi bağlı değil")
		return
	}
	out, err := s.deps.Updates.Check(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleUpdateSelf(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.deps.Updates == nil {
		writeErr(w, http.StatusNotImplemented, "güncelleme servisi bağlı değil")
		return
	}
	v, err := s.deps.Updates.SelfUpdate(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"version": v, "note": "binary değişti; yeniden başlatma önerilir"})
}
