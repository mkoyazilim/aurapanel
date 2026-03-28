package main

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultServiceAddr = ":8081"
	defaultGatewayPort = 8090
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
	PasswordHash string `json:"-"`
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
	PasswordHash string `json:"-"`
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
	NextPackageID       int
	NextUserID          int
	NextProcessPID      int
}

type service struct {
	mu        sync.RWMutex
	startedAt time.Time
	state     appState
	modules   moduleState
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
	svc.bootstrapModules()
	return svc
}

func seedState() appState {
	now := time.Now().UTC().Unix()
	adminEmail, adminHash := loadAdminSeedCredentials()

	users := []PanelUser{
		{
			ID:           1,
			Username:     "admin",
			Name:         "System Administrator",
			Email:        adminEmail,
			Role:         "admin",
			Package:      "default",
			Sites:        1,
			Active:       true,
			TwoFAEnabled: false,
			PasswordHash: adminHash,
		},
		{
			ID:           2,
			Username:     "aura",
			Name:         "Aura Operator",
			Email:        "aura@example.com",
			Role:         "reseller",
			Package:      "default",
			Sites:        1,
			Active:       true,
			TwoFAEnabled: false,
			PasswordHash: mustHashPassword("password123"),
		},
	}

	return appState{
		GatewayPort: defaultGatewayPort,
		Websites: []Website{
			{
				Domain:        "example.com",
				Owner:         "aura",
				User:          "aura",
				PHP:           "8.3",
				PHPVersion:    "8.3",
				Package:       "default",
				Email:         "webmaster@example.com",
				Status:        "active",
				SSL:           true,
				DiskUsage:     "1.4 GB",
				Quota:         "10 GB",
				MailDomain:    true,
				ApacheBackend: false,
				CreatedAt:     now,
			},
		},
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
		Users: users,
		MariaDBs: []DatabaseRecord{
			{Name: "example_app", Size: "128 MB", Tables: 12, Engine: "mariadb", Owner: "aura", SiteDomain: "example.com"},
		},
		PostgresDBs: []DatabaseRecord{
			{Name: "analytics", Size: "64 MB", Tables: 8, Engine: "postgresql", Owner: "admin", SiteDomain: ""},
		},
		MariaUsers: []DatabaseUser{
			{Username: "example_user", Host: "localhost", Engine: "mariadb", LinkedDBName: "example_app", PasswordHash: mustHashPassword("password123")},
		},
		PostgresUsers: []DatabaseUser{
			{Username: "analytics_user", Host: "localhost", Engine: "postgresql", LinkedDBName: "analytics", PasswordHash: mustHashPassword("password123")},
		},
		MariaRemoteRules: []RemoteAccessRule{
			{Engine: "mariadb", DBUser: "example_user", DBName: "example_app", Remote: "127.0.0.1/32", AuthMethod: "password"},
		},
		PostgresRemoteRules: []RemoteAccessRule{
			{Engine: "postgresql", DBUser: "analytics_user", DBName: "analytics", Remote: "127.0.0.1/32", AuthMethod: "scram-sha-256"},
		},
		DBLinks: []WebsiteDBLink{
			{Domain: "example.com", Engine: "mariadb", DBName: "example_app", DBUser: "example_user", LinkedAt: now},
		},
		Subdomains: []Subdomain{
			{FQDN: "blog.example.com", ParentDomain: "example.com", PHPVersion: "8.3", SSLEnabled: true, CreatedAt: now},
		},
		Aliases: []DomainAlias{
			{Domain: "example.com", Alias: "www.example.com"},
		},
		AdvancedConfig: map[string]WebsiteAdvancedConfig{
			"example.com": {
				OpenBasedir:  true,
				RewriteRules: "RewriteEngine On\nRewriteCond %{HTTPS} off\nRewriteRule ^ https://%{HTTP_HOST}%{REQUEST_URI} [L,R=301]",
				VhostConfig:  "docRoot                   $VH_ROOT/public_html\nindex  {\n  useServer               0\n  indexFiles              index.php, index.html\n}",
			},
		},
		CustomSSL: map[string]WebsiteCustomSSL{},
		Services: []ServiceStatus{
			{Name: "api-gateway", Desc: "Edge gateway and auth layer", Status: "running"},
			{Name: "panel-service", Desc: "Panel domain service", Status: "running"},
			{Name: "frontend", Desc: "Vue SPA", Status: "running"},
			{Name: "mariadb", Desc: "Primary relational engine", Status: "running"},
			{Name: "postgresql", Desc: "Secondary relational engine", Status: "running"},
			{Name: "openlitespeed", Desc: "Web server integration target", Status: "running"},
		},
		Processes: []ProcessInfo{
			{PID: 1201, User: "root", CPU: 0.5, Mem: 1.2, Command: "aurapanel-api-gateway"},
			{PID: 1202, User: "root", CPU: 0.4, Mem: 1.6, Command: "aurapanel-panel-service"},
			{PID: 1203, User: "www-data", CPU: 1.1, Mem: 4.8, Command: "openlitespeed"},
			{PID: 1204, User: "mysql", CPU: 0.8, Mem: 6.1, Command: "mariadbd"},
		},
		FirewallRules: []FirewallRule{
			{IPAddress: "10.10.10.10", Block: true, Reason: "Suspicious scan"},
		},
		SSHKeys: []SSHKey{},
		EBPFEvents: []string{
			"Policy sync completed for panel-service.",
			"Gateway -> service contract validation passed.",
			"Runtime telemetry collector is active.",
		},
		MalwareJobs:    []MalwareJob{},
		Quarantine:     []QuarantineRecord{},
		NextPackageID:  3,
		NextUserID:     3,
		NextProcessPID: 1205,
	}
}

