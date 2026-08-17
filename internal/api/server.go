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
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/auth"
	"github.com/mkoyazilim/aurapanel/internal/backup"
	"github.com/mkoyazilim/aurapanel/internal/config"
	"github.com/mkoyazilim/aurapanel/internal/cron"
	"github.com/mkoyazilim/aurapanel/internal/crypto"
	"github.com/mkoyazilim/aurapanel/internal/db"
	"github.com/mkoyazilim/aurapanel/internal/drift"
	"github.com/mkoyazilim/aurapanel/internal/fm"
	"github.com/mkoyazilim/aurapanel/internal/health"
	"github.com/mkoyazilim/aurapanel/internal/nodejs"
	"github.com/mkoyazilim/aurapanel/internal/staging"
	"github.com/mkoyazilim/aurapanel/internal/cloudflare"
	"github.com/mkoyazilim/aurapanel/internal/mail"
	"github.com/mkoyazilim/aurapanel/internal/php"
	"github.com/mkoyazilim/aurapanel/internal/git"
	"github.com/mkoyazilim/aurapanel/internal/privclient"
	"github.com/mkoyazilim/aurapanel/internal/security"
	"github.com/mkoyazilim/aurapanel/internal/sftp"
	"github.com/mkoyazilim/aurapanel/internal/site"
	"github.com/mkoyazilim/aurapanel/internal/ssl"
	"github.com/mkoyazilim/aurapanel/internal/extdns"
	"github.com/mkoyazilim/aurapanel/internal/reseller"
	"github.com/mkoyazilim/aurapanel/internal/store"
	"github.com/mkoyazilim/aurapanel/internal/update"
	"github.com/mkoyazilim/aurapanel/internal/wordpress"
)

