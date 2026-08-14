// Package api, panelin REST API sunucusudur (ROADMAP W11).
// Varsayılan bağlama: 127.0.0.1 (config); public mod kurulumda bilinçli
// seçilir (ARCHITECTURE §9.4).
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/auth"
	"github.com/mkoyazilim/aurapanel/internal/backup"
	"github.com/mkoyazilim/aurapanel/internal/config"
	"github.com/mkoyazilim/aurapanel/internal/crypto"
	"github.com/mkoyazilim/aurapanel/internal/db"
	"github.com/mkoyazilim/aurapanel/internal/drift"
	"github.com/mkoyazilim/aurapanel/internal/fm"
	"github.com/mkoyazilim/aurapanel/internal/php"
	"github.com/mkoyazilim/aurapanel/internal/privclient"
	"github.com/mkoyazilim/aurapanel/internal/sftp"
	"github.com/mkoyazilim/aurapanel/internal/site"
	"github.com/mkoyazilim/aurapanel/internal/ssl"
	"github.com/mkoyazilim/aurapanel/internal/store"
	"github.com/mkoyazilim/aurapanel/internal/update"
)

// SiteManager, site yaşam döngüsü arayüzü (site.Manager bunu sağlar).
type SiteManager interface {
	Create(ctx context.Context, req site.CreateRequest) (string, error)
	Delete(ctx context.Context, siteID string) error
	UpdateLimits(ctx context.Context, siteID string, l site.Limits) error
	SetFeatureFlags(ctx context.Context, siteID string, flags map[string]bool) error
	ListSites(ctx context.Context) ([]store.Site, error)
}

// Deps, sunucunun bağımlılıkları (testlerde sahte uygulamalarla doldurulur).
type Deps struct {
	Store    *store.Store
	Audit    *audit.Service
	Sessions *auth.SessionStore
	Priv     *privclient.Client // priv helper (OLS WebAdmin senkronu vb.)
	Cipher   *crypto.Cipher
	Cfg      *config.Config
	Log      *slog.Logger
	Web      http.Handler // gömülü SPA (webui.Handler)

	Sites     SiteManager
	Files     *fm.FileService
	Uploads   *fm.UploadService
	Archive   *fm.ArchiveService
	Trash     *fm.TrashService
	PHP       *php.Service
	SFTP      *sftp.Service
	DB        *db.Service
	SSL       *ssl.Service
	Backups   *backup.Service
	DriftScan *drift.Scanner
	DriftFix  *drift.Repairer
	Updates   *update.Service
}

// Server, HTTP sunucusu.
type Server struct {
	deps     Deps
	mux      *http.ServeMux
	throttle *loginThrottle
}

// New, Server oluşturur ve rotaları kaydeder.
func New(deps Deps) *Server {
	s := &Server{deps: deps, mux: http.NewServeMux(), throttle: newLoginThrottle(5, 15*time.Minute)}
	s.routes()
	return s
}

// Handler, middleware zinciriyle sarılmış handler döndürür.
func (s *Server) Handler() http.Handler {
	var h http.Handler = s.mux
	h = s.authMiddleware(h)      // session/PAT + CSRF + zorunlu şifre değişimi
	h = s.recoverMiddleware(h)   // panic → 500 JSON
	h = s.requestIDMiddleware(h) // request_id → audit bağlamı
	return h
}