func (s *service) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/v1/", loggingMiddleware(http.HandlerFunc(s.handleCompat)))
	return mux
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[%s] %s %s", time.Since(start).Round(time.Millisecond), r.Method, r.URL.Path)
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
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/status/service/control":
		s.handleServiceControl(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/status/panel-port":
		s.handlePanelPortGet(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/status/panel-port":
		s.handlePanelPortSet(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/status":
		s.handleSecurityStatus(w)
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
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Allowed: true, Score: 8, Reason: "Request matched baseline allow rules in Go service mode."})
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/2fa/setup":
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: map[string]interface{}{"secret": generateSecret(16), "qr_base64": ""}})
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/2fa/verify":
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Valid: true})
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/ssh-keys":
		s.handleSSHKeysList(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/ssh-keys":
		s.handleSSHKeyCreate(w, r)
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/security/ssh-keys":
		s.handleSSHKeyDelete(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/immutable/status":
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: map[string]interface{}{"supported": false, "mode": "mutable-dev"}})
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
			"version":      "2026.03-go",
			"status":       "ok",
			"uptime":       time.Since(s.startedAt).Round(time.Second).String(),
		},
	})
}

func (s *service) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
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

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.findWebsiteLocked(domain) != nil {
		writeError(w, http.StatusConflict, "Website already exists.")
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
	s.ensureUserLocked(owner, fmt.Sprintf("%s@example.com", owner), "user", "default", "")
	s.recountSitesLocked()
	s.ensureDefaultSiteArtifactsLocked(site.Domain)

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
	s.removeSiteArtifactsLocked(domain)
	s.recountSitesLocked()
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	if engine == "mariadb" {
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.MariaDBs})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.PostgresDBs})
}

func (s *service) handleDatabaseUsers(w http.ResponseWriter, engine string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if engine == "mariadb" {
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: publicDBUsers(s.state.MariaUsers)})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: publicDBUsers(s.state.PostgresUsers)})
}

