package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultServiceAddr  = ":8081"
	defaultGatewayPort  = 8090
	currentPanelVersion = "Aura Panel V1"
	updateCacheTTL      = 15 * time.Minute
)

type apiResponse struct {
	Status     string      `json:"status"`
	Message    string      `json:"message,omitempty"`
	Data       interface{} `json:"data,omitempty"`
	Pagination interface{} `json:"pagination,omitempty"`
	Valid      bool        `json:"valid,omitempty"`
	Allowed    bool        `json:"allowed,omitempty"`
	Score      int         `json:"score,omitempty"`
	Reason     string      `json:"reason,omitempty"`
}

type pagination struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type Website struct {
	Domain        string `json:"domain"`
	Owner         string `json:"owner"`
	User          string `json:"user"`
	PHP           string `json:"php"`
	PHPVersion    string `json:"php_version"`
	Package       string `json:"package"`
	Email         string `json:"email"`
	Status        string `json:"status"`
	SSL           bool   `json:"ssl"`
	DiskUsage     string `json:"disk_usage"`
	Quota         string `json:"quota"`
	MailDomain    bool   `json:"mail_domain"`
	ApacheBackend bool   `json:"apache_backend"`
	CreatedAt     int64  `json:"created_at"`
}

type Package struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	PlanType    string `json:"plan_type"`
	DiskGB      int    `json:"disk_gb"`
	BandwidthGB int    `json:"bandwidth_gb"`
	Domains     int    `json:"domains"`
	Databases   int    `json:"databases"`
	Emails      int    `json:"emails"`
	CPULimit    int    `json:"cpu_limit"`
	RamMB       int    `json:"ram_mb"`
	IOLimit     int    `json:"io_limit"`
}

type PanelUser struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	Package      string `json:"package"`
	Sites        int    `json:"sites"`
	Active       bool   `json:"active"`
	TwoFAEnabled bool   `json:"two_fa_enabled"`
	PasswordHash string `json:"password_hash,omitempty"`
}

type DatabaseRecord struct {
	Name       string `json:"name"`
	Size       string `json:"size"`
	Tables     int    `json:"tables"`
	Engine     string `json:"engine"`
	Owner      string `json:"owner,omitempty"`
	SiteDomain string `json:"site_domain,omitempty"`
}

type DatabaseUser struct {
	Username     string `json:"username"`
	Host         string `json:"host"`
	Engine       string `json:"engine"`
	LinkedDBName string `json:"db_name,omitempty"`
	PasswordHash string `json:"password_hash,omitempty"`
}

type RemoteAccessRule struct {
	Engine     string `json:"engine"`
	DBUser     string `json:"db_user"`
	DBName     string `json:"db_name"`
	Remote     string `json:"remote"`
	AuthMethod string `json:"auth_method"`
}

type WebsiteDBLink struct {
	Domain   string `json:"domain"`
	Engine   string `json:"engine"`
	DBName   string `json:"db_name"`
	DBUser   string `json:"db_user"`
	LinkedAt int64  `json:"linked_at"`
}

type Subdomain struct {
	FQDN         string `json:"fqdn"`
	ParentDomain string `json:"parent_domain"`
	PHPVersion   string `json:"php_version"`
	SSLEnabled   bool   `json:"ssl_enabled"`
	CreatedAt    int64  `json:"created_at"`
}

type DomainAlias struct {
	Domain string `json:"domain"`
	Alias  string `json:"alias"`
}

type WebsiteAdvancedConfig struct {
	OpenBasedir  bool   `json:"open_basedir"`
	RewriteRules string `json:"rewrite_rules"`
	VhostConfig  string `json:"vhost_config"`
}

func defaultWebsiteAdvancedConfig() WebsiteAdvancedConfig {
	return WebsiteAdvancedConfig{
		OpenBasedir:  true,
		RewriteRules: "RewriteEngine On",
		VhostConfig:  "",
	}
}

type WebsiteCustomSSL struct {
	CertPEM string `json:"cert_pem"`
	KeyPEM  string `json:"key_pem"`
}

type ServiceStatus struct {
	Name   string `json:"name"`
	Desc   string `json:"desc"`
	Status string `json:"status"`
}

type ProcessInfo struct {
	PID     int     `json:"pid"`
	User    string  `json:"user"`
	CPU     float64 `json:"cpu"`
	Mem     float64 `json:"mem"`
	Command string  `json:"command"`
}

type FirewallRule struct {
	IPAddress string `json:"ip_address"`
	Block     bool   `json:"block"`
	Reason    string `json:"reason"`
}

type UpdateStatus struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseName     string `json:"release_name,omitempty"`
	ReleaseURL      string `json:"release_url,omitempty"`
	ReleaseNotes    string `json:"release_notes,omitempty"`
	PublishedAt     string `json:"published_at,omitempty"`
	Source          string `json:"source"`
	CheckedAt       string `json:"checked_at,omitempty"`
	Error           string `json:"error,omitempty"`
}

type updateStatusCache struct {
	Data      UpdateStatus
	CheckedAt time.Time
}

type SSHKey struct {
	ID        string `json:"id"`
	User      string `json:"user"`
	Title     string `json:"title"`
	PublicKey string `json:"public_key"`
}

type MalwareJob struct {
	ID            string           `json:"id"`
	Status        string           `json:"status"`
	Progress      int              `json:"progress"`
	InfectedFiles int              `json:"infected_files"`
	TargetPath    string           `json:"target_path"`
	Findings      []MalwareFinding `json:"findings"`
	Logs          []string         `json:"logs"`
}

type MalwareFinding struct {
	ID          string `json:"id"`
	FilePath    string `json:"file_path"`
	Signature   string `json:"signature"`
	Engine      string `json:"engine"`
	Quarantined bool   `json:"quarantined"`
}

type QuarantineRecord struct {
	ID             string `json:"id"`
	JobID          string `json:"job_id"`
	FindingID      string `json:"finding_id"`
	OriginalPath   string `json:"original_path"`
	QuarantinePath string `json:"quarantine_path"`
	RestoredAt     string `json:"restored_at,omitempty"`
}

type appState struct {
	GatewayPort         int
	Websites            []Website
	Packages            []Package
	Users               []PanelUser
	MariaDBs            []DatabaseRecord
	PostgresDBs         []DatabaseRecord
	MariaUsers          []DatabaseUser
	PostgresUsers       []DatabaseUser
	MariaRemoteRules    []RemoteAccessRule
	PostgresRemoteRules []RemoteAccessRule
	DBLinks             []WebsiteDBLink
	Subdomains          []Subdomain
	Aliases             []DomainAlias
	AdvancedConfig      map[string]WebsiteAdvancedConfig
	CustomSSL           map[string]WebsiteCustomSSL
	Services            []ServiceStatus
	Processes           []ProcessInfo
	FirewallRules       []FirewallRule
	SSHKeys             []SSHKey
	EBPFEvents          []string
	MalwareJobs         []MalwareJob
	Quarantine          []QuarantineRecord
	TwoFASecrets        map[string]string
	NextPackageID       int
	NextUserID          int
	NextProcessPID      int
}

type service struct {
	mu        sync.RWMutex
	startedAt time.Time
	state     appState
	modules   moduleState
	update    updateStatusCache
}

type jwtClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