// SiteManager, site yaşam döngüsü arayüzü (site.Manager bunu sağlar).
type SiteManager interface {
	Create(ctx context.Context, req site.CreateRequest) (string, error)
	Delete(ctx context.Context, siteID string) error
	UpdateLimits(ctx context.Context, siteID string, l site.Limits) error
	SetFeatureFlags(ctx context.Context, siteID string, flags map[string]bool) error
	ListSites(ctx context.Context, userID int64) ([]store.Site, error)
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
	Git       *git.Service
	Nodejs    *nodejs.Service
	Staging   *staging.Service
	Cloudflare *cloudflare.Service
	Mail      *mail.Service
	SFTP      *sftp.Service
	DB        *db.Service
	SSL       *ssl.Service
	Backups   *backup.Service
	DriftScan *drift.Scanner
	DriftFix  *drift.Repairer
	Updates   *update.Service
	Cron      *cron.Service
	Security  *security.Service
	Health    *health.Checker
	Wordpress *wordpress.Service
	Reseller  *reseller.Service
	ExtDNS    *extdns.Service
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
	go s.startHealthPoller()
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

	// SnappyMail OLS (8088) Proxy
	if snappyURL, err := url.Parse("http://127.0.0.1:8088"); err == nil {
		proxy := httputil.NewSingleHostReverseProxy(snappyURL)
		m.Handle("GET /snappymail/", proxy)
		m.Handle("POST /snappymail/", proxy)
	}

	// Auth (login public; diğerleri oturumlu).
	m.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	m.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	m.HandleFunc("GET /api/v1/auth/me", s.handleMe)
	m.HandleFunc("POST /api/v1/auth/change-password", s.handleChangePassword)
	m.HandleFunc("POST /api/v1/auth/change-username", s.handleChangeUsername)
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

	// Kullanıcı yönetimi.
	m.HandleFunc("GET /api/v1/users", s.handleUsersList)
	m.HandleFunc("POST /api/v1/users", s.handleUserCreate)
	m.HandleFunc("DELETE /api/v1/users/{id}", s.handleUserDelete)

	// Cluster yönetimi.
	m.HandleFunc("GET /api/v1/cluster", s.handleClusterList)
	m.HandleFunc("POST /api/v1/cluster", s.handleClusterAdd)
	m.HandleFunc("DELETE /api/v1/cluster/{id}", s.handleClusterDelete)
	m.HandleFunc("GET /api/v1/cluster/{id}/health", s.handleClusterHealth)
	// F3: Multi-server genişletme
	m.HandleFunc("GET /api/v1/cluster/metrics", s.handleClusterMetrics)
	m.HandleFunc("GET /api/v1/cluster/events", s.handleClusterEvents)
	m.HandleFunc("POST /api/v1/servers/{id}/rotate-key", s.handleClusterKeyRotate)
	m.HandleFunc("POST /api/v1/servers/{id}/sites", s.handleClusterCreateSite)

	// DNS yönetimi (F4 — PowerDNS tam entegrasyonu).
	m.HandleFunc("GET /api/v1/dns/zones", s.handleDNSZonesList)
	m.HandleFunc("POST /api/v1/dns/zones", s.handleDNSZoneCreate)
	m.HandleFunc("DELETE /api/v1/dns/zones/{domain}", s.handleDNSZoneDelete)
	m.HandleFunc("GET /api/v1/dns/zones/{domain}/records", s.handleDNSRecordsList)
	m.HandleFunc("POST /api/v1/dns/zones/{domain}/records", s.handleDNSRecordCreate)
	m.HandleFunc("DELETE /api/v1/dns/zones/{domain}/records", s.handleDNSRecordDelete)
	m.HandleFunc("PUT /api/v1/dns/zones/{domain}/soa", s.handleDNSSOAUpdate)
	m.HandleFunc("GET /api/v1/dns/zones/{domain}/export", s.handleDNSZoneExport)
	m.HandleFunc("POST /api/v1/dns/zones/{domain}/dnssec", s.handleDNSSECEnable)
	m.HandleFunc("DELETE /api/v1/dns/zones/{domain}/dnssec", s.handleDNSSECDisable)
	m.HandleFunc("GET /api/v1/dns/zones/{domain}/cryptokeys", s.handleDNSCryptoKeysList)
	m.HandleFunc("POST /api/v1/dns/zones/{domain}/cryptokeys", s.handleDNSCryptoKeyAdd)
	m.HandleFunc("DELETE /api/v1/dns/zones/{domain}/cryptokeys/{keyid}", s.handleDNSCryptoKeyDelete)
	m.HandleFunc("POST /api/v1/dns/zones/{domain}/rectify", s.handleDNSRectify)

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
	m.HandleFunc("GET /api/v1/sites/{id}/backup-schedule", s.handleSiteBackupScheduleGet)
	m.HandleFunc("POST /api/v1/sites/{id}/backup-schedule", s.handleSiteBackupScheduleSave)

	// Drift.
	m.HandleFunc("GET /api/v1/drift", s.handleDriftList)
	m.HandleFunc("POST /api/v1/drift/scan", s.handleDriftScan)
	m.HandleFunc("POST /api/v1/drift/repair", s.handleDriftRepair)
	m.HandleFunc("PUT /api/v1/drift/auto-repair", s.handleDriftAutoRepair)

	// Güncelleme merkezi (W14).
	m.HandleFunc("GET /api/v1/update/check", s.handleUpdateCheck)
	m.HandleFunc("POST /api/v1/update/self", s.handleUpdateSelf)

	// Cron yönetimi.
	m.HandleFunc("GET /api/v1/sites/{id}/crons", s.handleCronList)
	m.HandleFunc("POST /api/v1/sites/{id}/crons", s.handleCronCreate)
	m.HandleFunc("PUT /api/v1/sites/{id}/crons/{jobid}", s.handleCronUpdate)
	m.HandleFunc("DELETE /api/v1/sites/{id}/crons/{jobid}", s.handleCronDelete)

	// Metrikler.
	m.HandleFunc("GET /api/v1/sites/{id}/metrics", s.handleMetrics)



	// Sağlık kontrolleri.
	m.HandleFunc("GET /api/v1/health", s.handleHealthCheck)

	// Sistem kaynakları.
	m.HandleFunc("GET /api/v1/system/stats", s.handleSystemStats)

	// Canlı log (SSE).
	m.HandleFunc("GET /api/v1/sites/{id}/logs/tail", s.handleLogsTail)

	// S3 & Cloudflare R2 Remote Storage.
	m.HandleFunc("GET /api/v1/settings/s3", s.handleS3SettingsGet)
	m.HandleFunc("POST /api/v1/settings/s3", s.handleS3SettingsSave)
	m.HandleFunc("POST /api/v1/backups/s3/test", s.handleS3Test)

	// WordPress 1-Click Installer.
	m.HandleFunc("POST /api/v1/sites/{id}/wordpress/install", s.handleWordpressInstall)

	// Git Deploy.
	m.HandleFunc("GET /api/v1/sites/{id}/git", s.handleGitGet)
	m.HandleFunc("POST /api/v1/sites/{id}/git", s.handleGitSetup)
	m.HandleFunc("POST /api/v1/sites/{id}/git/deploy", s.handleGitDeploy)
	m.HandleFunc("DELETE /api/v1/sites/{id}/git", s.handleGitDelete)

	// Node.js
	m.HandleFunc("GET /api/v1/sites/{id}/nodejs", s.handleNodeAppsList)
	m.HandleFunc("POST /api/v1/sites/{id}/nodejs", s.handleNodeAppCreate)
	m.HandleFunc("DELETE /api/v1/sites/{id}/nodejs/{appId}", s.handleNodeAppDelete)
	m.HandleFunc("POST /api/v1/sites/{id}/nodejs/{appId}/restart", s.handleNodeAppRestart)

	// Staging
	m.HandleFunc("GET /api/v1/sites/{id}/staging", s.handleStagingList)
	m.HandleFunc("POST /api/v1/sites/{id}/staging", s.handleStagingCreate)
	m.HandleFunc("POST /api/v1/sites/{id}/staging/push", s.handleStagingPush)
	m.HandleFunc("DELETE /api/v1/sites/{id}/staging/{envId}", s.handleStagingDelete)

	// Cloudflare — Global hesap yönetimi
	m.HandleFunc("GET /api/v1/cloudflare/account", s.handleCFAccountGet)
	m.HandleFunc("POST /api/v1/cloudflare/account", s.handleCFAccountSave)
	m.HandleFunc("POST /api/v1/cloudflare/verify", s.handleCFVerifyToken)
	m.HandleFunc("GET /api/v1/cloudflare/zones", s.handleCFListZones)

	// Cloudflare — Site bazlı zone bağlaması
	m.HandleFunc("GET /api/v1/sites/{id}/cloudflare", s.handleCloudflareGet)
	m.HandleFunc("POST /api/v1/sites/{id}/cloudflare", s.handleCloudflareSave)

	// Cloudflare — DNS kayıtları
	m.HandleFunc("GET /api/v1/sites/{id}/cloudflare/dns", s.handleCFDNSList)
	m.HandleFunc("POST /api/v1/sites/{id}/cloudflare/dns", s.handleCFDNSCreate)
	m.HandleFunc("PATCH /api/v1/sites/{id}/cloudflare/dns/{recid}", s.handleCFDNSUpdate)
	m.HandleFunc("DELETE /api/v1/sites/{id}/cloudflare/dns/{recid}", s.handleCFDNSDelete)

	// Cloudflare — Zone ayarları
	m.HandleFunc("GET /api/v1/sites/{id}/cloudflare/settings", s.handleCFSettingsGet)
	m.HandleFunc("PATCH /api/v1/sites/{id}/cloudflare/settings/{key}", s.handleCFSettingUpdate)

	// Cloudflare — Cache
	m.HandleFunc("POST /api/v1/sites/{id}/cloudflare/purge", s.handleCloudflarePurge)
	m.HandleFunc("POST /api/v1/sites/{id}/cloudflare/purge-urls", s.handleCFPurgeURLs)

	// Cloudflare — Firewall kuralları
	m.HandleFunc("GET /api/v1/sites/{id}/cloudflare/firewall", s.handleCFFirewallList)
	m.HandleFunc("POST /api/v1/sites/{id}/cloudflare/firewall", s.handleCFFirewallCreate)
	m.HandleFunc("DELETE /api/v1/sites/{id}/cloudflare/firewall/{ruleid}", s.handleCFFirewallDelete)

	// Cloudflare — Analytics
	m.HandleFunc("GET /api/v1/sites/{id}/cloudflare/analytics", s.handleCFAnalytics)


	// Mail
	m.HandleFunc("GET /api/v1/sites/{id}/mail", s.handleMailList)
	m.HandleFunc("POST /api/v1/sites/{id}/mail", s.handleMailCreate)
	m.HandleFunc("DELETE /api/v1/sites/{id}/mail/{email}", s.handleMailDelete)
	m.HandleFunc("GET /api/v1/mail/status", s.handleMailStatus)
	m.HandleFunc("POST /api/v1/mail/setup", s.handleMailSetup)
	m.HandleFunc("POST /api/v1/sites/{id}/mail/dkim", s.handleMailDKIM)
	m.HandleFunc("GET /api/v1/sites/{id}/mail/dkim", s.handleMailDKIMGet)
	m.HandleFunc("PUT /api/v1/sites/{id}/mail/{email}/password", s.handleMailPasswordChange)

	// Phase 3 (Cluster)
	m.HandleFunc("GET /api/v1/servers", s.handlePhase3Servers)

	// Security (WAF)
	m.HandleFunc("GET /api/v1/sites/{id}/security", s.handleSecurityGet)
	m.HandleFunc("POST /api/v1/sites/{id}/security", s.handleSecuritySet)
	m.HandleFunc("POST /api/v1/sites/{id}/security/waf", s.handleSecurityWAF)
	
	// Webhook (NO AUTH REQUIRED)
	m.HandleFunc("POST /api/v1/webhooks/git/{secret}", s.handleGitWebhook)
	// Sunucu Yönetimi
	m.HandleFunc("GET /api/v1/server/metrics", s.handleServerMetrics)
	m.HandleFunc("GET /api/v1/server/services", s.handleServerServices)
	m.HandleFunc("POST /api/v1/server/action", s.handleServerAction)
	m.HandleFunc("GET /api/v1/server/firewall", s.handleFirewallList)
	m.HandleFunc("POST /api/v1/server/firewall", s.handleFirewallRuleAdd)
	m.HandleFunc("DELETE /api/v1/server/firewall", s.handleFirewallRuleDelete)
	m.HandleFunc("PUT /api/v1/server/ssh-port", s.handleSSHPortChange)
	m.HandleFunc("PUT /api/v1/server/panel-port", s.handlePanelPortChange)

	// Reseller yönetimi (admin tarafı)
	m.HandleFunc("GET /api/v1/resellers", s.handleResellerList)
	m.HandleFunc("POST /api/v1/resellers", s.handleResellerCreate)
	m.HandleFunc("DELETE /api/v1/resellers/{id}", s.handleResellerDelete)
	m.HandleFunc("GET /api/v1/resellers/{id}/quota", s.handleResellerGetQuota)
	m.HandleFunc("PUT /api/v1/resellers/{id}/quota", s.handleResellerSetQuota)

	// Reseller portal (reseller kendi tarafı)
	m.HandleFunc("GET /api/v1/reseller/me", s.handleResellerMe)
	m.HandleFunc("GET /api/v1/reseller/my/sites", s.handleResellerSites)

	// External DNS (F5 — Cloudflare çift-yönlü senkron + Route53)
	m.HandleFunc("GET /api/v1/extdns/providers", s.handleExtDNSList)
	m.HandleFunc("POST /api/v1/extdns/providers", s.handleExtDNSCreate)
	m.HandleFunc("DELETE /api/v1/extdns/providers/{id}", s.handleExtDNSDelete)
	m.HandleFunc("GET /api/v1/extdns/providers/{id}/cf/records", s.handleExtDNSCFRecords)
	m.HandleFunc("POST /api/v1/extdns/providers/{id}/cf/sync", s.handleExtDNSCFSync)
	m.HandleFunc("GET /api/v1/extdns/providers/{id}/r53/zones", s.handleExtDNSR53Zones)
	m.HandleFunc("GET /api/v1/extdns/sync-log", s.handleExtDNSSyncLog)

	// Advanced WAF (F6)
	m.HandleFunc("GET /api/v1/waf/crs", s.handleWAFCRSGet)
	m.HandleFunc("PUT /api/v1/waf/crs", s.handleWAFCRSUpdate)
	m.HandleFunc("GET /api/v1/sites/{id}/waf/rules", s.handleWAFRulesList)
	m.HandleFunc("POST /api/v1/sites/{id}/waf/rules", s.handleWAFRuleCreate)
	m.HandleFunc("PUT /api/v1/sites/{id}/waf/rules/{ruleid}", s.handleWAFRuleUpdate)
	m.HandleFunc("DELETE /api/v1/sites/{id}/waf/rules/{ruleid}", s.handleWAFRuleDelete)
	m.HandleFunc("POST /api/v1/sites/{id}/waf/test", s.handleWAFTest)
	m.HandleFunc("GET /api/v1/sites/{id}/waf/log", s.handleWAFLog)

	// CDN yönetimi (F7)
	m.HandleFunc("POST /api/v1/sites/{id}/cdn/purge", s.handleCDNPurge)
	m.HandleFunc("POST /api/v1/sites/{id}/cdn/cf-purge", s.handleCDNCFPurge)
	m.HandleFunc("GET /api/v1/sites/{id}/cdn/rules", s.handleCDNRulesList)
	m.HandleFunc("POST /api/v1/sites/{id}/cdn/rules", s.handleCDNRuleCreate)
	m.HandleFunc("PUT /api/v1/sites/{id}/cdn/rules/{ruleid}", s.handleCDNRuleUpdate)
	m.HandleFunc("DELETE /api/v1/sites/{id}/cdn/rules/{ruleid}", s.handleCDNRuleDelete)
	m.HandleFunc("GET /api/v1/sites/{id}/cdn/stats", s.handleCDNStats)
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