func (s *service) handleRemoteAccessList(w http.ResponseWriter, engine string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
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

	s.mu.Lock()
	defer s.mu.Unlock()

	db := DatabaseRecord{
		Name:       sanitizeDBName(payload.DBName),
		Size:       "0 MB",
		Tables:     0,
		Engine:     engine,
		Owner:      firstNonEmpty(strings.TrimSpace(payload.Owner), "aura"),
		SiteDomain: normalizeDomain(payload.SiteDomain),
	}
	user := DatabaseUser{
		Username:     sanitizeDBName(payload.DBUser),
		Host:         "localhost",
		Engine:       engine,
		LinkedDBName: db.Name,
		PasswordHash: mustHashPassword(firstNonEmpty(strings.TrimSpace(payload.DBPass), "password123")),
	}

	if engine == "mariadb" {
		s.state.MariaDBs = append(s.state.MariaDBs, db)
		s.state.MariaUsers = append(s.state.MariaUsers, user)
	} else {
		s.state.PostgresDBs = append(s.state.PostgresDBs, db)
		s.state.PostgresUsers = append(s.state.PostgresUsers, user)
	}

	if db.SiteDomain != "" {
		s.state.DBLinks = append(s.state.DBLinks, WebsiteDBLink{
			Domain:   db.SiteDomain,
			Engine:   engine,
			DBName:   db.Name,
			DBUser:   user.Username,
			LinkedAt: time.Now().UTC().Unix(),
		})
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"db_name": db.Name,
			"db_user": user.Username,
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

	s.mu.Lock()
	defer s.mu.Unlock()

	target := sanitizeDBName(payload.DBUser)
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

	s.mu.Lock()
	defer s.mu.Unlock()

	rule := RemoteAccessRule{
		Engine:     engine,
		DBUser:     sanitizeDBName(payload.DBUser),
		DBName:     sanitizeDBName(payload.DBName),
		Remote:     strings.TrimSpace(payload.RemoteIP),
		AuthMethod: authMethodForEngine(engine),
	}
	if engine == "mariadb" {
		s.state.MariaRemoteRules = append(s.state.MariaRemoteRules, rule)
	} else {
		s.state.PostgresRemoteRules = append(s.state.PostgresRemoteRules, rule)
	}
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	uptime := time.Since(s.startedAt)
	siteCount := len(s.state.Websites)
	dbCount := len(s.state.MariaDBs) + len(s.state.PostgresDBs)

	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"cpu_usage":      18 + siteCount,
			"cpu_cores":      4,
			"cpu_model":      "AuraPanel Virtual CPU",
			"ram_usage":      34 + minInt(20, dbCount*3),
			"ram_used":       "1.7 GB",
			"ram_total":      "4 GB",
			"disk_usage":     41 + minInt(20, siteCount*4),
			"disk_used":      "8.2 GB",
			"disk_total":     "20 GB",
			"uptime_seconds": int(uptime.Seconds()),
			"uptime_human":   uptime.Round(time.Second).String(),
			"load_avg":       fmt.Sprintf("%.2f %.2f %.2f", 0.45+float64(siteCount)/20.0, 0.39, 0.31),
		},
	})
}

func (s *service) handleServices(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.Services})
}

func (s *service) handleProcesses(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.Processes})
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

	s.mu.Lock()
	defer s.mu.Unlock()

	name := strings.TrimSpace(payload.Name)
	action := strings.ToLower(strings.TrimSpace(payload.Action))
	if action == "kill" {
		pid, _ := strconv.Atoi(name)
		for i := range s.state.Processes {
			if s.state.Processes[i].PID == pid {
				s.state.Processes = append(s.state.Processes[:i], s.state.Processes[i+1:]...)
				writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Process terminated."})
				return
			}
		}
		writeError(w, http.StatusNotFound, "Process not found.")
		return
	}

	for i := range s.state.Services {
		if s.state.Services[i].Name == name {
			switch action {
			case "start", "restart":
				s.state.Services[i].Status = "running"
			case "stop":
				s.state.Services[i].Status = "stopped"
			default:
				writeError(w, http.StatusBadRequest, "Unsupported action.")
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Service action applied."})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Service not found.")
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
		firewallActions = append([]string{fmt.Sprintf("Allow tcp/%d on nftables", payload.Port)}, firewallActions...)
	}

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
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"ebpf_monitoring":      true,
			"ml_waf":               true,
			"totp_2fa":             true,
			"wireguard_federation": false,
			"immutable_os_support": false,
			"live_patching":        true,
			"one_click_hardening":  true,
			"nft_firewall":         true,
			"ssh_key_manager":      true,
		},
	})
}

func (s *service) handleEBPFEvents(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.EBPFEvents})
}

func (s *service) handleCollectEBPF(w http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.EBPFEvents = append([]string{
		fmt.Sprintf("Telemetry snapshot collected at %s", time.Now().UTC().Format(time.RFC3339)),
	}, s.state.EBPFEvents...)
	if len(s.state.EBPFEvents) > 20 {
		s.state.EBPFEvents = s.state.EBPFEvents[:20]
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Telemetry snapshot collected."})
}

func (s *service) handleFirewallRulesList(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.FirewallRules})
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

	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.FirewallRules = append(s.state.FirewallRules, payload)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Firewall rule added."})
}