func main() {
	svc := newService()
	addr := strings.TrimSpace(os.Getenv("AURAPANEL_SERVICE_ADDR"))
	if addr == "" {
		addr = defaultServiceAddr
	}

	log.Printf("AuraPanel panel-service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, svc.routes()))
}

func newService() *service {
	svc := &service{
		startedAt: time.Now().UTC(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	if err := svc.loadRuntimeState(); err != nil {
		log.Printf("runtime state load skipped: %v", err)
	}
	svc.bootstrapModules()
	return svc
}

func seedState() appState {
	adminEmail, adminHash := loadAdminSeedCredentials()

	users := []PanelUser{
		{
			ID:           1,
			Username:     "admin",
			Name:         "System Administrator",
			Email:        adminEmail,
			Role:         "admin",
			Package:      "default",
			Sites:        0,
			Active:       true,
			TwoFAEnabled: false,
			PasswordHash: adminHash,
		},
	}

	return appState{
		GatewayPort: defaultGatewayPort,
		Websites:    []Website{},
		Packages: []Package{
			{
				ID:          1,
				Name:        "default",
				PlanType:    "hosting",
				DiskGB:      10,
				BandwidthGB: 0,
				Domains:     3,
				Databases:   5,
				Emails:      20,
				CPULimit:    100,
				RamMB:       2048,
				IOLimit:     50,
			},
			{
				ID:          2,
				Name:        "reseller-starter",
				PlanType:    "reseller",
				DiskGB:      50,
				BandwidthGB: 0,
				Domains:     50,
				Databases:   100,
				Emails:      200,
				CPULimit:    200,
				RamMB:       4096,
				IOLimit:     100,
			},
		},
		Users:               users,
		MariaDBs:            []DatabaseRecord{},
		PostgresDBs:         []DatabaseRecord{},
		MariaUsers:          []DatabaseUser{},
		PostgresUsers:       []DatabaseUser{},
		MariaRemoteRules:    []RemoteAccessRule{},
		PostgresRemoteRules: []RemoteAccessRule{},
		DBLinks:             []WebsiteDBLink{},
		Subdomains:          []Subdomain{},
		Aliases:             []DomainAlias{},
		AdvancedConfig:      map[string]WebsiteAdvancedConfig{},
		CustomSSL:           map[string]WebsiteCustomSSL{},
		Services:            []ServiceStatus{},
		Processes:           []ProcessInfo{},
		FirewallRules:       []FirewallRule{},
		SSHKeys:             []SSHKey{},
		EBPFEvents:          []string{},
		MalwareJobs:         []MalwareJob{},
		Quarantine:          []QuarantineRecord{},
		TwoFASecrets:        map[string]string{},
		NextPackageID:       3,
		NextUserID:          2,
		NextProcessPID:      1201,
	}
}

func (s *service) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/v1/", persistenceMiddleware(loggingMiddleware(http.HandlerFunc(s.handleCompat)), s))
	return mux
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[%s] %s %s", time.Since(start).Round(time.Millisecond), r.Method, r.URL.Path)
	})
}

type statusCapturingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCapturingResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(data)
}

func (w *statusCapturingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusCapturingResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *statusCapturingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (w *statusCapturingResponseWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (w *statusCapturingResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return readerFrom.ReadFrom(r)
	}
	return io.Copy(w.ResponseWriter, r)
}

func (w *statusCapturingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func isWebsocketUpgradeRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if strings.HasSuffix(r.URL.Path, "/terminal/ws") {
		return true
	}
	return strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") &&
		strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket")
}

func persistenceMiddleware(next http.Handler, svc *service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isWebsocketUpgradeRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		rec := &statusCapturingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		// Sadece GET ve OPTIONS isteklerini kaydetmiyoruz.
		// Diger her turlu mutation (POST) sonucunda state diske yazilir (Hata donse bile)
		// Cunku bazi durumlarda islem yarida kalsa da DB (state) guncellenmis olabilir.
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			return
		}

		// Eger 500 Internal Server error vb bir cok kritik hata varsa state bozmamak icin yazmayabiliriz,
		// ama guvenlik amaciyla genel olarak mutation sonrasi state'i senkronize etmekte fayda var.
		// Sadece validation hatalarinda (400) eger hicbir sey degismediyse diye pas geciyorduk,
		// ancak biz simdilik her halukarda save yapalim ki state ile in-memory kopmasin.
		if err := svc.saveRuntimeState(); err != nil {
			log.Printf("runtime state save failed after %s %s: %v", r.Method, r.URL.Path, err)
		}
	})
}