// routes, uç noktaları kaydeder.
func (s *Server) routes() {
	m := s.mux
	m.HandleFunc("GET /healthz", s.handleHealthz)

	// Gömülü SPA: "/" catch-all (API rotaları daha spesifik — kazanır).
	if s.deps.Web != nil {
		m.Handle("GET /", s.deps.Web)
	}

	// Auth (login public; diğerleri oturumlu).
	m.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	m.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	m.HandleFunc("GET /api/v1/auth/me", s.handleMe)
	m.HandleFunc("POST /api/v1/auth/change-password", s.handleChangePassword)
	m.HandleFunc("GET /api/v1/auth/mfa/start", s.handleMFAStart)
	m.HandleFunc("POST /api/v1/auth/mfa/enable", s.handleMFAEnable)
	m.HandleFunc("POST /api/v1/auth/mfa/disable", s.handleMFADisable)
	m.HandleFunc("POST /api/v1/auth/pat", s.handlePATCreate)
	m.HandleFunc("GET /api/v1/auth/pat", s.handlePATList)
	m.HandleFunc("DELETE /api/v1/auth/pat/{id}", s.handlePATDelete)

	// Durum ve Ayarlar.
	m.HandleFunc("GET /api/v1/status", s.handleStatus)
	m.HandleFunc("GET /api/v1/settings", s.handleGetSettings)
	m.HandleFunc("POST /api/v1/settings", s.handleSaveSettings)
	m.HandleFunc("GET /api/v1/settings/public", s.handlePublicSettings)

	// Siteler.
	m.HandleFunc("GET /api/v1/sites", s.handleSitesList)
	m.HandleFunc("POST /api/v1/sites", s.handleSiteCreate)
	m.HandleFunc("DELETE /api/v1/sites/{id}", s.handleSiteDelete)
	m.HandleFunc("PUT /api/v1/sites/{id}/limits", s.handleSiteLimits)
	m.HandleFunc("PUT /api/v1/sites/{id}/features", s.handleSiteFeatures)

	// Dosya yöneticisi.
	m.HandleFunc("GET /api/v1/sites/{id}/files", s.handleFilesList)
	m.HandleFunc("GET /api/v1/sites/{id}/files/content", s.handleFileRead)
	m.HandleFunc("PUT /api/v1/sites/{id}/files/content", s.handleFileWrite)
	m.HandleFunc("POST /api/v1/sites/{id}/files/mkdir", s.handleFileMkdir)
	m.HandleFunc("POST /api/v1/sites/{id}/files/rename", s.handleFileRename)
	m.HandleFunc("POST /api/v1/sites/{id}/files/copy", s.handleFileCopy)
	m.HandleFunc("POST /api/v1/sites/{id}/files/delete", s.handleFileDelete)
	m.HandleFunc("POST /api/v1/sites/{id}/files/symlink", s.handleFileSymlink)

	// Upload.
	m.HandleFunc("POST /api/v1/upload/init", s.handleUploadInit)
	m.HandleFunc("POST /api/v1/upload/chunk", s.handleUploadChunk)
	m.HandleFunc("POST /api/v1/upload/finalize", s.handleUploadFinalize)
	m.HandleFunc("POST /api/v1/upload/abort", s.handleUploadAbort)

	// Arşiv + trash.
	m.HandleFunc("POST /api/v1/sites/{id}/archive", s.handleArchive)
	m.HandleFunc("POST /api/v1/sites/{id}/trash/empty", s.handleTrashEmpty)

	// PHP.
	m.HandleFunc("GET /api/v1/php/versions", s.handlePHPVersions)
	m.HandleFunc("POST /api/v1/sites/{id}/php/switch", s.handlePHPSwitch)

	// SFTP.
	m.HandleFunc("GET /api/v1/sites/{id}/sftp", s.handleSFTPList)
	m.HandleFunc("POST /api/v1/sites/{id}/sftp", s.handleSFTPCreate)
	m.HandleFunc("DELETE /api/v1/sites/{id}/sftp/{username}", s.handleSFTPDelete)

	// Veritabanları.
	m.HandleFunc("GET /api/v1/sites/{id}/databases", s.handleDBList)
	m.HandleFunc("GET /api/v1/sites/{id}/db-users", s.handleDBUserList)
	m.HandleFunc("POST /api/v1/sites/{id}/databases", s.handleDBCreate)
	m.HandleFunc("DELETE /api/v1/sites/{id}/databases/{name}", s.handleDBDrop)
	m.HandleFunc("POST /api/v1/sites/{id}/db-users", s.handleDBUserCreate)
	m.HandleFunc("DELETE /api/v1/sites/{id}/db-users/{name}", s.handleDBUserDrop)
	m.HandleFunc("POST /api/v1/sites/{id}/db-users/{name}/password", s.handleDBUserPassword)
	m.HandleFunc("POST /api/v1/sites/{id}/db-grant", s.handleDBGrant)
	m.HandleFunc("POST /api/v1/adminer/open", s.handleAdminerOpen)

	// SSL.
	m.HandleFunc("GET /api/v1/sites/{id}/ssl", s.handleSSLInfo)
	m.HandleFunc("POST /api/v1/sites/{id}/ssl/letsencrypt", s.handleSSLEnableLE)
	m.HandleFunc("POST /api/v1/sites/{id}/ssl/custom", s.handleSSLCustom)
	m.HandleFunc("POST /api/v1/sites/{id}/ssl/disable", s.handleSSLDisable)

	// Yedekler.
	m.HandleFunc("POST /api/v1/sites/{id}/backups/run", s.handleBackupRun)
	m.HandleFunc("GET /api/v1/sites/{id}/backups", s.handleBackupList)

	// Drift.
	m.HandleFunc("GET /api/v1/drift", s.handleDriftList)
	m.HandleFunc("POST /api/v1/drift/scan", s.handleDriftScan)
	m.HandleFunc("POST /api/v1/drift/repair", s.handleDriftRepair)
	m.HandleFunc("PUT /api/v1/drift/auto-repair", s.handleDriftAutoRepair)

	// Güncelleme merkezi (W14).
	m.HandleFunc("GET /api/v1/update/check", s.handleUpdateCheck)
	m.HandleFunc("POST /api/v1/update/self", s.handleUpdateSelf)
}

// --- JSON yardımcıları ---

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// decodeBody, istek gövdesini sıkı kurallarla çözer (1 MiB sınır).
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("istek gövdesi: %v", err))
		return false
	}
	return true
}