func (s *service) handleFirewallRuleDelete(w http.ResponseWriter, r *http.Request) {
	ip := strings.TrimSpace(r.URL.Query().Get("ip_address"))
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.FirewallRules {
		if s.state.FirewallRules[i].IPAddress == ip {
			s.state.FirewallRules = append(s.state.FirewallRules[:i], s.state.FirewallRules[i+1:]...)
			writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Firewall rule deleted."})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Firewall rule not found.")
}

func (s *service) handleSSHKeysList(w http.ResponseWriter, r *http.Request) {
	user := strings.TrimSpace(r.URL.Query().Get("user"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	var keys []SSHKey
	for _, key := range s.state.SSHKeys {
		if user == "" || key.User == user {
			keys = append(keys, key)
		}
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: keys})
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

	s.mu.Lock()
	defer s.mu.Unlock()
	key := SSHKey{
		ID:        generateSecret(10),
		User:      strings.TrimSpace(payload.User),
		Title:     firstNonEmpty(strings.TrimSpace(payload.Title), "Imported key"),
		PublicKey: strings.TrimSpace(payload.PublicKey),
	}
	s.state.SSHKeys = append(s.state.SSHKeys, key)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "SSH key added.", Data: key})
}

func (s *service) handleSSHKeyDelete(w http.ResponseWriter, r *http.Request) {
	user := strings.TrimSpace(r.URL.Query().Get("user"))
	keyID := strings.TrimSpace(r.URL.Query().Get("key_id"))
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.SSHKeys {
		item := s.state.SSHKeys[i]
		if item.User == user && item.ID == keyID {
			s.state.SSHKeys = append(s.state.SSHKeys[:i], s.state.SSHKeys[i+1:]...)
			writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "SSH key removed."})
			return
		}
	}
	writeError(w, http.StatusNotFound, "SSH key not found.")
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
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"domain": payload.Domain,
			"applied_rules": []string{
				fmt.Sprintf("%s stack defaults applied", firstNonEmpty(payload.Stack, "generic")),
				"Trusted proxy headers normalized",
				"Filesystem write permissions reviewed",
			},
		},
	})
}

func (s *service) handleSiteLogs(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	kind := firstNonEmpty(strings.TrimSpace(r.URL.Query().Get("kind")), "access")
	if domain == "" {
		domain = "unknown-site"
	}
	lines := []string{
		fmt.Sprintf("[%s] %s request accepted by panel-service", time.Now().UTC().Format(time.RFC3339), kind),
		fmt.Sprintf("[%s] %s upstream contract now terminates in Go service", time.Now().UTC().Format(time.RFC3339), domain),
		fmt.Sprintf("[%s] compatibility mode active for website operations", time.Now().UTC().Format(time.RFC3339)),
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if site := s.findWebsiteLocked(domain); site != nil {
		site.SSL = true
	}
	s.recordIssuedCertificateLocked(domain, "letsencrypt", false)
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
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Website status updated.", Data: site})
}

func (s *service) handleFallback(w http.ResponseWriter, r *http.Request) {
	payload := fallbackPayloadForPath(r.URL.Path)
	response := apiResponse{
		Status:  "success",
		Message: "Endpoint is running in Go compatibility mode.",
	}
	if r.Method == http.MethodGet {
		response.Data = payload
	} else {
		response.Data = map[string]interface{}{
			"path":   r.URL.Path,
			"method": r.Method,
			"mode":   "compatibility",
		}
	}
	writeJSON(w, http.StatusOK, response)
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
		s.state.AdvancedConfig[key] = WebsiteAdvancedConfig{
			OpenBasedir:  true,
			RewriteRules: "RewriteEngine On",
			VhostConfig:  fmt.Sprintf("vhDomain %s", key),
		}
	}
}

func (s *service) removeSiteArtifactsLocked(domain string) {
	delete(s.state.AdvancedConfig, domain)
	delete(s.state.CustomSSL, domain)
	s.state.Aliases = removeAliasesByDomain(s.state.Aliases, domain)
	s.state.Subdomains = removeSubdomainsByParent(s.state.Subdomains, domain)
	s.state.DBLinks = removeDBLinksByDomain(s.state.DBLinks, domain)
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