func (s *service) handleCompat(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/health":
		s.handleHealth(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/login":
		s.handleAuthLogin(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/vhost/list":
		s.handleVhostList(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/vhost":
		s.handleVhostCreate(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/vhost/create":
		s.handleVhostCreate(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/vhost/delete":
		s.handleVhostDelete(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/vhost/suspend":
		s.setWebsiteStatus(w, r, "suspended")
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/vhost/unsuspend":
		s.setWebsiteStatus(w, r, "active")
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/vhost/update":
		s.handleVhostUpdate(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/users/list":
		s.handleUsersList(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/users/create":
		s.handleUsersCreate(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/users/delete":
		s.handleUsersDelete(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/users/change-password":
		s.handleUsersChangePassword(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/packages/list":
		s.handlePackagesList(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/packages/create":
		s.handlePackagesCreate(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/packages/update":
		s.handlePackagesUpdate(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/packages/delete":
		s.handlePackagesDelete(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/db/mariadb/list":
		s.handleDatabaseList(w, "mariadb")
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/db/mariadb/users":
		s.handleDatabaseUsers(w, "mariadb")
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/db/mariadb/remote-access":
		s.handleRemoteAccessList(w, "mariadb")
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/mariadb/create":
		s.handleDatabaseCreate(w, r, "mariadb")
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/mariadb/drop":
		s.handleDatabaseDrop(w, r, "mariadb")
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/mariadb/password":
		s.handleDatabasePasswordUpdate(w, r, "mariadb")
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/mariadb/remote-access":
		s.handleRemoteAccessCreate(w, r, "mariadb")
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/db/postgres/list":
		s.handleDatabaseList(w, "postgresql")
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/db/postgres/users":
		s.handleDatabaseUsers(w, "postgresql")
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/db/postgres/remote-access":
		s.handleRemoteAccessList(w, "postgresql")
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/postgres/create":
		s.handleDatabaseCreate(w, r, "postgresql")
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/postgres/drop":
		s.handleDatabaseDrop(w, r, "postgresql")
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/postgres/password":
		s.handleDatabasePasswordUpdate(w, r, "postgresql")
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/postgres/remote-access":
		s.handleRemoteAccessCreate(w, r, "postgresql")
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/websites/subdomains":
		s.handleSubdomainList(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/websites/subdomains":
		s.handleSubdomainCreate(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/websites/db-links":
		s.handleDBLinksList(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/websites/db-links":
		s.handleDBLinksCreate(w, r)
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/websites/db-links":
		s.handleDBLinksDelete(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/websites/db-links/verify":
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: map[string]interface{}{"ready": true}})
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/websites/aliases":
		s.handleAliasesList(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/status/metrics":
		s.handleMetrics(w)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/status/services":
		s.handleServices(w)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/status/processes":
		s.handleProcesses(w)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/status/update":
		s.handleUpdateStatus(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/status/service/control":
		s.handleServiceControl(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/status/panel-port":
		s.handlePanelPortGet(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/status/panel-port":
		s.handlePanelPortSet(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/status":
		s.handleSecurityStatus(w)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/cloudflare/status":
		s.handleCloudflareStatus(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloudflare/server-auth":
		s.handleCloudflareServerAuth(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/ebpf/events":
		s.handleEBPFEvents(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/ebpf/collect":
		s.handleCollectEBPF(w)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/firewall/rules":
		s.handleFirewallRulesList(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/firewall":
		s.handleFirewallRuleCreate(w, r)
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/security/firewall/rules":
		s.handleFirewallRuleDelete(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/waf":
		s.handleSecurityWAF(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/2fa/setup":
		s.handleTOTPSetup(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/2fa/verify":
		s.handleTOTPVerify(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/ssh-keys":
		s.handleSSHKeysList(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/ssh-keys":
		s.handleSSHKeyCreate(w, r)
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/security/ssh-keys":
		s.handleSSHKeyDelete(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/immutable/status":
		s.handleImmutableStatus(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/hardening/apply":
		s.handleHardeningApply(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/monitor/logs/site":
		s.handleSiteLogs(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ssl/issue":
		s.handleSSLIssue(w, r)
	default:
		if s.handleExtendedRoutes(w, r) {
			return
		}
		s.handleFallback(w, r)
	}
}

func (s *service) handleHealth(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"name":         "AuraPanel Go Service",
			"architecture": "vue -> go-gateway -> go-service",
			"version":      currentPanelVersion,
			"status":       "ok",
			"uptime":       time.Since(s.startedAt).Round(time.Second).String(),
		},
	})
}

func (s *service) handleUpdateStatus(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data:   s.getUpdateStatus(),
	})
}

func (s *service) getUpdateStatus() UpdateStatus {
	s.mu.RLock()
	cached := s.update
	s.mu.RUnlock()

	if !cached.CheckedAt.IsZero() && time.Since(cached.CheckedAt) < updateCacheTTL {
		return cached.Data
	}

	status := fetchLatestReleaseStatus()

	s.mu.Lock()
	s.update = updateStatusCache{
		Data:      status,
		CheckedAt: time.Now().UTC(),
	}
	s.mu.Unlock()

	return status
}

func fetchLatestReleaseStatus() UpdateStatus {
	status := UpdateStatus{
		CurrentVersion: currentPanelVersion,
		Source:         "GitHub Releases",
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	owner := envOr("AURAPANEL_GH_OWNER", "mkoyazilim")
	repo := envOr("AURAPANEL_GH_REPO", "aurapanel")
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "aurapanel-panel-service")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status.Error = fmt.Sprintf("GitHub release check returned HTTP %d", resp.StatusCode)
		return status
	}

	var payload struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		HTMLURL     string `json:"html_url"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
		Draft       bool   `json:"draft"`
		PreRelease  bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		status.Error = err.Error()
		return status
	}

	if payload.Draft {
		status.Error = "Latest GitHub release is still marked as draft."
		return status
	}

	status.ReleaseName = strings.TrimSpace(payload.Name)
	status.LatestVersion = firstNonEmpty(strings.TrimSpace(payload.Name), strings.TrimSpace(payload.TagName))
	status.ReleaseURL = strings.TrimSpace(payload.HTMLURL)
	status.PublishedAt = strings.TrimSpace(payload.PublishedAt)
	status.ReleaseNotes = summarizeReleaseNotes(payload.Body)

	currentComparable := normalizeComparableVersion(status.CurrentVersion)
	latestComparable := normalizeComparableVersion(status.LatestVersion)
	status.UpdateAvailable = latestComparable != "" && latestComparable != currentComparable
	return status
}

func summarizeReleaseNotes(body string) string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimLeft(line, "-*# "))
		if trimmed == "" {
			continue
		}
		if len(trimmed) > 180 {
			return trimmed[:180] + "..."
		}
		return trimmed
	}
	return ""
}

func normalizeComparableVersion(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(
		"aurapanel", "",
		"aura panel", "",
		"release", "",
		"version", "",
		" ", "",
		"_", "",
		"-", "",
	)
	return replacer.Replace(normalized)
}

func (s *service) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		TOTPToken string `json:"totp_token"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid login payload.")
		return
	}

	email := strings.TrimSpace(payload.Email)
	password := strings.TrimSpace(payload.Password)
	if email == "" || password == "" {
		writeError(w, http.StatusBadRequest, "Email and password are required.")
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var matched *PanelUser
	for i := range s.state.Users {
		user := &s.state.Users[i]
		if strings.EqualFold(user.Email, email) || strings.EqualFold(user.Username, email) {
			matched = user
			break
		}
	}
	if matched == nil {
		writeError(w, http.StatusUnauthorized, "Invalid credentials.")
		return
	}
	if !matched.Active {
		writeError(w, http.StatusForbidden, "Account is inactive.")
		return
	}
	if matched.TwoFAEnabled && !verifyStoredTOTPSecret(s.state.TwoFASecrets[matched.Username], strings.TrimSpace(payload.TOTPToken), time.Now().UTC()) {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"status":       "error",
			"message":      "2FA code is required.",
			"requires_2fa": true,
		})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(matched.PasswordHash), []byte(password)) != nil {
		writeError(w, http.StatusUnauthorized, "Invalid credentials.")
		return
	}

	token, err := issueToken(*matched)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Token generation failed.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"token":  token,
		"user":   publicUser(*matched),
	})
}

func (s *service) handleVhostList(w http.ResponseWriter, r *http.Request) {
	search := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("search")))
	php := strings.TrimSpace(r.URL.Query().Get("php"))
	page := maxInt(1, queryInt(r, "page", 1))
	perPage := clampInt(queryInt(r, "per_page", 20), 1, 200)

	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []Website
	for _, site := range s.state.Websites {
		if search != "" && !strings.Contains(strings.ToLower(site.Domain), search) && !strings.Contains(strings.ToLower(site.Owner), search) {
			continue
		}
		if php != "" && site.PHPVersion != php && site.PHP != php {
			continue
		}

		// Ensure SSL status accurately reflects reality
		if cert, ok := s.modules.SSLCertificates[site.Domain]; ok {
			site.SSL = cert.Status == "issued"
		} else {
			certPath, _ := findCertificatePair(site.Domain)
			site.SSL = certPath != ""
		}

		filtered = append(filtered, site)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Domain < filtered[j].Domain
	})

	total := len(filtered)
	totalPages := maxInt(1, (total+perPage-1)/perPage)
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data:   filtered[start:end],
		Pagination: pagination{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

func (s *service) handleVhostCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain        string `json:"domain"`
		User          string `json:"user"`
		Owner         string `json:"owner"`
		PHPVersion    string `json:"php_version"`
		Package       string `json:"package"`
		Email         string `json:"email"`
		MailDomain    bool   `json:"mail_domain"`
		ApacheBackend bool   `json:"apache_backend"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid website payload.")
		return
	}

	domain := normalizeDomain(payload.Domain)
	if domain == "" {
		writeError(w, http.StatusBadRequest, "Domain is required.")
		return
	}
	if payload.ApacheBackend && !apacheBackendAvailable() {
		writeError(w, http.StatusBadRequest, "Apache backend is not available on this server.")
		return
	}
	if payload.MailDomain && !collectSecuritySnapshot().MailDomainAvailable {
		writeError(w, http.StatusBadRequest, "Mail domain stack is not active on this server.")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.findWebsiteLocked(domain) != nil {
		writeError(w, http.StatusConflict, "Website already exists.")
		return
	}
	snapshot, err := s.captureRuntimeSnapshotLocked()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to prepare website provisioning rollback state.")
		return
	}

	owner := firstNonEmpty(strings.TrimSpace(payload.Owner), strings.TrimSpace(payload.User), "aura")
	email := firstNonEmpty(strings.TrimSpace(payload.Email), fmt.Sprintf("webmaster@%s", domain))
	phpVersion := firstNonEmpty(strings.TrimSpace(payload.PHPVersion), "8.3")

	site := Website{
		Domain:        domain,
		Owner:         owner,
		User:          owner,
		PHP:           phpVersion,
		PHPVersion:    phpVersion,
		Package:       firstNonEmpty(strings.TrimSpace(payload.Package), "default"),
		Email:         email,
		Status:        "active",
		SSL:           false,
		DiskUsage:     "0.0 GB",
		Quota:         quotaForPackage(s.state.Packages, firstNonEmpty(strings.TrimSpace(payload.Package), "default")),
		MailDomain:    payload.MailDomain,
		ApacheBackend: payload.ApacheBackend,
		CreatedAt:     time.Now().UTC().Unix(),
	}

	s.state.Websites = append(s.state.Websites, site)
	s.ensureUserLocked(owner, fmt.Sprintf("%s@example.com", owner), "user", site.Package, "")
	s.recountSitesLocked()
	if err := s.provisionWebsiteArtifactsLocked(site); err != nil {
		s.restoreRuntimeSnapshotLocked(snapshot)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.saveRuntimeStateLocked()

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Website created.", Data: site})
}

func (s *service) handleVhostDelete(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string `json:"domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid delete payload.")
		return
	}

	domain := normalizeDomain(payload.Domain)
	s.mu.Lock()
	defer s.mu.Unlock()

	index := s.findWebsiteIndexLocked(domain)
	if index < 0 {
		writeError(w, http.StatusNotFound, "Website not found.")
		return
	}

	s.state.Websites = append(s.state.Websites[:index], s.state.Websites[index+1:]...)
	if err := s.removeSiteArtifactsLocked(domain); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.recountSitesLocked()
	s.saveRuntimeStateLocked()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Website removed."})
}

func (s *service) handleVhostUpdate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain     string `json:"domain"`
		Owner      string `json:"owner"`
		User       string `json:"user"`
		PHPVersion string `json:"php_version"`
		Package    string `json:"package"`
		Email      string `json:"email"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid update payload.")
		return
	}

	domain := normalizeDomain(payload.Domain)
	s.mu.Lock()
	defer s.mu.Unlock()

	site := s.findWebsiteLocked(domain)
	if site == nil {
		writeError(w, http.StatusNotFound, "Website not found.")
		return
	}

	owner := firstNonEmpty(strings.TrimSpace(payload.Owner), strings.TrimSpace(payload.User), site.Owner)
	site.Owner = owner
	site.User = owner
	if phpVersion := strings.TrimSpace(payload.PHPVersion); phpVersion != "" {
		site.PHP = phpVersion
		site.PHPVersion = phpVersion
	}
	if pkg := strings.TrimSpace(payload.Package); pkg != "" {
		site.Package = pkg
		site.Quota = quotaForPackage(s.state.Packages, pkg)
	}
	if email := strings.TrimSpace(payload.Email); email != "" {
		site.Email = email
	}
	s.ensureUserLocked(owner, fmt.Sprintf("%s@example.com", owner), "user", site.Package, "")
	s.recountSitesLocked()
	if err := s.syncOLSVhostsLocked(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Website updated.", Data: site})
}

func (s *service) handleUsersList(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: publicUsers(s.state.Users)})
}

func (s *service) handleUsersCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Package  string `json:"package"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user payload.")
		return
	}
	if strings.TrimSpace(payload.Username) == "" || strings.TrimSpace(payload.Email) == "" {
		writeError(w, http.StatusBadRequest, "Username and email are required.")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	username := sanitizeName(payload.Username)
	if s.findUserLocked(username) != nil {
		writeError(w, http.StatusConflict, "User already exists.")
		return
	}

	passwordHash := mustHashPassword(firstNonEmpty(strings.TrimSpace(payload.Password), "password123"))
	user := PanelUser{
		ID:           s.state.NextUserID,
		Username:     username,
		Name:         strings.Title(username),
		Email:        strings.TrimSpace(payload.Email),
		Role:         normalizeRole(payload.Role),
		Package:      firstNonEmpty(strings.TrimSpace(payload.Package), "default"),
		Sites:        0,
		Active:       true,
		TwoFAEnabled: false,
		PasswordHash: passwordHash,
	}
	s.state.NextUserID++
	s.state.Users = append(s.state.Users, user)

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "User created.", Data: publicUser(user)})
}

func (s *service) handleUsersDelete(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Username string `json:"username"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid delete payload.")
		return
	}
	username := sanitizeName(payload.Username)

	s.mu.Lock()
	defer s.mu.Unlock()

	index := s.findUserIndexLocked(username)
	if index < 0 {
		writeError(w, http.StatusNotFound, "User not found.")
		return
	}
	if s.state.Users[index].Role == "admin" {
		writeError(w, http.StatusForbidden, "Admin user cannot be deleted.")
		return
	}
	s.state.Users = append(s.state.Users[:index], s.state.Users[index+1:]...)
	for i := range s.state.Websites {
		if s.state.Websites[i].Owner == username || s.state.Websites[i].User == username {
			s.state.Websites[i].Owner = "aura"
			s.state.Websites[i].User = "aura"
		}
	}
	s.recountSitesLocked()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "User deleted."})
}

func (s *service) handleUsersChangePassword(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Username    string `json:"username"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid password payload.")
		return
	}
	if strings.TrimSpace(payload.NewPassword) == "" {
		writeError(w, http.StatusBadRequest, "New password is required.")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user := s.findUserLocked(sanitizeName(payload.Username))
	if user == nil {
		writeError(w, http.StatusNotFound, "User not found.")
		return
	}
	user.PasswordHash = mustHashPassword(payload.NewPassword)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Password updated."})
}

func (s *service) handlePackagesList(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.Packages})
}

func (s *service) handlePackagesCreate(w http.ResponseWriter, r *http.Request) {
	var payload Package
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid package payload.")
		return
	}
	if strings.TrimSpace(payload.Name) == "" {
		writeError(w, http.StatusBadRequest, "Package name is required.")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	payload.ID = s.state.NextPackageID
	s.state.NextPackageID++
	payload.PlanType = normalizePlanType(payload.PlanType)
	s.state.Packages = append(s.state.Packages, payload)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Package created.", Data: payload})
}

func (s *service) handlePackagesUpdate(w http.ResponseWriter, r *http.Request) {
	var payload Package
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid package payload.")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.state.Packages {
		if s.state.Packages[i].ID == payload.ID {
			payload.PlanType = normalizePlanType(payload.PlanType)
			s.state.Packages[i] = payload
			s.refreshPackageQuotasLocked(payload.Name)
			writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Package updated.", Data: payload})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Package not found.")
}

func (s *service) handlePackagesDelete(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID int `json:"id"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid package delete payload.")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.state.Packages {
		if s.state.Packages[i].ID == payload.ID {
			name := s.state.Packages[i].Name
			s.state.Packages = append(s.state.Packages[:i], s.state.Packages[i+1:]...)
			for j := range s.state.Websites {
				if s.state.Websites[j].Package == name {
					s.state.Websites[j].Package = "default"
					s.state.Websites[j].Quota = quotaForPackage(s.state.Packages, "default")
				}
			}
			writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Package deleted."})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Package not found.")
}

func (s *service) handleDatabaseList(w http.ResponseWriter, engine string) {
	records, err := runtimeDatabaseList(engine)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.syncRuntimeDatabaseStateLocked(engine); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if engine == "mariadb" {
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.MariaDBs})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: records})
}

func (s *service) handleDatabaseUsers(w http.ResponseWriter, engine string) {
	if _, err := runtimeDatabaseUsers(engine); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.syncRuntimeDatabaseStateLocked(engine); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if engine == "mariadb" {
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: publicDBUsers(s.state.MariaUsers)})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: publicDBUsers(s.state.PostgresUsers)})
}

func (s *service) handleRemoteAccessList(w http.ResponseWriter, engine string) {
	if _, err := runtimeRemoteAccessList(engine); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.syncRuntimeDatabaseStateLocked(engine); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if engine == "mariadb" {
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.MariaRemoteRules})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.PostgresRemoteRules})
}

func (s *service) handleDatabaseCreate(w http.ResponseWriter, r *http.Request, engine string) {
	var payload struct {
		DBName     string `json:"db_name"`
		DBUser     string `json:"db_user"`
		DBPass     string `json:"db_pass"`
		SiteDomain string `json:"site_domain"`
		Owner      string `json:"owner"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid database payload.")
		return
	}
	if strings.TrimSpace(payload.DBName) == "" || strings.TrimSpace(payload.DBUser) == "" {
		writeError(w, http.StatusBadRequest, "DB name and DB user are required.")
		return
	}
	dbPass := firstNonEmpty(strings.TrimSpace(payload.DBPass), generateSecret(16))
	dbName := sanitizeDBName(payload.DBName)
	dbUser := sanitizeDBName(payload.DBUser)
	if err := createRuntimeDatabase(engine, dbName, dbUser, dbPass); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	db := DatabaseRecord{
		Name:       dbName,
		Size:       "0 MB",
		Tables:     0,
		Engine:     engine,
		Owner:      firstNonEmpty(strings.TrimSpace(payload.Owner), "aura"),
		SiteDomain: normalizeDomain(payload.SiteDomain),
	}
	user := DatabaseUser{
		Username:     dbUser,
		Host:         "localhost",
		Engine:       engine,
		LinkedDBName: db.Name,
		PasswordHash: mustHashPassword(dbPass),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if engine == "mariadb" {
		s.state.MariaDBs = append(removeDatabaseByName(s.state.MariaDBs, db.Name), db)
		s.state.MariaUsers = append(removeDatabaseUserByUsername(s.state.MariaUsers, user.Username), user)
	} else {
		s.state.PostgresDBs = append(removeDatabaseByName(s.state.PostgresDBs, db.Name), db)
		s.state.PostgresUsers = append(removeDatabaseUserByUsername(s.state.PostgresUsers, user.Username), user)
	}

	if db.SiteDomain != "" {
		s.state.DBLinks = append(removeDBLinksByDBName(s.state.DBLinks, db.Name), WebsiteDBLink{
			Domain:   db.SiteDomain,
			Engine:   engine,
			DBName:   db.Name,
			DBUser:   user.Username,
			LinkedAt: time.Now().UTC().Unix(),
		})
	}
	_ = s.syncRuntimeDatabaseStateLocked(engine)

	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"db_name": db.Name,
			"db_user": user.Username,
			"db_pass": dbPass,
			"engine":  engine,
		},
	})
}

func (s *service) handleDatabaseDrop(w http.ResponseWriter, r *http.Request, engine string) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid DB drop payload.")
		return
	}

	target := sanitizeDBName(payload.Name)
	if err := dropRuntimeDatabase(engine, target); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if engine == "mariadb" {
		s.state.MariaDBs = removeDatabaseByName(s.state.MariaDBs, target)
		s.state.MariaUsers = removeDatabaseUsersByDBName(s.state.MariaUsers, target)
		s.state.MariaRemoteRules = removeRemoteRulesByDBName(s.state.MariaRemoteRules, target)
	} else {
		s.state.PostgresDBs = removeDatabaseByName(s.state.PostgresDBs, target)
		s.state.PostgresUsers = removeDatabaseUsersByDBName(s.state.PostgresUsers, target)
		s.state.PostgresRemoteRules = removeRemoteRulesByDBName(s.state.PostgresRemoteRules, target)
	}
	s.state.DBLinks = removeDBLinksByDBName(s.state.DBLinks, target)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Database deleted."})
}

func (s *service) handleDatabasePasswordUpdate(w http.ResponseWriter, r *http.Request, engine string) {
	var payload struct {
		DBUser      string `json:"db_user"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid password payload.")
		return
	}
	if strings.TrimSpace(payload.NewPassword) == "" {
		writeError(w, http.StatusBadRequest, "New password is required.")
		return
	}

	target := sanitizeDBName(payload.DBUser)
	if err := updateRuntimeDatabasePassword(engine, target, payload.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	users := &s.state.MariaUsers
	if engine != "mariadb" {
		users = &s.state.PostgresUsers
	}
	for i := range *users {
		if (*users)[i].Username == target {
			(*users)[i].PasswordHash = mustHashPassword(payload.NewPassword)
			writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Database password updated."})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Database user not found.")
}

func (s *service) handleRemoteAccessCreate(w http.ResponseWriter, r *http.Request, engine string) {
	var payload struct {
		DBUser   string `json:"db_user"`
		DBName   string `json:"db_name"`
		RemoteIP string `json:"remote_ip"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid remote access payload.")
		return
	}
	if strings.TrimSpace(payload.DBUser) == "" || strings.TrimSpace(payload.DBName) == "" || strings.TrimSpace(payload.RemoteIP) == "" {
		writeError(w, http.StatusBadRequest, "db_user, db_name and remote_ip are required.")
		return
	}

	rule := RemoteAccessRule{
		Engine:     engine,
		DBUser:     sanitizeDBName(payload.DBUser),
		DBName:     sanitizeDBName(payload.DBName),
		Remote:     strings.TrimSpace(payload.RemoteIP),
		AuthMethod: authMethodForEngine(engine),
	}
	if err := grantRuntimeRemoteAccess(engine, rule.DBUser, rule.DBName, rule.Remote); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if engine == "mariadb" {
		s.state.MariaRemoteRules = append(s.state.MariaRemoteRules, rule)
	} else {
		s.state.PostgresRemoteRules = append(s.state.PostgresRemoteRules, rule)
	}
	_ = s.syncRuntimeDatabaseStateLocked(engine)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Remote access granted.", Data: rule})
}

func (s *service) handleSubdomainList(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.Subdomains})
}

func (s *service) handleSubdomainCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ParentDomain string `json:"parent_domain"`
		Subdomain    string `json:"subdomain"`
		PHPVersion   string `json:"php_version"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid subdomain payload.")
		return
	}
	parent := normalizeDomain(payload.ParentDomain)
	sub := sanitizeName(payload.Subdomain)
	if parent == "" || sub == "" {
		writeError(w, http.StatusBadRequest, "Parent domain and subdomain are required.")
		return
	}

	fqdn := fmt.Sprintf("%s.%s", sub, parent)
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range s.state.Subdomains {
		if item.FQDN == fqdn {
			writeError(w, http.StatusConflict, "Subdomain already exists.")
			return
		}
	}

	entry := Subdomain{
		FQDN:         fqdn,
		ParentDomain: parent,
		PHPVersion:   firstNonEmpty(strings.TrimSpace(payload.PHPVersion), "8.3"),
		SSLEnabled:   true,
		CreatedAt:    time.Now().UTC().Unix(),
	}
	s.state.Subdomains = append(s.state.Subdomains, entry)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Subdomain created.", Data: entry})
}

func (s *service) handleDBLinksList(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.DBLinks})
}

func (s *service) handleDBLinksCreate(w http.ResponseWriter, r *http.Request) {
	var payload WebsiteDBLink
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid DB link payload.")
		return
	}
	payload.Domain = normalizeDomain(payload.Domain)
	if payload.Domain == "" || payload.DBName == "" || payload.DBUser == "" {
		writeError(w, http.StatusBadRequest, "Domain, db name and db user are required.")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	payload.Engine = normalizeEngine(payload.Engine)
	payload.LinkedAt = time.Now().UTC().Unix()
	s.state.DBLinks = append(s.state.DBLinks, payload)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "DB link created.", Data: payload})
}

func (s *service) handleDBLinksDelete(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	engine := normalizeEngine(r.URL.Query().Get("engine"))
	dbName := sanitizeDBName(r.URL.Query().Get("db_name"))
	if domain == "" || dbName == "" {
		writeError(w, http.StatusBadRequest, "Domain and database name are required.")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := s.state.DBLinks[:0]
	removed := false
	for _, item := range s.state.DBLinks {
		sameEngine := engine == "" || normalizeEngine(item.Engine) == engine
		if !removed && item.Domain == domain && item.DBName == dbName && sameEngine {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.state.DBLinks = filtered
	if !removed {
		writeError(w, http.StatusNotFound, "DB link not found.")
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "DB link removed."})
}

func (s *service) handleAliasesList(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	s.mu.RLock()
	defer s.mu.RUnlock()

	var items []DomainAlias
	for _, alias := range s.state.Aliases {
		if domain == "" || alias.Domain == domain {
			items = append(items, alias)
		}
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
}

func (s *service) handleMetrics(w http.ResponseWriter) {
	metrics := collectHostMetrics(s.startedAt)

	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"cpu_usage":      metrics.CPUUsage,
			"cpu_cores":      metrics.CPUCores,
			"cpu_model":      metrics.CPUModel,
			"ram_usage":      metrics.RAMUsage,
			"ram_used":       metrics.RAMUsed,
			"ram_total":      metrics.RAMTotal,
			"disk_usage":     metrics.DiskUsage,
			"disk_used":      metrics.DiskUsed,
			"disk_total":     metrics.DiskTotal,
			"uptime_seconds": metrics.UptimeSeconds,
			"uptime_human":   metrics.UptimeHuman,
			"load_avg":       metrics.LoadAvg,
		},
	})
}

func (s *service) handleServices(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: collectHostServices()})
}

func (s *service) handleProcesses(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: collectHostProcesses(20)})
}

func (s *service) handleServiceControl(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name   string `json:"name"`
		Action string `json:"action"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid service control payload.")
		return
	}

	name := strings.TrimSpace(payload.Name)
	action := strings.ToLower(strings.TrimSpace(payload.Action))
	if action == "kill" {
		pid, _ := strconv.Atoi(name)
		if err := terminateProcess(pid); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Process terminated."})
		return
	}

	switch action {
	case "start", "restart", "stop":
	default:
		writeError(w, http.StatusBadRequest, "Unsupported action.")
		return
	}
	if err := executeServiceAction(name, action); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Service action applied."})
}

func (s *service) handlePanelPortGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"current_port": s.state.GatewayPort,
			"gateway_addr": fmt.Sprintf(":%d", s.state.GatewayPort),
		},
	})
}

func (s *service) handlePanelPortSet(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Port         int  `json:"port"`
		OpenFirewall bool `json:"open_firewall"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid panel port payload.")
		return
	}
	if payload.Port < 1 || payload.Port > 65535 {
		writeError(w, http.StatusBadRequest, "Port must be between 1 and 65535.")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.GatewayPort = payload.Port

	firewallActions := []string{"Gateway restart requested"}
	if payload.OpenFirewall {
		if err := openFirewallPort(payload.Port); err == nil {
			firewallActions = append([]string{fmt.Sprintf("Allow tcp/%d on firewall", payload.Port)}, firewallActions...)
		}
	}
	_ = writeEnvFileValues("/etc/aurapanel/aurapanel.env", map[string]string{
		"AURAPANEL_GATEWAY_ADDR": fmt.Sprintf(":%d", payload.Port),
	})

	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: "Gateway port updated.",
		Data: map[string]interface{}{
			"gateway_addr":      fmt.Sprintf(":%d", payload.Port),
			"firewall_actions":  firewallActions,
			"warnings":          []string{"Manual process supervisor reload is still required."},
			"restart_scheduled": true,
		},
	})
}

func (s *service) handleSecurityStatus(w http.ResponseWriter) {
	snapshot := collectSecuritySnapshot()
	twoFAEnabled := false
	s.mu.RLock()
	for _, user := range s.state.Users {
		if user.TwoFAEnabled {
			twoFAEnabled = true
			break
		}
	}
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"ebpf_monitoring":          snapshot.EBPFMonitoring,
			"ml_waf":                   snapshot.MLWAFActive,
			"totp_2fa":                 twoFAEnabled,
			"wireguard_federation":     snapshot.WireGuardActive,
			"immutable_os_support":     snapshot.ImmutableOS,
			"live_patching":            snapshot.LivePatchingActive,
			"one_click_hardening":      snapshot.OneClickHardening,
			"nft_firewall":             snapshot.FirewallActive,
			"ssh_key_manager":          snapshot.SSHKeyManager,
			"firewall_active":          snapshot.FirewallActive,
			"firewall_manager":         snapshot.FirewallManager,
			"firewall_open_ports":      snapshot.FirewallOpenPorts,
			"apache_backend_available": snapshot.ApacheBackendAvailable,
			"mail_domain_available":    snapshot.MailDomainAvailable,
			"detected_mail_stack":      snapshot.DetectedMailStack,
			"detected_web_stack":       snapshot.DetectedWebStack,
			"server_ip":                snapshot.ServerIP,
		},
	})
}

func (s *service) handleEBPFEvents(w http.ResponseWriter) {
	s.mu.RLock()
	events := append([]string(nil), s.state.EBPFEvents...)
	s.mu.RUnlock()
	if len(events) == 0 {
		events = collectEBPFStatusLines()
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: events})
}

func (s *service) handleCollectEBPF(w http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.EBPFEvents = append(collectEBPFStatusLines(), s.state.EBPFEvents...)
	if len(s.state.EBPFEvents) > 20 {
		s.state.EBPFEvents = s.state.EBPFEvents[:20]
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Telemetry snapshot collected."})
}

func (s *service) handleFirewallRulesList(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: listFirewallRuntimeRules()})
}

func (s *service) handleFirewallRuleCreate(w http.ResponseWriter, r *http.Request) {
	var payload FirewallRule
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid firewall payload.")
		return
	}
	if strings.TrimSpace(payload.IPAddress) == "" {
		writeError(w, http.StatusBadRequest, "IP address is required.")
		return
	}

	if err := addFirewallRuntimeRule(payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Firewall rule added."})
}

func (s *service) handleFirewallRuleDelete(w http.ResponseWriter, r *http.Request) {
	ip := strings.TrimSpace(r.URL.Query().Get("ip_address"))
	if err := deleteFirewallRuntimeRule(ip); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Firewall rule deleted."})
}

func (s *service) handleSSHKeysList(w http.ResponseWriter, r *http.Request) {
	user := strings.TrimSpace(r.URL.Query().Get("user"))
	if user == "" {
		user = "root"
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: listAuthorizedKeys(user)})
}

func (s *service) handleSSHKeyCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		User      string `json:"user"`
		Title     string `json:"title"`
		PublicKey string `json:"public_key"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid SSH key payload.")
		return
	}
	if strings.TrimSpace(payload.User) == "" || strings.TrimSpace(payload.PublicKey) == "" {
		writeError(w, http.StatusBadRequest, "User and public key are required.")
		return
	}

	key, err := addAuthorizedKey(strings.TrimSpace(payload.User), strings.TrimSpace(payload.Title), strings.TrimSpace(payload.PublicKey))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "SSH key added.", Data: key})
}

func (s *service) handleSSHKeyDelete(w http.ResponseWriter, r *http.Request) {
	user := strings.TrimSpace(r.URL.Query().Get("user"))
	keyID := strings.TrimSpace(r.URL.Query().Get("key_id"))
	if user == "" {
		user = "root"
	}
	if err := deleteAuthorizedKey(user, keyID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "SSH key removed."})
}

func (s *service) handleHardeningApply(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Stack  string `json:"stack"`
		Domain string `json:"domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid hardening payload.")
		return
	}
	applied, err := applySystemHardeningProfile(firstNonEmpty(payload.Stack, "generic"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"domain":        payload.Domain,
			"applied_rules": applied,
		},
	})
}

func (s *service) handleSiteLogs(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	kind := firstNonEmpty(strings.TrimSpace(r.URL.Query().Get("kind")), "access")
	if domain == "" {
		writeError(w, http.StatusBadRequest, "Domain is required.")
		return
	}
	lines, err := realSiteLogs(domain, kind)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: lines})
}

func (s *service) handleSSLIssue(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string `json:"domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid SSL payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	domains := []string{domain}
	if normalizeDomain(domain) != "" {
		domains = append(domains, "www."+domain)
	}

	// Ensure docroot exists before issuing SSL
	docroot := domainDocroot(domain)
	if err := os.MkdirAll(docroot, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create docroot for SSL validation.")
		return
	}

	// Ensure user exists and owns the directory
	s.mu.Lock()
	siteOwner := "aura"
	if site := s.findWebsiteLocked(domain); site != nil {
		siteOwner = firstNonEmpty(site.Owner, site.User, "aura")
	}
	s.mu.Unlock()

	_ = exec.Command("chown", "-R", siteOwner+":"+siteOwner, filepath.Dir(docroot)).Run()

	// Pre-sync OpenLiteSpeed configs so it can serve the acme-challenge before issuing the SSL
	s.mu.Lock()
	_ = s.syncOLSVhostsLocked()
	s.mu.Unlock()

	if err := issueLetsEncryptCertificate(domains, docroot, false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if site := s.findWebsiteLocked(domain); site != nil {
		site.SSL = true
	}
	s.modules.SSLCertificates[domain] = inspectCertificate(domain)
	if err := s.syncOLSVhostsLocked(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("SSL issued for %s.", domain)})
}

func (s *service) setWebsiteStatus(w http.ResponseWriter, r *http.Request, status string) {
	var payload struct {
		Domain string `json:"domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid website status payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	site := s.findWebsiteLocked(normalizeDomain(payload.Domain))
	if site == nil {
		writeError(w, http.StatusNotFound, "Website not found.")
		return
	}
	site.Status = status
	if err := s.syncOLSVhostsLocked(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Website status updated.", Data: site})
}

func (s *service) handleFallback(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, apiResponse{
		Status:  "error",
		Message: "Endpoint has no real runtime integration yet.",
		Data: map[string]interface{}{
			"path":   r.URL.Path,
			"method": r.Method,
		},
	})
}

func (s *service) findWebsiteLocked(domain string) *Website {
	for i := range s.state.Websites {
		if s.state.Websites[i].Domain == domain {
			return &s.state.Websites[i]
		}
	}
	return nil
}

func (s *service) findWebsiteIndexLocked(domain string) int {
	for i := range s.state.Websites {
		if s.state.Websites[i].Domain == domain {
			return i
		}
	}
	return -1
}

func (s *service) findUserLocked(username string) *PanelUser {
	for i := range s.state.Users {
		if s.state.Users[i].Username == username {
			return &s.state.Users[i]
		}
	}
	return nil
}

func (s *service) findUserIndexLocked(username string) int {
	for i := range s.state.Users {
		if s.state.Users[i].Username == username {
			return i
		}
	}
	return -1
}

func (s *service) ensureUserLocked(username, email, role, pkg, password string) {
	key := sanitizeName(username)
	if key == "" {
		return
	}
	if existing := s.findUserLocked(key); existing != nil {
		if existing.Email == "" {
			existing.Email = email
		}
		if existing.Package == "" {
			existing.Package = pkg
		}
		return
	}

	s.state.Users = append(s.state.Users, PanelUser{
		ID:           s.state.NextUserID,
		Username:     key,
		Name:         strings.Title(key),
		Email:        email,
		Role:         normalizeRole(role),
		Package:      firstNonEmpty(pkg, "default"),
		Active:       true,
		PasswordHash: mustHashPassword(firstNonEmpty(password, "password123")),
	})
	s.state.NextUserID++
}

func (s *service) recountSitesLocked() {
	counts := map[string]int{}
	for _, site := range s.state.Websites {
		key := firstNonEmpty(site.Owner, site.User)
		if key != "" {
			counts[key]++
		}
	}
	for i := range s.state.Users {
		s.state.Users[i].Sites = counts[s.state.Users[i].Username]
	}
}

func (s *service) refreshPackageQuotasLocked(packageName string) {
	for i := range s.state.Websites {
		if s.state.Websites[i].Package == packageName {
			s.state.Websites[i].Quota = quotaForPackage(s.state.Packages, packageName)
		}
	}
}

func (s *service) ensureDefaultSiteArtifactsLocked(domain string) {
	key := normalizeDomain(domain)
	if key == "" {
		return
	}
	if _, ok := s.state.AdvancedConfig[key]; !ok {
		s.state.AdvancedConfig[key] = defaultWebsiteAdvancedConfig()
	}
}

func (s *service) provisionWebsiteArtifactsLocked(site Website) error {
	s.ensureDefaultSiteArtifactsLocked(site.Domain)
	s.ensureDNSArtifactsLocked(site.Domain, site.MailDomain)
	if site.MailDomain {
		s.ensureMailArtifactsLocked(site)
	}
	_ = s.syncCloudflareZoneRecordsLocked(site.Domain)
	return s.syncOLSVhostsLocked()
}

func (s *service) ensureDNSArtifactsLocked(domain string, mailDomain bool) {
	normalizedDomain := normalizeDomain(domain)
	if normalizedDomain == "" {
		return
	}

	serverIP := detectPrimaryIPv4()
	if serverIP == "" {
		serverIP = "127.0.0.1"
	}

	zoneIndex := -1
	for i := range s.modules.DNSZones {
		if s.modules.DNSZones[i].Name == normalizedDomain {
			zoneIndex = i
			break
		}
	}
	if zoneIndex == -1 {
		s.modules.DNSZones = append(s.modules.DNSZones, DNSZone{
			ID:            generateSecret(6),
			Name:          normalizedDomain,
			Kind:          "native",
			Records:       0,
			DNSSECEnabled: false,
		})
	}

	if s.modules.DNSRecords == nil {
		s.modules.DNSRecords = map[string][]DNSRecord{}
	}
	if _, ok := s.modules.DNSRecords[normalizedDomain]; !ok {
		s.modules.DNSRecords[normalizedDomain] = []DNSRecord{}
	}

	s.upsertDNSRecordLocked(normalizedDomain, DNSRecord{RecordType: "A", Name: normalizedDomain, Content: serverIP, TTL: 3600})
	s.upsertDNSRecordLocked(normalizedDomain, DNSRecord{RecordType: "A", Name: "www", Content: serverIP, TTL: 3600})
	s.upsertDNSRecordLocked(normalizedDomain, DNSRecord{RecordType: "A", Name: "panel", Content: serverIP, TTL: 3600})
	s.upsertDNSRecordLocked(normalizedDomain, DNSRecord{RecordType: "TXT", Name: normalizedDomain, Content: "v=spf1 mx a ~all", TTL: 3600})

	if mailDomain {
		s.upsertDNSRecordLocked(normalizedDomain, DNSRecord{RecordType: "A", Name: "mail", Content: serverIP, TTL: 3600})
		s.upsertDNSRecordLocked(normalizedDomain, DNSRecord{RecordType: "MX", Name: normalizedDomain, Content: fmt.Sprintf("mail.%s", normalizedDomain), TTL: 3600})
	}

	s.recalcDNSZoneLocked(normalizedDomain)
	if s.modules.DefaultNameservers.NS1 == "" && s.modules.DefaultNameservers.NS2 == "" {
		s.modules.DefaultNameservers = DefaultNameservers{
			NS1: fmt.Sprintf("ns1.%s", normalizedDomain),
			NS2: fmt.Sprintf("ns2.%s", normalizedDomain),
		}
	}
}

func (s *service) upsertDNSRecordLocked(domain string, record DNSRecord) {
	items := s.modules.DNSRecords[domain]
	for i := range items {
		if strings.EqualFold(items[i].RecordType, record.RecordType) && items[i].Name == record.Name {
			items[i].Content = record.Content
			items[i].TTL = record.TTL
			s.modules.DNSRecords[domain] = items
			return
		}
	}
	s.modules.DNSRecords[domain] = append(items, record)
}

func (s *service) ensureMailArtifactsLocked(site Website) {
	normalizedDomain := normalizeDomain(site.Domain)
	if normalizedDomain == "" {
		return
	}

	if s.modules.MailCatchAll == nil {
		s.modules.MailCatchAll = map[string]MailCatchAll{}
	}
	if _, ok := s.modules.MailCatchAll[normalizedDomain]; !ok {
		s.modules.MailCatchAll[normalizedDomain] = MailCatchAll{
			Domain:  normalizedDomain,
			Enabled: false,
			Target:  fmt.Sprintf("postmaster@%s", normalizedDomain),
		}
	}

	if s.modules.MailDKIM == nil {
		s.modules.MailDKIM = map[string]DKIMRecord{}
	}
	if _, ok := s.modules.MailDKIM[normalizedDomain]; !ok {
		s.modules.MailDKIM[normalizedDomain] = DKIMRecord{
			Domain:    normalizedDomain,
			Selector:  "selector1",
			PublicKey: fmt.Sprintf("v=DKIM1; k=rsa; p=%s", generateSecret(48)),
		}
	}

	_ = provisionMailDomain(normalizedDomain)
	s.recordIssuedCertificateLocked(fmt.Sprintf("mail.%s", normalizedDomain), "Let's Encrypt", false)
}

func (s *service) removeDNSArtifactsLocked(domain string) {
	filteredZones := s.modules.DNSZones[:0]
	for _, zone := range s.modules.DNSZones {
		if zone.Name != domain {
			filteredZones = append(filteredZones, zone)
		}
	}
	s.modules.DNSZones = filteredZones
	delete(s.modules.DNSRecords, domain)
}

func (s *service) removeMailArtifactsLocked(domain string) {
	delete(s.modules.MailCatchAll, domain)
	delete(s.modules.MailDKIM, domain)

	// Remove mailboxes associated with this domain
	filteredMailboxes := s.modules.Mailboxes[:0]
	for _, mb := range s.modules.Mailboxes {
		if !strings.HasSuffix(mb.Address, "@"+domain) {
			filteredMailboxes = append(filteredMailboxes, mb)
		} else {
			_ = deleteSystemMailbox(mb.Address)
		}
	}
	s.modules.Mailboxes = filteredMailboxes

	// Remove forwarders
	filteredForwards := s.modules.MailForwards[:0]
	for _, fw := range s.modules.MailForwards {
		if fw.Domain != domain {
			filteredForwards = append(filteredForwards, fw)
		} else {
			_ = deleteSystemForward(fw.Domain, fw.Source)
		}
	}
	s.modules.MailForwards = filteredForwards

	// Remove routing
	filteredRouting := s.modules.MailRouting[:0]
	for _, rt := range s.modules.MailRouting {
		if rt.Domain != domain {
			filteredRouting = append(filteredRouting, rt)
		}
	}
	s.modules.MailRouting = filteredRouting

	// Remove physical mail directory if using standard postfix/dovecot path
	_ = exec.Command("rm", "-rf", fmt.Sprintf("/var/vmail/%s", domain)).Run()
}

func (s *service) removeSiteArtifactsLocked(domain string) error {
	delete(s.state.AdvancedConfig, domain)
	delete(s.state.CustomSSL, domain)
	s.state.Aliases = removeAliasesByDomain(s.state.Aliases, domain)
	s.state.Subdomains = removeSubdomainsByParent(s.state.Subdomains, domain)
	s.state.DBLinks = removeDBLinksByDomain(s.state.DBLinks, domain)

	// Remove DNS Zones and Records
	s.removeDNSArtifactsLocked(domain)

	// Remove Mail Configurations and Directories
	s.removeMailArtifactsLocked(domain)

	// Remove physical document root directory
	docroot := domainDocroot(domain)
	_ = exec.Command("rm", "-rf", docroot).Run()

	// Ensure we remove the user's home directory if it's completely empty and user only had one site
	homeDir := fmt.Sprintf("/home/%s", domain)
	if docroot != homeDir {
		_ = exec.Command("rm", "-rf", homeDir).Run()
	}

	return s.syncOLSVhostsLocked()
}

func issueToken(user PanelUser) (string, error) {
	now := time.Now().UTC()
	claims := jwtClaims{
		Email: user.Email,
		Name:  firstNonEmpty(user.Name, user.Username),
		Role:  normalizeRole(user.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.Email,
			Issuer:    envOr("AURAPANEL_JWT_ISSUER", "aurapanel-gateway"),
			Audience:  jwt.ClaimStrings{envOr("AURAPANEL_JWT_AUDIENCE", "aurapanel-ui")},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(12 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret()))
}

func jwtSecret() string {
	return envOr("AURAPANEL_JWT_SECRET", "aurapanel_dev_only_secret_change_me")
}

func publicUsers(users []PanelUser) []PanelUser {
	out := make([]PanelUser, 0, len(users))
	for _, user := range users {
		out = append(out, publicUser(user))
	}
	return out
}

func publicUser(user PanelUser) PanelUser {
	user.PasswordHash = ""
	return user
}

func publicDBUsers(users []DatabaseUser) []DatabaseUser {
	out := make([]DatabaseUser, 0, len(users))
	for _, user := range users {
		user.PasswordHash = ""
		out = append(out, user)
	}
	return out
}

func quotaForPackage(packages []Package, packageName string) string {
	for _, pkg := range packages {
		if pkg.Name == packageName {
			if pkg.DiskGB <= 0 {
				return "Unlimited"
			}
			return fmt.Sprintf("%d GB", pkg.DiskGB)
		}
	}
	return "10 GB"
}

func authMethodForEngine(engine string) string {
	if engine == "mariadb" {
		return "password"
	}
	return "scram-sha-256"
}

func normalizeEngine(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "postgres", "postgresql":
		return "postgresql"
	default:
		return "mariadb"
	}
}

func normalizeRole(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "admin":
		return "admin"
	case "reseller":
		return "reseller"
	default:
		return "user"
	}
}

func normalizePlanType(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "reseller") {
		return "reseller"
	}
	return "hosting"
}

func normalizeDomain(value string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(value)), ".")
}

func sanitizeName(value string) string {
	cleaned := strings.ToLower(strings.TrimSpace(value))
	cleaned = strings.ReplaceAll(cleaned, " ", "_")
	cleaned = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return -1
	}, cleaned)
	return cleaned
}

func sanitizeDBName(value string) string {
	cleaned := strings.TrimSpace(strings.ToLower(value))
	cleaned = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, cleaned)
	cleaned = strings.Trim(cleaned, "_")
	return firstNonEmpty(cleaned, "database")
}

func envOr(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func readEnvFileValue(path, key string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	prefix := key + "="
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		}
	}

	return ""
}

func writeEnvFileValues(path string, updates map[string]string) error {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	lines := []string{}
	if len(raw) > 0 {
		lines = strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	}

	seen := map[string]bool{}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "=") {
			continue
		}
		key := strings.TrimSpace(strings.SplitN(trimmed, "=", 2)[0])
		if _, ok := updates[key]; !ok {
			continue
		}
		lines[i] = key + "=" + updates[key]
		seen[key] = true
	}

	keys := make([]string, 0, len(updates))
	for key := range updates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if seen[key] {
			continue
		}
		lines = append(lines, key+"="+updates[key])
	}

	content := strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
	tempPath := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(tempPath, []byte(content), 0o600); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func readTrimmedFile(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func loadAdminSeedCredentials() (string, string) {
	const (
		gatewayEnvPath      = "/etc/aurapanel/aurapanel.env"
		initialPasswordPath = "/opt/aurapanel/logs/initial_password.txt"
		defaultAdminEmail   = "admin@server.com"
		defaultAdminPass    = "password123"
	)

	adminEmail := firstNonEmpty(
		strings.TrimSpace(os.Getenv("AURAPANEL_ADMIN_EMAIL")),
		readEnvFileValue(gatewayEnvPath, "AURAPANEL_ADMIN_EMAIL"),
		defaultAdminEmail,
	)

	adminHash := firstNonEmpty(
		strings.TrimSpace(os.Getenv("AURAPANEL_ADMIN_PASSWORD_BCRYPT")),
		readEnvFileValue(gatewayEnvPath, "AURAPANEL_ADMIN_PASSWORD_BCRYPT"),
	)
	if adminHash != "" {
		return adminEmail, adminHash
	}

	adminPassword := firstNonEmpty(
		strings.TrimSpace(os.Getenv("AURAPANEL_ADMIN_PASSWORD")),
		readEnvFileValue(gatewayEnvPath, "AURAPANEL_ADMIN_PASSWORD"),
		readTrimmedFile(initialPasswordPath),
		defaultAdminPass,
	)

	return adminEmail, mustHashPassword(adminPassword)
}

func decodeJSON(r *http.Request, dst interface{}) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiResponse{
		Status:  "error",
		Message: message,
	})
}

func mustHashPassword(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(hash)
}

func generateSecret(length int) string {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return strings.TrimRight(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), "=")
}

func queryInt(r *http.Request, key string, fallback int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func maxInt(values ...int) int {
	result := values[0]
	for _, value := range values[1:] {
		if value > result {
			result = value
		}
	}
	return result
}

func minInt(values ...int) int {
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func removeDatabaseByName(items []DatabaseRecord, name string) []DatabaseRecord {
	filtered := items[:0]
	for _, item := range items {
		if item.Name != name {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func removeDatabaseUsersByDBName(items []DatabaseUser, dbName string) []DatabaseUser {
	filtered := items[:0]
	for _, item := range items {
		if item.LinkedDBName != dbName {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func removeDatabaseUserByUsername(items []DatabaseUser, username string) []DatabaseUser {
	filtered := items[:0]
	for _, item := range items {
		if item.Username != username {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func removeRemoteRulesByDBName(items []RemoteAccessRule, dbName string) []RemoteAccessRule {
	filtered := items[:0]
	for _, item := range items {
		if item.DBName != dbName {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func removeDBLinksByDBName(items []WebsiteDBLink, dbName string) []WebsiteDBLink {
	filtered := items[:0]
	for _, item := range items {
		if item.DBName != dbName {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func removeDBLinksByDomain(items []WebsiteDBLink, domain string) []WebsiteDBLink {
	filtered := items[:0]
	for _, item := range items {
		if item.Domain != domain {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func removeTransferAccountByUsername(items []TransferAccount, username string) []TransferAccount {
	filtered := items[:0]
	for _, item := range items {
		if item.Username != username {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func removeCronJobByID(items []CronJob, id string) []CronJob {
	filtered := items[:0]
	for _, item := range items {
		if item.ID != id {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func removeAliasesByDomain(items []DomainAlias, domain string) []DomainAlias {
	filtered := items[:0]
	for _, item := range items {
		if item.Domain != domain {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func removeSubdomainsByParent(items []Subdomain, domain string) []Subdomain {
	filtered := items[:0]
	for _, item := range items {
		if item.ParentDomain != domain {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func fallbackPayloadForPath(path string) interface{} {
	switch {
	case strings.Contains(path, "/status/"), strings.Contains(path, "/config"), strings.Contains(path, "/settings"), strings.Contains(path, "/mode"), strings.Contains(path, "/detail"):
		return map[string]interface{}{}
	case strings.Contains(path, "/list"), strings.Contains(path, "/zones"), strings.Contains(path, "/records"), strings.Contains(path, "/rules"), strings.Contains(path, "/logs"), strings.Contains(path, "/jobs"), strings.Contains(path, "/services"), strings.Contains(path, "/processes"), strings.Contains(path, "/backups"), strings.Contains(path, "/packages"), strings.Contains(path, "/policies"), strings.Contains(path, "/assignments"), strings.Contains(path, "/buckets"):
		return []interface{}{}
	default:
		return map[string]interface{}{}
	}
}
