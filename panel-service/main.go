package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
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
	defaultServiceAddr       = "127.0.0.1:8081"
	defaultGatewayPort       = 8090
	currentPanelVersion      = "Aura Panel V1"
	updateCacheTTL           = 45 * time.Second
	updateErrorCacheTTL      = 2 * time.Minute
	defaultAdminEmail        = "admin@server.com"
	maxJSONBodyBytes         = 1 << 20
	defaultJWTSessionTTL     = 12 * time.Hour
	defaultAuthCookie        = "aurapanel_session"
	defaultDBToolTempUserTTL = 4 * time.Hour

	serviceMaxFailedAttempts = 5
	serviceFailureWindow     = 10 * time.Minute
	serviceLockDuration      = 15 * time.Minute

	defaultStatePersistDebounce   = 900 * time.Millisecond
	defaultHousekeepingInterval   = 60 * time.Second
	defaultSecurityStatusCacheTTL = 8 * time.Second
	securityStatusRateWindow      = 10 * time.Second
	securityStatusNonAdminLimit   = 8

	defaultGitHubReleaseTimeout = 12 * time.Second
	defaultGitHubRetryAttempts  = 3
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
	ID             int      `json:"id"`
	Username       string   `json:"username"`
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	Role           string   `json:"role"`
	Package        string   `json:"package"`
	ParentUsername string   `json:"parent_username,omitempty"`
	IsOwner        bool     `json:"is_owner,omitempty"`
	RolePolicyID   string   `json:"role_policy_id,omitempty"`
	RolePolicy     string   `json:"role_policy_name,omitempty"`
	Permissions    []string `json:"permissions,omitempty"`
	Sites          int      `json:"sites"`
	Active         bool     `json:"active"`
	TwoFAEnabled   bool     `json:"two_fa_enabled"`
	PasswordHash   string   `json:"password_hash,omitempty"`
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
	DBHost   string `json:"db_host,omitempty"`
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

type FirewallPortRule struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Block    bool   `json:"block"`
	Reason   string `json:"reason"`
}

type UpdateStatus struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version,omitempty"`
	LatestTag       string `json:"latest_tag,omitempty"`
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
	GatewayPort          int
	Websites             []Website
	Packages             []Package
	Users                []PanelUser
	MariaDBs             []DatabaseRecord
	PostgresDBs          []DatabaseRecord
	MariaUsers           []DatabaseUser
	PostgresUsers        []DatabaseUser
	MariaRemoteRules     []RemoteAccessRule
	PostgresRemoteRules  []RemoteAccessRule
	DBLinks              []WebsiteDBLink
	Subdomains           []Subdomain
	Aliases              []DomainAlias
	AdvancedConfig       map[string]WebsiteAdvancedConfig
	CustomSSL            map[string]WebsiteCustomSSL
	Services             []ServiceStatus
	Processes            []ProcessInfo
	FirewallRules        []FirewallRule
	SSHKeys              []SSHKey
	EBPFEvents           []string
	MalwareJobs          []MalwareJob
	Quarantine           []QuarantineRecord
	TwoFASecrets         map[string]string
	ResellerToken        string
	ResellerTokenSavedAt int64
	NextPackageID        int
	NextUserID           int
	NextProcessPID       int
}

type service struct {
	mu                  sync.RWMutex
	startedAt           time.Time
	state               appState
	modules             moduleState
	dbToolLaunchSecrets map[string]dbToolLaunchSecret
	dbToolTempUsers     map[string]dbToolTempUser
	update              updateStatusCache
	updateJob           panelUpdateJobState
	dbAccess            map[string]dbToolSessionGrant
	dbACLFile           string
	dbACLReloadInFlight bool
	dbACLReloadNeeded   bool
	dbACLLastReload     time.Time
	persistQueue        chan struct{}
	persistDebounce     time.Duration
	olsSyncQueue        chan olsSyncRequest
	olsSyncDebounce     time.Duration
	housekeepingEvery   time.Duration

	securityMu              sync.Mutex
	securityStatusTTL       time.Duration
	securityStatusCache     securitySnapshot
	securityStatusCacheTime time.Time
	securityStatusRate      map[string]securityStatusRateWindowState
}

type olsSyncRequest struct {
	sites    []Website
	advanced map[string]WebsiteAdvancedConfig
	aliases  []DomainAlias
	done     chan error
}

type jwtClaims struct {
	Email        string   `json:"email"`
	Name         string   `json:"name"`
	Role         string   `json:"role"`
	Username     string   `json:"username,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
	RolePolicyID string   `json:"role_policy_id,omitempty"`
	RolePolicy   string   `json:"role_policy_name,omitempty"`
	jwt.RegisteredClaims
}

type serviceContextKey string

const servicePrincipalContextKey serviceContextKey = "service_principal"

type servicePrincipal struct {
	Email        string
	Name         string
	Role         string
	Username     string
	Permissions  []string
	RolePolicyID string
	RolePolicy   string
}

type serviceLoginAttempt struct {
	Failures    int
	FirstFail   time.Time
	LockedUntil time.Time
}

var (
	serviceLoginAttemptsMu sync.Mutex
	serviceLoginAttempts   = map[string]serviceLoginAttempt{}
)

func main() {
	if err := requireServiceSecurityConfig(); err != nil {
		log.Fatalf("security configuration error: %v", err)
	}

	svc := newService()
	addr := strings.TrimSpace(os.Getenv("AURAPANEL_SERVICE_ADDR"))
	if addr == "" {
		addr = defaultServiceAddr
	}
	if !serviceAllowRemoteBind() && !isLoopbackBindAddress(addr) {
		log.Fatalf("refusing non-loopback bind address %q without AURAPANEL_ALLOW_REMOTE_SERVICE=true", addr)
	}

	log.Printf("AuraPanel panel-service listening on %s", addr)
	server := &http.Server{
		Addr:              addr,
		Handler:           svc.routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}

func newService() *service {
	svc := &service{
		startedAt:           time.Now().UTC(),
		state:               seedState(),
		modules:             seedModuleState(),
		dbToolLaunchSecrets: map[string]dbToolLaunchSecret{},
		dbToolTempUsers:     map[string]dbToolTempUser{},
		persistQueue:        make(chan struct{}, 1),
		persistDebounce:     statePersistDebounce(),
		olsSyncQueue:        make(chan olsSyncRequest, 32),
		olsSyncDebounce:     olsSyncDebounce(),
		housekeepingEvery:   housekeepingInterval(),
		securityStatusTTL:   securityStatusCacheTTL(),
		securityStatusRate:  map[string]securityStatusRateWindowState{},
	}
	if err := svc.loadRuntimeState(); err != nil {
		log.Printf("runtime state load skipped: %v", err)
	}
	svc.mu.Lock()
	svc.ensureOwnerConsistencyLocked()
	svc.ensureUserHierarchyLocked()
	svc.reconcileUserRolePoliciesLocked()
	svc.mu.Unlock()
	svc.startOLSSyncWorker()
	svc.bootstrapModules()
	svc.initializeDBToolAccessRuntime()
	svc.cleanupRuntimeTemporaryDBUsersOnStartup()
	svc.startStatePersistenceWorker()
	svc.startHousekeepingWorker()
	go func() {
		if err := svc.selfHealOLSManagedConfig(); err != nil {
			log.Printf("OpenLiteSpeed startup self-heal skipped: %v", err)
		}
	}()
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
			IsOwner:      true,
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
		Users:                users,
		MariaDBs:             []DatabaseRecord{},
		PostgresDBs:          []DatabaseRecord{},
		MariaUsers:           []DatabaseUser{},
		PostgresUsers:        []DatabaseUser{},
		MariaRemoteRules:     []RemoteAccessRule{},
		PostgresRemoteRules:  []RemoteAccessRule{},
		DBLinks:              []WebsiteDBLink{},
		Subdomains:           []Subdomain{},
		Aliases:              []DomainAlias{},
		AdvancedConfig:       map[string]WebsiteAdvancedConfig{},
		CustomSSL:            map[string]WebsiteCustomSSL{},
		Services:             []ServiceStatus{},
		Processes:            []ProcessInfo{},
		FirewallRules:        []FirewallRule{},
		SSHKeys:              []SSHKey{},
		EBPFEvents:           []string{},
		MalwareJobs:          []MalwareJob{},
		Quarantine:           []QuarantineRecord{},
		TwoFASecrets:         map[string]string{},
		ResellerToken:        "",
		ResellerTokenSavedAt: 0,
		NextPackageID:        3,
		NextUserID:           2,
		NextProcessPID:       1201,
	}
}

func (s *service) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/v1/", serviceAuthMiddleware(persistenceMiddleware(loggingMiddleware(http.HandlerFunc(s.handleCompat)), s)))
	return mux
}

func requireServiceSecurityConfig() error {
	if devSimulationEnabled() {
		return nil
	}
	secret := strings.TrimSpace(os.Getenv("AURAPANEL_JWT_SECRET"))
	if len(secret) < 32 {
		return fmt.Errorf("AURAPANEL_JWT_SECRET must be set and at least 32 chars")
	}
	proxyToken := strings.TrimSpace(os.Getenv("AURAPANEL_INTERNAL_PROXY_TOKEN"))
	if len(proxyToken) < 32 {
		return fmt.Errorf("AURAPANEL_INTERNAL_PROXY_TOKEN must be set and at least 32 chars")
	}
	return nil
}

func devSimulationEnabled() bool {
	normalized := strings.ToLower(strings.TrimSpace(os.Getenv("AURAPANEL_DEV_SIMULATION")))
	return normalized == "1" || normalized == "true" || normalized == "yes" || normalized == "on"
}

func serviceAllowRemoteBind() bool {
	normalized := strings.ToLower(strings.TrimSpace(os.Getenv("AURAPANEL_ALLOW_REMOTE_SERVICE")))
	return normalized == "1" || normalized == "true" || normalized == "yes" || normalized == "on"
}

func isLoopbackBindAddress(addr string) bool {
	host := strings.TrimSpace(addr)
	if strings.Contains(host, ":") {
		parsedHost, _, err := net.SplitHostPort(host)
		if err == nil {
			host = parsedHost
		}
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[%s] %s %s", time.Since(start).Round(time.Millisecond), r.Method, r.URL.Path)
	})
}

func servicePublicRoute(method, path string) bool {
	switch {
	case method == http.MethodGet && path == "/api/v1/health":
		return true
	case method == http.MethodPost && path == "/api/v1/auth/login":
		return true
	case path == "/api/v1/mail/webmail/sso/consume":
		return true
	case path == "/api/v1/db/tools/phpmyadmin/sso/consume":
		return true
	case path == "/api/v1/db/tools/pgadmin/sso/consume":
		return true
	default:
		return false
	}
}

func serviceAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if servicePublicRoute(r.Method, r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		expected := strings.TrimSpace(os.Getenv("AURAPANEL_INTERNAL_PROXY_TOKEN"))
		received := strings.TrimSpace(r.Header.Get("X-Aura-Proxy-Token"))
		if expected != "" {
			if subtle.ConstantTimeCompare([]byte(expected), []byte(received)) != 1 {
				writeError(w, http.StatusUnauthorized, "Unauthorized.")
				return
			}
		} else if !devSimulationEnabled() {
			writeError(w, http.StatusUnauthorized, "Unauthorized.")
			return
		} else if !isLoopbackRemoteAddr(r.RemoteAddr) {
			writeError(w, http.StatusUnauthorized, "Unauthorized.")
			return
		}

		principal, ok := servicePrincipalFromHeaders(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "Unauthorized.")
			return
		}
		ctx := context.WithValue(r.Context(), servicePrincipalContextKey, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isLoopbackRemoteAddr(remoteAddr string) bool {
	host := strings.TrimSpace(remoteAddr)
	if strings.Contains(host, ":") {
		parsedHost, _, err := net.SplitHostPort(host)
		if err == nil {
			host = parsedHost
		}
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func servicePrincipalFromHeaders(r *http.Request) (servicePrincipal, bool) {
	email := strings.TrimSpace(r.Header.Get("X-Aura-Auth-Email"))
	role := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Aura-Auth-Role")))
	name := strings.TrimSpace(r.Header.Get("X-Aura-Auth-Name"))
	username := sanitizeName(strings.TrimSpace(r.Header.Get("X-Aura-Auth-Username")))
	rolePolicyID := strings.TrimSpace(r.Header.Get("X-Aura-Auth-Role-Policy-Id"))
	rolePolicy := strings.TrimSpace(r.Header.Get("X-Aura-Auth-Role-Policy-Name"))
	permissions := splitCSV(strings.TrimSpace(r.Header.Get("X-Aura-Auth-Permissions")))
	if username == "" {
		username = sanitizeName(strings.Split(strings.ToLower(email), "@")[0])
	}
	if email == "" {
		return servicePrincipal{}, false
	}
	if role != "admin" && role != "reseller" && role != "user" {
		return servicePrincipal{}, false
	}
	return servicePrincipal{
		Email:        strings.ToLower(email),
		Name:         name,
		Role:         role,
		Username:     username,
		Permissions:  permissions,
		RolePolicyID: rolePolicyID,
		RolePolicy:   rolePolicy,
	}, true
}

func principalFromContext(ctx context.Context) (servicePrincipal, bool) {
	value := ctx.Value(servicePrincipalContextKey)
	if value == nil {
		return servicePrincipal{}, false
	}
	principal, ok := value.(servicePrincipal)
	return principal, ok
}

func servicePathMatchesPrefix(path, prefix string) bool {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return false
	}
	normalizedPrefix := strings.TrimSuffix(prefix, "/")
	return path == normalizedPrefix || strings.HasPrefix(path, normalizedPrefix+"/")
}

func (s *service) readRequestJSONMap(r *http.Request) map[string]interface{} {
	if r == nil || r.Body == nil {
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBodyBytes))
	if err != nil {
		return nil
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(raw))
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return payload
}

func requestStringField(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
	raw, ok := payload[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func normalizeDomainCandidate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "@") {
		parts := strings.Split(value, "@")
		value = parts[len(parts)-1]
	}
	return normalizeDomain(value)
}

func (s *service) resolveOwnedDomainCandidate(principal servicePrincipal, raw string) (string, bool) {
	candidate := normalizeDomainCandidate(raw)
	if candidate == "" {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	current := candidate
	for {
		if s.canAccessDomainLocked(principal, current) {
			return current, true
		}
		dot := strings.Index(current, ".")
		if dot < 0 {
			break
		}
		current = current[dot+1:]
	}
	return "", false
}

func (s *service) domainContextFromRequest(r *http.Request) (string, bool) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		return "", false
	}
	queryKeys := []string{"domain", "parent_domain", "base_domain", "site_domain", "source_domain", "staging_domain", "fqdn", "address", "source", "email"}
	for _, key := range queryKeys {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			if resolved, ok := s.resolveOwnedDomainCandidate(principal, value); ok {
				return resolved, true
			}
		}
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/dns/zones/") {
		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/dns/zones/")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) > 0 {
			if resolved, ok := s.resolveOwnedDomainCandidate(principal, parts[0]); ok {
				return resolved, true
			}
		}
	}
	payload := s.readRequestJSONMap(r)
	bodyKeys := []string{"domain", "parent_domain", "base_domain", "site_domain", "source_domain", "staging_domain", "fqdn", "address", "source", "email"}
	for _, key := range bodyKeys {
		if value := requestStringField(payload, key); value != "" {
			if resolved, ok := s.resolveOwnedDomainCandidate(principal, value); ok {
				return resolved, true
			}
		}
	}
	return "", false
}

func (s *service) rawDomainFromRequest(r *http.Request) string {
	keys := []string{"domain", "parent_domain", "base_domain", "site_domain", "source_domain", "staging_domain", "fqdn", "address", "source", "email"}
	for _, key := range keys {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			if normalized := normalizeDomainCandidate(value); normalized != "" {
				return normalized
			}
		}
	}
	payload := s.readRequestJSONMap(r)
	for _, key := range keys {
		if value := requestStringField(payload, key); value != "" {
			if normalized := normalizeDomainCandidate(value); normalized != "" {
				return normalized
			}
		}
	}
	return ""
}

func (s *service) nonAdminCanProvisionDomain(principal servicePrincipal, r *http.Request, domain string) bool {
	s.mu.RLock()
	if s.canAccessDomainLocked(principal, domain) {
		s.mu.RUnlock()
		return true
	}
	ids := s.principalScopedUsernamesLocked(principal)
	s.mu.RUnlock()
	payload := s.readRequestJSONMap(r)
	for _, key := range []string{"user", "owner", "email"} {
		value := strings.TrimSpace(requestStringField(payload, key))
		if value == "" {
			continue
		}
		if key == "email" {
			value = strings.Split(strings.ToLower(value), "@")[0]
		}
		if _, ok := ids[sanitizeName(value)]; ok {
			return true
		}
	}
	return false
}

func (s *service) nonAdminRoutePolicy(w http.ResponseWriter, r *http.Request) bool {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return false
	}
	if principal.Role == "admin" {
		return true
	}
	path := strings.TrimSpace(r.URL.Path)
	method := r.Method

	allowedWithoutDomain := []string{
		"/api/v1/auth/me",
		"/api/v1/auth/logout",
		"/api/v1/status/metrics",
		"/api/v1/status/services",
		"/api/v1/status/update",
		"/api/v1/security/status",
		"/api/v1/security/immutable/status",
		"/api/v1/security/2fa/setup",
		"/api/v1/security/2fa/verify",
		"/api/v1/vhost/list",
		"/api/v1/files",
		"/api/v1/db/tools",
	}
	for _, prefix := range allowedWithoutDomain {
		if servicePathMatchesPrefix(path, prefix) {
			return true
		}
	}

	adminOnlyPrefixes := []string{
		"/api/v1/ai",
		"/api/v1/packages",
		"/api/v1/platform/capabilities",
		"/api/v1/cloudlinux",
		"/api/v1/php",
		"/api/v1/vhost/discover",
		"/api/v1/vhost/import",
		"/api/v1/ols",
		"/api/v1/storage/minio",
		"/api/v1/federated",
		"/api/v1/activity/log",
		"/api/v1/migration",
		"/api/v1/acl",
		"/api/v1/reseller",
		"/api/v1/docker",
		"/api/v1/gitops",
		"/api/v1/perf",
		"/api/v1/cloudflare",
		"/api/v1/security/ssh-keys",
		"/api/v1/security/firewall",
		"/api/v1/security/waf",
		"/api/v1/security/hardening/apply",
		"/api/v1/security/fail2ban",
		"/api/v1/security/ssh/config",
		"/api/v1/security/live-patch",
		"/api/v1/security/malware",
		"/api/v1/security/ddos",
		"/api/v1/status/service/control",
		"/api/v1/status/processes",
		"/api/v1/status/panel-port",
		"/api/v1/status/panel-reverse-domain",
		"/api/v1/status/web-stack",
		"/api/v1/status/update/apply",
		"/api/v1/backup/destinations",
		"/api/v1/backup/schedules",
		"/api/v1/db/mariadb/tuning",
		"/api/v1/db/postgresql/tuning",
		"/api/v1/mail/tuning",
		"/api/v1/ftp/tuning",
		"/api/v1/system/reseller-token",
		"/api/v1/websites/vhost-config",
		"/api/v1/websites/custom-ssl",
	}
	for _, prefix := range adminOnlyPrefixes {
		if servicePathMatchesPrefix(path, prefix) {
			writeError(w, http.StatusForbidden, "This endpoint is restricted to admin users.")
			return false
		}
	}

	if servicePathMatchesPrefix(path, "/api/v1/users") {
		if normalizeRole(principal.Role) != "reseller" {
			writeError(w, http.StatusForbidden, "This endpoint is restricted to reseller or admin users.")
			return false
		}
		return true
	}

	if (path == "/api/v1/vhost" || path == "/api/v1/vhost/create") && method == http.MethodPost {
		rawDomain := s.rawDomainFromRequest(r)
		if rawDomain == "" {
			writeError(w, http.StatusForbidden, "Domain context is required for this operation.")
			return false
		}
		if !isValidDomainName(rawDomain) {
			writeError(w, http.StatusBadRequest, "A valid domain is required.")
			return false
		}
		if s.nonAdminCanProvisionDomain(principal, r, rawDomain) {
			return true
		}
		writeError(w, http.StatusForbidden, "You cannot provision this domain with current account context.")
		return false
	}

	domain, hasDomain := s.domainContextFromRequest(r)
	if !hasDomain {
		writeError(w, http.StatusForbidden, "Domain context is required for this operation.")
		return false
	}
	if !isValidDomainName(domain) {
		writeError(w, http.StatusBadRequest, "A valid domain is required.")
		return false
	}
	if servicePathMatchesPrefix(path, "/api/v1/dns") && (path == "/api/v1/dns/zone" || strings.Contains(path, "/dns/zones/")) {
		if s.nonAdminCanProvisionDomain(principal, r, domain) {
			return true
		}
	}
	if servicePathMatchesPrefix(path, "/api/v1/mail/webmail/sso") {
		if s.nonAdminCanProvisionDomain(principal, r, domain) {
			return true
		}
		return s.requireDomainAccess(w, r, domain)
	}
	return s.requireDomainAccess(w, r, domain)
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
		svc.enqueueStatePersist()
	})
}

func (s *service) handleCompat(w http.ResponseWriter, r *http.Request) {
	if !servicePublicRoute(r.Method, r.URL.Path) {
		if !s.nonAdminRoutePolicy(w, r) {
			return
		}
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/health":
		s.handleHealth(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/login":
		s.handleAuthLogin(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/logout":
		s.handleAuthLogout(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/auth/me":
		s.handleAuthMe(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/system/reseller-token":
		s.handleResellerTokenGet(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/system/reseller-token":
		s.handleResellerTokenSet(w, r)
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/system/reseller-token":
		s.handleResellerTokenDelete(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/vhost/list":
		s.handleVhostList(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/vhost/discover":
		s.handleVhostDiscovery(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/vhost/import":
		s.handleVhostImport(w, r)
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
		s.handleUsersList(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/users/create":
		s.handleUsersCreate(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/users/update":
		s.handleUsersUpdate(w, r)
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
		s.handleDatabaseList(w, r, "mariadb")
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/db/mariadb/users":
		s.handleDatabaseUsers(w, r, "mariadb")
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/db/mariadb/remote-access":
		s.handleRemoteAccessList(w, r, "mariadb")
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/mariadb/create":
		s.handleDatabaseCreate(w, r, "mariadb")
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/mariadb/drop":
		s.handleDatabaseDrop(w, r, "mariadb")
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/mariadb/password":
		s.handleDatabasePasswordUpdate(w, r, "mariadb")
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/mariadb/remote-access":
		s.handleRemoteAccessCreate(w, r, "mariadb")
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/db/postgres/list":
		s.handleDatabaseList(w, r, "postgresql")
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/db/postgres/users":
		s.handleDatabaseUsers(w, r, "postgresql")
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/db/postgres/remote-access":
		s.handleRemoteAccessList(w, r, "postgresql")
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
		s.handleUpdateStatus(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/status/update/apply":
		s.handleUpdateApply(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/status/service/control":
		s.handleServiceControl(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/status/panel-port":
		s.handlePanelPortGet(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/status/panel-port":
		s.handlePanelPortSet(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/status/panel-reverse-domain":
		s.handlePanelReverseDomainGet(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/status/panel-reverse-domain":
		s.handlePanelReverseDomainSet(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/status/web-stack":
		s.handleWebStackGet(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/status/web-stack":
		s.handleWebStackSet(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/status":
		s.handleSecurityStatus(w, r)
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
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/firewall/rules/delete":
		s.handleFirewallRuleDeleteAction(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/firewall/ports":
		s.handleFirewallPortRulesList(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/firewall/ports":
		s.handleFirewallPortRuleCreate(w, r)
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/security/firewall/ports":
		s.handleFirewallPortRuleDelete(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/firewall/ports/delete":
		s.handleFirewallPortRuleDeleteAction(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/waf":
		s.handleSecurityWAF(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/ddos":
		s.handleSecurityDDoSGet(w)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/ddos":
		s.handleSecurityDDoSSet(w, r)
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

func (s *service) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	force := isTruthyQueryValue(r.URL.Query().Get("force"))
	status := s.getUpdateStatus(force)
	job := s.getUpdateJobSnapshot()

	data := map[string]interface{}{
		"current_version":  status.CurrentVersion,
		"latest_version":   status.LatestVersion,
		"latest_tag":       status.LatestTag,
		"update_available": status.UpdateAvailable,
		"release_name":     status.ReleaseName,
		"release_url":      status.ReleaseURL,
		"release_notes":    status.ReleaseNotes,
		"published_at":     status.PublishedAt,
		"source":           status.Source,
		"checked_at":       status.CheckedAt,
		"error":            status.Error,
	}
	if !isPanelUpdateJobEmpty(job) {
		data["job"] = job
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data:   data,
	})
}

func (s *service) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	principal, ok := principalFromContext(r.Context())
	if !ok || principal.Role != "admin" {
		writeError(w, http.StatusForbidden, "This endpoint is restricted to admin users.")
		return
	}

	job, started := s.beginPanelUpdateJob()
	if !started {
		writeJSON(w, http.StatusOK, apiResponse{
			Status:  "success",
			Message: "Panel deploy is already running in the background.",
			Data: map[string]interface{}{
				"running": true,
				"job":     job,
			},
		})
		return
	}

	if err := s.saveRuntimeState(); err != nil {
		log.Printf("panel update job start persist failed: %v", err)
	}

	go s.runPanelUpdateJob()

	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: "Panel deploy started in background.",
		Data: map[string]interface{}{
			"running": true,
			"job":     job,
		},
	})
}

func (s *service) getUpdateStatus(force bool) UpdateStatus {
	s.mu.RLock()
	cached := s.update
	s.mu.RUnlock()

	cacheTTL := updateCacheTTL
	if strings.TrimSpace(cached.Data.Error) != "" {
		cacheTTL = updateErrorCacheTTL
	}
	if !force && !cached.CheckedAt.IsZero() && time.Since(cached.CheckedAt) < cacheTTL {
		return cached.Data
	}

	status := fetchGitDeployUpdateStatus()
	if status.Error != "" && !cached.CheckedAt.IsZero() {
		previous := cached.Data
		previous.CurrentVersion = status.CurrentVersion
		previous.CheckedAt = status.CheckedAt
		previous.Error = status.Error
		previous.Source = status.Source
		status = previous
	}

	s.mu.Lock()
	s.update = updateStatusCache{
		Data:      status,
		CheckedAt: time.Now().UTC(),
	}
	s.mu.Unlock()

	return status
}

func isTruthyQueryValue(raw string) bool {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	return normalized == "1" || normalized == "true" || normalized == "yes" || normalized == "on"
}

func fetchLatestReleaseStatus() UpdateStatus {
	status := UpdateStatus{
		CurrentVersion: resolveCurrentPanelVersion(),
		Source:         "GitHub Releases",
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	owner := envOr("AURAPANEL_GH_OWNER", "mkoyazilim")
	repo := envOr("AURAPANEL_GH_REPO", "aurapanel")
	baseURL := strings.TrimRight(envOr("AURAPANEL_GH_API_BASE_URL", "https://api.github.com"), "/")
	token := strings.TrimSpace(os.Getenv("AURAPANEL_GH_TOKEN"))
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", baseURL, owner, repo)

	timeout := envDurationSeconds("AURAPANEL_GH_TIMEOUT_SECONDS", defaultGitHubReleaseTimeout)
	attempts := envInt("AURAPANEL_GH_RETRY_ATTEMPTS", defaultGitHubRetryAttempts)
	if attempts < 1 {
		attempts = 1
	}
	client := &http.Client{Timeout: timeout}

	var payload struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		HTMLURL     string `json:"html_url"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
		Draft       bool   `json:"draft"`
		PreRelease  bool   `json:"prerelease"`
	}
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			status.Error = err.Error()
			return status
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "aurapanel-panel-service")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < attempts {
				time.Sleep(time.Duration(attempt*attempt) * 350 * time.Millisecond)
				continue
			}
			break
		}

		rawBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < attempts {
				time.Sleep(time.Duration(attempt*attempt) * 350 * time.Millisecond)
				continue
			}
			break
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			message := fmt.Sprintf("GitHub release check returned HTTP %d", resp.StatusCode)
			if (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests) && resp.Header.Get("X-RateLimit-Remaining") == "0" {
				reset := strings.TrimSpace(resp.Header.Get("X-RateLimit-Reset"))
				message = "GitHub API rate limit exceeded for release check"
				if reset != "" {
					message = fmt.Sprintf("%s (reset=%s)", message, reset)
				}
			}
			if snippet := strings.TrimSpace(string(rawBody)); snippet != "" {
				if len(snippet) > 180 {
					snippet = snippet[:180] + "..."
				}
				message = fmt.Sprintf("%s: %s", message, snippet)
			}
			lastErr = fmt.Errorf(message)
			if attempt < attempts {
				time.Sleep(time.Duration(attempt*attempt) * 350 * time.Millisecond)
				continue
			}
			break
		}

		if err := json.Unmarshal(rawBody, &payload); err != nil {
			lastErr = err
			if attempt < attempts {
				time.Sleep(time.Duration(attempt*attempt) * 350 * time.Millisecond)
				continue
			}
			break
		}

		if payload.Draft {
			status.Error = "Latest GitHub release is still marked as draft."
			return status
		}

		status.ReleaseName = strings.TrimSpace(payload.Name)
		status.LatestTag = strings.TrimSpace(payload.TagName)
		status.LatestVersion = firstNonEmpty(strings.TrimSpace(payload.TagName), strings.TrimSpace(payload.Name))
		status.ReleaseURL = strings.TrimSpace(payload.HTMLURL)
		status.PublishedAt = strings.TrimSpace(payload.PublishedAt)
		status.ReleaseNotes = summarizeReleaseNotes(payload.Body)

		status.UpdateAvailable = shouldMarkUpdateAvailable(status.CurrentVersion, status.LatestVersion, status.LatestTag)
		return status
	}

	if lastErr != nil {
		status.Error = lastErr.Error()
	}
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

func shouldMarkUpdateAvailable(currentVersion, latestVersion, latestTag string) bool {
	latest := strings.TrimSpace(firstNonEmpty(latestTag, latestVersion))
	if latest == "" {
		return false
	}
	current := strings.TrimSpace(currentVersion)
	if current == "" {
		return true
	}
	if matchesReleaseBase(current, latest) {
		return false
	}

	currentSemver, currentSemverOK := extractSemverTriplet(current)
	latestSemver, latestSemverOK := extractSemverTriplet(latest)
	if currentSemverOK && latestSemverOK {
		return compareSemverTriplet(latestSemver, currentSemver) > 0
	}

	currentComparable := normalizeComparableVersion(current)
	latestComparable := normalizeComparableVersion(latest)
	return latestComparable != "" && latestComparable != currentComparable
}

func matchesReleaseBase(currentVersion, latestVersion string) bool {
	current := strings.ToLower(strings.TrimSpace(currentVersion))
	if current == "" {
		return false
	}

	for _, alias := range versionAliases(latestVersion) {
		if alias == "" {
			continue
		}
		if current == alias || strings.HasPrefix(current, alias+"-") {
			return true
		}
	}
	return false
}

func versionAliases(value string) []string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return nil
	}

	aliases := []string{normalized}
	if strings.HasPrefix(normalized, "v") {
		aliases = append(aliases, strings.TrimPrefix(normalized, "v"))
	} else if normalized[0] >= '0' && normalized[0] <= '9' {
		aliases = append(aliases, "v"+normalized)
	}
	return aliases
}

func extractSemverTriplet(value string) ([3]int, bool) {
	var triplet [3]int
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return triplet, false
	}

	start := -1
	for i := 0; i < len(normalized); i++ {
		if normalized[i] >= '0' && normalized[i] <= '9' {
			start = i
			break
		}
	}
	if start < 0 {
		return triplet, false
	}

	end := start
	for end < len(normalized) {
		c := normalized[end]
		if (c >= '0' && c <= '9') || c == '.' {
			end++
			continue
		}
		break
	}

	token := strings.Trim(normalized[start:end], ".")
	if strings.Count(token, ".") < 2 {
		return triplet, false
	}

	parts := strings.Split(token, ".")
	if len(parts) < 3 {
		return triplet, false
	}
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return triplet, false
		}
		triplet[i] = n
	}
	return triplet, true
}

func compareSemverTriplet(left, right [3]int) int {
	for i := 0; i < 3; i++ {
		if left[i] > right[i] {
			return 1
		}
		if left[i] < right[i] {
			return -1
		}
	}
	return 0
}

func serviceClientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func serviceAttemptKey(r *http.Request, email string) string {
	return strings.ToLower(strings.TrimSpace(serviceClientIP(r) + "|" + email))
}

func serviceIsLoginBlocked(key string) (bool, time.Duration) {
	serviceLoginAttemptsMu.Lock()
	defer serviceLoginAttemptsMu.Unlock()

	attempt, ok := serviceLoginAttempts[key]
	if !ok {
		return false, 0
	}
	if attempt.LockedUntil.After(time.Now()) {
		return true, time.Until(attempt.LockedUntil)
	}
	if !attempt.LockedUntil.IsZero() {
		delete(serviceLoginAttempts, key)
	}
	return false, 0
}

func serviceRecordLoginFailure(key string) {
	serviceLoginAttemptsMu.Lock()
	defer serviceLoginAttemptsMu.Unlock()

	now := time.Now()
	attempt := serviceLoginAttempts[key]
	if attempt.FirstFail.IsZero() || now.Sub(attempt.FirstFail) > serviceFailureWindow {
		attempt = serviceLoginAttempt{Failures: 0, FirstFail: now}
	}
	attempt.Failures++
	if attempt.Failures >= serviceMaxFailedAttempts {
		attempt.LockedUntil = now.Add(serviceLockDuration)
	}
	serviceLoginAttempts[key] = attempt
}

func serviceClearLoginAttempts(key string) {
	serviceLoginAttemptsMu.Lock()
	defer serviceLoginAttemptsMu.Unlock()
	delete(serviceLoginAttempts, key)
}

func serviceAuthCookieName() string {
	value := strings.TrimSpace(os.Getenv("AURAPANEL_AUTH_COOKIE_NAME"))
	if value == "" {
		return defaultAuthCookie
	}
	return value
}

func serviceRequestSecure(r *http.Request) bool {
	if strings.EqualFold(forwardedHeaderValue(r.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}
	return r.TLS != nil
}

func setServiceAuthCookie(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) {
	seconds := int(ttl / time.Second)
	if seconds < 0 {
		seconds = 0
	}
	http.SetCookie(w, &http.Cookie{
		Name:     serviceAuthCookieName(),
		Value:    strings.TrimSpace(token),
		Path:     "/",
		HttpOnly: true,
		Secure:   serviceRequestSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   seconds,
		Expires:  time.Now().UTC().Add(ttl),
	})
}

func clearServiceAuthCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     serviceAuthCookieName(),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   serviceRequestSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
	})
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
	attemptKey := serviceAttemptKey(r, email)
	if blocked, remaining := serviceIsLoginBlocked(attemptKey); blocked {
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("Too many failed attempts. Try again in %s.", remaining.Round(time.Second)))
		return
	}

	s.mu.RLock()
	var (
		matchedUser    PanelUser
		matchedOK      bool
		totpSecret     string
		rolePolicyID   string
		rolePolicyName string
		permissions    []string
	)
	for i := range s.state.Users {
		user := s.state.Users[i]
		if strings.EqualFold(user.Email, email) || strings.EqualFold(user.Username, email) {
			matchedUser = user
			totpSecret = s.state.TwoFASecrets[user.Username]
			rolePolicyID, rolePolicyName, permissions = s.resolveUserACLLocked(user)
			matchedOK = true
			break
		}
	}
	s.mu.RUnlock()

	if !matchedOK {
		serviceRecordLoginFailure(attemptKey)
		writeError(w, http.StatusUnauthorized, "Invalid credentials.")
		return
	}
	if !matchedUser.Active {
		writeError(w, http.StatusForbidden, "Account is inactive.")
		return
	}
	if matchedUser.TwoFAEnabled && !verifyStoredTOTPSecret(totpSecret, strings.TrimSpace(payload.TOTPToken), time.Now().UTC()) {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"status":       "error",
			"message":      "2FA code is required.",
			"requires_2fa": true,
		})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(matchedUser.PasswordHash), []byte(password)) != nil {
		serviceRecordLoginFailure(attemptKey)
		writeError(w, http.StatusUnauthorized, "Invalid credentials.")
		return
	}
	serviceClearLoginAttempts(attemptKey)

	token, err := issueToken(matchedUser, permissions, rolePolicyID, rolePolicyName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Token generation failed.")
		return
	}
	setServiceAuthCookie(w, r, token, defaultJWTSessionTTL)
	s.registerDBToolAccess(matchedUser.Email, serviceClientIP(r), time.Now().UTC().Add(defaultJWTSessionTTL))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"token":  token,
		"user":   buildAuthUserPayload(matchedUser, permissions, rolePolicyID, rolePolicyName),
	})
}

func (s *service) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}
	s.revokeDBToolAccess(principal.Email, serviceClientIP(r))
	clearServiceAuthCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
	})
}

func (s *service) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	s.mu.RLock()
	for _, user := range s.state.Users {
		if strings.EqualFold(strings.TrimSpace(user.Email), strings.TrimSpace(principal.Email)) ||
			sanitizeName(user.Username) == sanitizeName(principal.Username) {
			policyID, policyName, permissions := s.resolveUserACLLocked(user)
			s.mu.RUnlock()
			writeJSON(w, http.StatusOK, apiResponse{
				Status: "success",
				Data:   buildAuthUserPayload(user, permissions, policyID, policyName),
			})
			return
		}
	}
	s.mu.RUnlock()

	// Fallback for principals that are valid in headers but not present in runtime users list.
	fallbackUser := PanelUser{
		Username: firstNonEmpty(principal.Username, strings.Split(strings.ToLower(principal.Email), "@")[0]),
		Name:     firstNonEmpty(principal.Name, principal.Username),
		Email:    strings.ToLower(strings.TrimSpace(principal.Email)),
		Role:     normalizeRole(principal.Role),
		Active:   true,
	}
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data:   buildAuthUserPayload(fallbackUser, principal.Permissions, principal.RolePolicyID, principal.RolePolicy),
	})
}

func adminPrincipalFromContext(w http.ResponseWriter, r *http.Request) (servicePrincipal, bool) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return servicePrincipal{}, false
	}
	if normalizeRole(principal.Role) != "admin" {
		writeError(w, http.StatusForbidden, "This endpoint is restricted to admin users.")
		return servicePrincipal{}, false
	}
	return principal, true
}

func (s *service) handleResellerTokenGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := adminPrincipalFromContext(w, r); !ok {
		return
	}

	s.mu.RLock()
	token := strings.TrimSpace(s.state.ResellerToken)
	savedAt := s.state.ResellerTokenSavedAt
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "success",
		"token":    token,
		"saved_at": savedAt,
	})
}

func (s *service) handleResellerTokenSet(w http.ResponseWriter, r *http.Request) {
	if _, ok := adminPrincipalFromContext(w, r); !ok {
		return
	}

	var payload struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid reseller token payload.")
		return
	}
	token := strings.TrimSpace(payload.Token)
	if token == "" {
		writeError(w, http.StatusBadRequest, "Token cannot be empty.")
		return
	}
	if len(token) > 4096 {
		writeError(w, http.StatusBadRequest, "Token is too long.")
		return
	}

	now := time.Now().UTC().UnixMilli()
	s.mu.Lock()
	prevToken := s.state.ResellerToken
	prevSavedAt := s.state.ResellerTokenSavedAt
	s.state.ResellerToken = token
	s.state.ResellerTokenSavedAt = now
	if err := s.saveRuntimeStateLocked(); err != nil {
		s.state.ResellerToken = prevToken
		s.state.ResellerTokenSavedAt = prevSavedAt
		s.mu.Unlock()
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to persist reseller token: %v", err))
		return
	}
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "success",
		"message":  "Reseller token saved successfully.",
		"token":    token,
		"saved_at": now,
	})
}

func (s *service) handleResellerTokenDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := adminPrincipalFromContext(w, r); !ok {
		return
	}

	s.mu.Lock()
	prevToken := s.state.ResellerToken
	prevSavedAt := s.state.ResellerTokenSavedAt
	s.state.ResellerToken = ""
	s.state.ResellerTokenSavedAt = 0
	if err := s.saveRuntimeStateLocked(); err != nil {
		s.state.ResellerToken = prevToken
		s.state.ResellerTokenSavedAt = prevSavedAt
		s.mu.Unlock()
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to remove reseller token: %v", err))
		return
	}
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Reseller token deleted successfully.",
	})
}

func (s *service) findUserByEmailLocked(email string) *PanelUser {
	needle := strings.ToLower(strings.TrimSpace(email))
	if needle == "" {
		return nil
	}
	for i := range s.state.Users {
		if strings.EqualFold(strings.TrimSpace(s.state.Users[i].Email), needle) {
			return &s.state.Users[i]
		}
	}
	return nil
}

func principalAliases(pr servicePrincipal) map[string]struct{} {
	ids := map[string]struct{}{}
	add := func(value string) {
		value = sanitizeName(value)
		if value != "" {
			ids[value] = struct{}{}
		}
	}
	add(pr.Username)
	local := strings.Split(strings.ToLower(strings.TrimSpace(pr.Email)), "@")
	if len(local) > 0 {
		add(local[0])
	}
	return ids
}

func principalDefaultOwner(pr servicePrincipal) string {
	if value := sanitizeName(pr.Username); value != "" {
		return value
	}
	local := strings.Split(strings.ToLower(strings.TrimSpace(pr.Email)), "@")
	if len(local) > 0 {
		if value := sanitizeName(local[0]); value != "" {
			return value
		}
	}
	return ""
}

func (s *service) principalScopedUsernamesLocked(pr servicePrincipal) map[string]struct{} {
	ids := principalAliases(pr)
	expansionRoots := map[string]struct{}{}
	if username := sanitizeName(pr.Username); username != "" {
		ids[username] = struct{}{}
		expansionRoots[username] = struct{}{}
	}
	if user := s.findUserByEmailLocked(pr.Email); user != nil {
		username := sanitizeName(user.Username)
		if username != "" {
			ids[username] = struct{}{}
			expansionRoots[username] = struct{}{}
		}
	}
	if normalizeRole(pr.Role) != "reseller" {
		return ids
	}

	queue := make([]string, 0, len(expansionRoots))
	for root := range expansionRoots {
		queue = append(queue, root)
	}
	visited := map[string]struct{}{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, seen := visited[current]; seen {
			continue
		}
		visited[current] = struct{}{}
		for _, user := range s.state.Users {
			parent := sanitizeName(user.ParentUsername)
			child := sanitizeName(user.Username)
			if child == "" || parent == "" || parent != current {
				continue
			}
			if _, known := ids[child]; !known {
				ids[child] = struct{}{}
				queue = append(queue, child)
			}
		}
	}
	return ids
}

func (s *service) principalCanManageOwnerLocked(pr servicePrincipal, owner string) bool {
	if normalizeRole(pr.Role) == "admin" {
		return true
	}
	owner = sanitizeName(owner)
	if owner == "" {
		return false
	}
	ids := s.principalScopedUsernamesLocked(pr)
	_, ok := ids[owner]
	return ok
}

func (s *service) principalCanManageUserLocked(pr servicePrincipal, target PanelUser) bool {
	if normalizeRole(pr.Role) == "admin" {
		return true
	}
	if normalizeRole(pr.Role) != "reseller" {
		return false
	}
	if normalizeRole(target.Role) == "admin" || target.IsOwner {
		return false
	}
	ids := s.principalScopedUsernamesLocked(pr)
	_, ok := ids[sanitizeName(target.Username)]
	return ok
}

func (s *service) principalOwnsWebsiteLocked(pr servicePrincipal, site Website) bool {
	if pr.Role == "admin" {
		return true
	}
	ids := s.principalScopedUsernamesLocked(pr)
	owner := sanitizeName(site.Owner)
	if _, ok := ids[owner]; ok && owner != "" {
		return true
	}
	siteUser := sanitizeName(site.User)
	if _, ok := ids[siteUser]; ok && siteUser != "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(site.Email), strings.TrimSpace(pr.Email))
}

func (s *service) canAccessDomainLocked(pr servicePrincipal, domain string) bool {
	if pr.Role == "admin" {
		return true
	}
	site := s.findWebsiteLocked(normalizeDomain(domain))
	if site == nil {
		return false
	}
	return s.principalOwnsWebsiteLocked(pr, *site)
}

func (s *service) defaultOwnerLocked() string {
	for _, user := range s.state.Users {
		if normalizeRole(user.Role) == "admin" {
			if value := sanitizeName(user.Username); value != "" {
				return value
			}
		}
	}
	if len(s.state.Users) > 0 {
		if value := sanitizeName(s.state.Users[0].Username); value != "" {
			return value
		}
	}
	return "admin"
}

func (s *service) resolveRequestedOwner(r *http.Request, values ...string) string {
	principal, hasPrincipal := principalFromContext(r.Context())
	for _, value := range values {
		if normalized := sanitizeName(value); normalized != "" {
			if hasPrincipal && normalizeRole(principal.Role) != "admin" {
				s.mu.RLock()
				allowed := s.principalCanManageOwnerLocked(principal, normalized)
				s.mu.RUnlock()
				if !allowed {
					continue
				}
			}
			return normalized
		}
	}
	if hasPrincipal {
		if candidate := principalDefaultOwner(principal); candidate != "" {
			return candidate
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.defaultOwnerLocked()
}

func (s *service) packageByNameLocked(name string) (Package, bool) {
	needle := strings.TrimSpace(name)
	if needle == "" {
		return Package{}, false
	}
	for _, pkg := range s.state.Packages {
		if strings.EqualFold(pkg.Name, needle) {
			return pkg, true
		}
	}
	return Package{}, false
}

func (s *service) ownerPackageLocked(owner string) (Package, bool) {
	owner = sanitizeName(owner)
	packageName := "default"
	if user := s.findUserLocked(owner); user != nil {
		packageName = firstNonEmpty(strings.TrimSpace(user.Package), "default")
	}
	if pkg, ok := s.packageByNameLocked(packageName); ok {
		return pkg, true
	}
	if pkg, ok := s.packageByNameLocked("default"); ok {
		return pkg, true
	}
	return Package{}, false
}

func (s *service) ownerIsUnlimitedLocked(owner string) bool {
	owner = sanitizeName(owner)
	if owner == "" {
		return false
	}
	if user := s.findUserLocked(owner); user != nil {
		return user.IsOwner || normalizeRole(user.Role) == "admin"
	}
	return owner == "admin"
}

func ownerMatches(candidate, owner string) bool {
	return sanitizeName(candidate) == sanitizeName(owner)
}

func (s *service) ownerWebsiteCountLocked(owner string) int {
	count := 0
	for _, site := range s.state.Websites {
		if ownerMatches(firstNonEmpty(site.Owner, site.User), owner) {
			count++
		}
	}
	return count
}

func (s *service) ownerDatabaseCountLocked(owner string) int {
	count := 0
	for _, item := range s.state.MariaDBs {
		if ownerMatches(item.Owner, owner) {
			count++
		}
	}
	for _, item := range s.state.PostgresDBs {
		if ownerMatches(item.Owner, owner) {
			count++
		}
	}
	return count
}

func (s *service) ownerEmailCountLocked(owner string) int {
	count := 0
	for _, mailbox := range s.modules.Mailboxes {
		if ownerMatches(mailbox.Owner, owner) {
			count++
		}
	}
	return count
}

func (s *service) enforceOwnerDomainsLimitLocked(owner string) error {
	if s.ownerIsUnlimitedLocked(owner) {
		return nil
	}
	pkg, ok := s.ownerPackageLocked(owner)
	if !ok || pkg.Domains <= 0 {
		return nil
	}
	if s.ownerWebsiteCountLocked(owner) >= pkg.Domains {
		return fmt.Errorf("package domain limit reached for owner %q (%d)", owner, pkg.Domains)
	}
	return nil
}

func (s *service) enforceOwnerDatabasesLimitLocked(owner string) error {
	if s.ownerIsUnlimitedLocked(owner) {
		return nil
	}
	pkg, ok := s.ownerPackageLocked(owner)
	if !ok || pkg.Databases <= 0 {
		return nil
	}
	if s.ownerDatabaseCountLocked(owner) >= pkg.Databases {
		return fmt.Errorf("package database limit reached for owner %q (%d)", owner, pkg.Databases)
	}
	return nil
}

func (s *service) enforceOwnerEmailsLimitLocked(owner string) error {
	if s.ownerIsUnlimitedLocked(owner) {
		return nil
	}
	pkg, ok := s.ownerPackageLocked(owner)
	if !ok || pkg.Emails <= 0 {
		return nil
	}
	if s.ownerEmailCountLocked(owner) >= pkg.Emails {
		return fmt.Errorf("package email limit reached for owner %q (%d)", owner, pkg.Emails)
	}
	return nil
}

func (s *service) principalCanAccessDatabaseLocked(pr servicePrincipal, item DatabaseRecord) bool {
	if pr.Role == "admin" {
		return true
	}
	if normalizeDomain(item.SiteDomain) != "" && s.canAccessDomainLocked(pr, item.SiteDomain) {
		return true
	}
	ids := s.principalScopedUsernamesLocked(pr)
	owner := sanitizeName(item.Owner)
	_, ok := ids[owner]
	return owner != "" && ok
}

func (s *service) requireDomainAccess(w http.ResponseWriter, r *http.Request, domain string) bool {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return false
	}
	if principal.Role == "admin" {
		return true
	}
	s.mu.RLock()
	allowed := s.canAccessDomainLocked(principal, domain)
	s.mu.RUnlock()
	if !allowed {
		writeError(w, http.StatusForbidden, "Access denied for this domain.")
		return false
	}
	return true
}

func (s *service) handleVhostList(w http.ResponseWriter, r *http.Request) {
	search := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("search")))
	php := strings.TrimSpace(r.URL.Query().Get("php"))
	page := maxInt(1, queryInt(r, "page", 1))
	perPage := clampInt(queryInt(r, "per_page", 20), 1, 200)
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []Website
	for _, site := range s.state.Websites {
		if !s.principalOwnsWebsiteLocked(principal, site) {
			continue
		}
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
	if !isValidDomainName(domain) {
		writeError(w, http.StatusBadRequest, "A valid domain is required.")
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

	owner := s.resolveRequestedOwner(r, payload.Owner, payload.User)

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

	email := firstNonEmpty(strings.TrimSpace(payload.Email), fmt.Sprintf("webmaster@%s", domain))
	phpVersion := firstNonEmpty(strings.TrimSpace(payload.PHPVersion), "8.3")
	if err := s.enforceOwnerDomainsLimitLocked(owner); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

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
	s.ensureDefaultFTPAccountLocked(site)

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
	if !isValidDomainName(domain) {
		writeError(w, http.StatusBadRequest, "A valid domain is required.")
		return
	}
	if !s.requireDomainAccess(w, r, domain) {
		return
	}
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
	if !isValidDomainName(domain) {
		writeError(w, http.StatusBadRequest, "A valid domain is required.")
		return
	}
	if !s.requireDomainAccess(w, r, domain) {
		return
	}
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

func (s *service) handleUsersList(w http.ResponseWriter, r *http.Request) {
	principal, hasPrincipal := principalFromContext(r.Context())
	s.mu.RLock()
	defer s.mu.RUnlock()

	if hasPrincipal && normalizeRole(principal.Role) != "admin" {
		if normalizeRole(principal.Role) != "reseller" {
			writeError(w, http.StatusForbidden, "Only reseller or admin can list users.")
			return
		}
		filtered := make([]PanelUser, 0, len(s.state.Users))
		for _, user := range s.state.Users {
			if !s.principalCanManageUserLocked(principal, user) {
				continue
			}
			filtered = append(filtered, user)
		}
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.publicUsersLocked(filtered)})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.publicUsersLocked(s.state.Users)})
}

func (s *service) handleUsersCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Username       string `json:"username"`
		Email          string `json:"email"`
		Password       string `json:"password"`
		Role           string `json:"role"`
		Package        string `json:"package"`
		ParentUsername string `json:"parent_username"`
		RolePolicyID   string `json:"role_policy_id"`
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
	if username == "" {
		writeError(w, http.StatusBadRequest, "Username must include at least one alphanumeric character.")
		return
	}
	email := strings.ToLower(strings.TrimSpace(payload.Email))
	if s.findUserLocked(username) != nil {
		writeError(w, http.StatusConflict, "User already exists.")
		return
	}
	for i := range s.state.Users {
		if strings.EqualFold(strings.TrimSpace(s.state.Users[i].Email), email) {
			writeError(w, http.StatusConflict, "Email is already in use by another user.")
			return
		}
	}

	rawPassword := strings.TrimSpace(payload.Password)
	if rawPassword == "" {
		writeError(w, http.StatusBadRequest, "Password is required.")
		return
	}
	passwordHash := mustHashPassword(rawPassword)
	principal, hasPrincipal := principalFromContext(r.Context())
	role := normalizeRole(payload.Role)
	parentUsername := ""
	parentCandidate := strings.TrimSpace(payload.ParentUsername)
	if hasPrincipal && normalizeRole(principal.Role) != "admin" {
		if normalizeRole(principal.Role) != "reseller" {
			writeError(w, http.StatusForbidden, "Only reseller or admin can create users.")
			return
		}
		if role == "admin" {
			writeError(w, http.StatusForbidden, "Reseller cannot create admin users.")
			return
		}
		if parentCandidate == "" {
			parentCandidate = principalDefaultOwner(principal)
		}
	}
	if role != "admin" {
		resolvedParent, err := s.resolveParentUsernameLocked(username, parentCandidate)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if hasPrincipal && normalizeRole(principal.Role) != "admin" && !s.principalCanManageOwnerLocked(principal, resolvedParent) {
			writeError(w, http.StatusForbidden, "You cannot assign parent outside your scope.")
			return
		}
		parentUsername = resolvedParent
	}
	rolePolicyID := strings.TrimSpace(payload.RolePolicyID)
	if rolePolicyID != "" {
		if !s.policyExistsLocked(rolePolicyID) {
			writeError(w, http.StatusBadRequest, "Selected role policy was not found.")
			return
		}
	}
	user := PanelUser{
		ID:             s.state.NextUserID,
		Username:       username,
		Name:           strings.Title(username),
		Email:          email,
		Role:           role,
		Package:        firstNonEmpty(strings.TrimSpace(payload.Package), "default"),
		ParentUsername: parentUsername,
		RolePolicyID:   rolePolicyID,
		Sites:          0,
		Active:         true,
		TwoFAEnabled:   false,
		PasswordHash:   passwordHash,
	}
	s.state.NextUserID++
	s.state.Users = append(s.state.Users, user)
	s.setACLAssignmentLocked(user.Username, rolePolicyID)

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "User created.", Data: s.publicUserLocked(user)})
}

func (s *service) handleUsersUpdate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Username       string  `json:"username"`
		Name           string  `json:"name"`
		Email          string  `json:"email"`
		Role           string  `json:"role"`
		Package        string  `json:"package"`
		ParentUsername *string `json:"parent_username"`
		RolePolicyID   *string `json:"role_policy_id"`
		Active         *bool   `json:"active"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user update payload.")
		return
	}

	key := sanitizeName(payload.Username)
	if key == "" {
		writeError(w, http.StatusBadRequest, "Username is required.")
		return
	}
	principal, hasPrincipal := principalFromContext(r.Context())

	s.mu.Lock()
	defer s.mu.Unlock()

	index := s.findUserIndexLocked(key)
	if index < 0 {
		writeError(w, http.StatusNotFound, "User not found.")
		return
	}

	current := s.state.Users[index]
	updated := current
	if hasPrincipal && normalizeRole(principal.Role) != "admin" {
		if normalizeRole(principal.Role) != "reseller" {
			writeError(w, http.StatusForbidden, "Only reseller or admin can update users.")
			return
		}
		if !s.principalCanManageUserLocked(principal, current) {
			writeError(w, http.StatusForbidden, "User is outside your management scope.")
			return
		}
	}
	if strings.TrimSpace(updated.RolePolicyID) == "" {
		if policyID, _, _ := s.resolveUserACLLocked(updated); policyID != "" {
			updated.RolePolicyID = policyID
		}
	}

	if name := strings.TrimSpace(payload.Name); name != "" {
		updated.Name = name
	}
	if email := strings.TrimSpace(payload.Email); email != "" {
		updated.Email = email
	}
	if updated.Email == "" {
		writeError(w, http.StatusBadRequest, "Email is required.")
		return
	}
	if role := strings.TrimSpace(payload.Role); role != "" {
		updated.Role = normalizeRole(role)
	}
	if hasPrincipal && normalizeRole(principal.Role) != "admin" && normalizeRole(updated.Role) == "admin" {
		writeError(w, http.StatusForbidden, "Reseller cannot promote users to admin.")
		return
	}
	if normalizeRole(updated.Role) == "admin" {
		updated.ParentUsername = ""
	} else if payload.ParentUsername != nil {
		resolvedParent, err := s.resolveParentUsernameLocked(updated.Username, *payload.ParentUsername)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if hasPrincipal && normalizeRole(principal.Role) != "admin" && !s.principalCanManageOwnerLocked(principal, resolvedParent) {
			writeError(w, http.StatusForbidden, "You cannot assign parent outside your scope.")
			return
		}
		updated.ParentUsername = resolvedParent
	}
	if pkg := strings.TrimSpace(payload.Package); pkg != "" {
		updated.Package = pkg
	}
	if payload.RolePolicyID != nil {
		rolePolicyID := strings.TrimSpace(*payload.RolePolicyID)
		if rolePolicyID != "" && !s.policyExistsLocked(rolePolicyID) {
			writeError(w, http.StatusBadRequest, "Selected role policy was not found.")
			return
		}
		updated.RolePolicyID = rolePolicyID
	}
	if updated.Package == "" {
		updated.Package = "default"
	}
	if payload.Active != nil {
		updated.Active = *payload.Active
	}
	if hasPrincipal && normalizeRole(principal.Role) != "admin" {
		if !s.principalCanManageUserLocked(principal, updated) {
			writeError(w, http.StatusForbidden, "Updated user scope is not allowed.")
			return
		}
	}

	for i := range s.state.Users {
		if i == index {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(s.state.Users[i].Email), updated.Email) {
			writeError(w, http.StatusConflict, "Email is already in use by another user.")
			return
		}
	}

	remainingActiveAdmins := 0
	for i, user := range s.state.Users {
		if i == index {
			user = updated
		}
		if user.Role == "admin" && user.Active {
			remainingActiveAdmins++
		}
	}
	if remainingActiveAdmins == 0 {
		writeError(w, http.StatusForbidden, "At least one active admin user is required.")
		return
	}
	if current.IsOwner {
		if normalizeRole(updated.Role) != "admin" {
			writeError(w, http.StatusForbidden, "Owner account role cannot be changed.")
			return
		}
		if !updated.Active {
			writeError(w, http.StatusForbidden, "Owner account cannot be deactivated.")
			return
		}
		updated.IsOwner = true
	}

	adminEmail, _ := loadAdminSeedCredentials()
	oldEmail := strings.TrimSpace(current.Email)
	newEmail := strings.TrimSpace(updated.Email)
	shouldSyncAdminArtifacts := isSeedAdminUser(current, adminEmail) && !strings.EqualFold(oldEmail, newEmail)
	rollbackEnv := func() {}
	if shouldSyncAdminArtifacts {
		oldEnvEmail := firstNonEmpty(
			readEnvFileValue(adminGatewayEnvPath(), "AURAPANEL_ADMIN_EMAIL"),
			oldEmail,
			defaultAdminEmail,
		)
		oldEnvPassword := firstNonEmpty(
			readEnvFileValue(adminGatewayEnvPath(), "AURAPANEL_ADMIN_PASSWORD"),
			readTrimmedFile(adminInitialPasswordPath()),
		)
		oldEnvHash := firstNonEmpty(
			readEnvFileValue(adminGatewayEnvPath(), "AURAPANEL_ADMIN_PASSWORD_BCRYPT"),
			strings.TrimSpace(current.PasswordHash),
		)
		rollbackEnv = func() {
			_ = syncAdminCredentialArtifacts(oldEnvEmail, oldEnvPassword, oldEnvHash)
		}
		if err := syncAdminCredentialArtifacts(newEmail, oldEnvPassword, strings.TrimSpace(current.PasswordHash)); err != nil {
			writeError(w, http.StatusInternalServerError, "Admin credential artifacts could not be synchronized.")
			return
		}
	}

	s.state.Users[index] = updated
	s.setACLAssignmentLocked(updated.Username, updated.RolePolicyID)
	s.recountSitesLocked()
	if err := s.saveRuntimeStateLocked(); err != nil {
		s.state.Users[index] = current
		s.setACLAssignmentLocked(current.Username, current.RolePolicyID)
		rollbackEnv()
		writeError(w, http.StatusInternalServerError, "User update could not be persisted.")
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "User updated.", Data: s.publicUserLocked(updated)})
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
	principal, hasPrincipal := principalFromContext(r.Context())

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
	if hasPrincipal && normalizeRole(principal.Role) != "admin" {
		if normalizeRole(principal.Role) != "reseller" {
			writeError(w, http.StatusForbidden, "Only reseller or admin can delete users.")
			return
		}
		target := s.state.Users[index]
		if !s.principalCanManageUserLocked(principal, target) {
			writeError(w, http.StatusForbidden, "User is outside your management scope.")
			return
		}
		if sanitizeName(target.Username) == principalDefaultOwner(principal) {
			writeError(w, http.StatusForbidden, "You cannot delete your own account.")
			return
		}
	}

	previousUsers := append([]PanelUser(nil), s.state.Users...)
	previousWebsites := append([]Website(nil), s.state.Websites...)
	previousAssignments := append([]ACLAssignment(nil), s.modules.ACLAssignments...)

	s.state.Users = append(s.state.Users[:index], s.state.Users[index+1:]...)
	s.setACLAssignmentLocked(username, "")
	reassignOwner := s.defaultOwnerLocked()
	for i := range s.state.Websites {
		if s.state.Websites[i].Owner == username || s.state.Websites[i].User == username {
			s.state.Websites[i].Owner = reassignOwner
			s.state.Websites[i].User = reassignOwner
		}
	}
	for i := range s.state.Users {
		if sanitizeName(s.state.Users[i].ParentUsername) == username {
			s.state.Users[i].ParentUsername = ""
		}
	}
	s.recountSitesLocked()
	if err := s.saveRuntimeStateLocked(); err != nil {
		s.state.Users = previousUsers
		s.state.Websites = previousWebsites
		s.modules.ACLAssignments = previousAssignments
		s.recountSitesLocked()
		writeError(w, http.StatusInternalServerError, "User deletion could not be persisted.")
		return
	}
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
	principal, hasPrincipal := principalFromContext(r.Context())

	s.mu.Lock()
	defer s.mu.Unlock()

	user := s.findUserLocked(sanitizeName(payload.Username))
	if user == nil {
		writeError(w, http.StatusNotFound, "User not found.")
		return
	}
	if hasPrincipal && normalizeRole(principal.Role) != "admin" {
		if normalizeRole(principal.Role) != "reseller" {
			writeError(w, http.StatusForbidden, "Only reseller or admin can change user password.")
			return
		}
		if !s.principalCanManageUserLocked(principal, *user) {
			writeError(w, http.StatusForbidden, "User is outside your management scope.")
			return
		}
	}

	adminEmail, _ := loadAdminSeedCredentials()
	oldHash := user.PasswordHash
	newHash := mustHashPassword(payload.NewPassword)
	shouldSyncAdminArtifacts := isSeedAdminUser(*user, adminEmail)
	rollbackEnv := func() {}

	user.PasswordHash = newHash
	if shouldSyncAdminArtifacts {
		oldEmail := firstNonEmpty(
			readEnvFileValue(adminGatewayEnvPath(), "AURAPANEL_ADMIN_EMAIL"),
			strings.TrimSpace(user.Email),
			defaultAdminEmail,
		)
		oldPassword := firstNonEmpty(
			readEnvFileValue(adminGatewayEnvPath(), "AURAPANEL_ADMIN_PASSWORD"),
			readTrimmedFile(adminInitialPasswordPath()),
		)
		oldHashValue := firstNonEmpty(
			readEnvFileValue(adminGatewayEnvPath(), "AURAPANEL_ADMIN_PASSWORD_BCRYPT"),
			oldHash,
		)
		rollbackEnv = func() {
			_ = syncAdminCredentialArtifacts(oldEmail, oldPassword, oldHashValue)
		}
		if err := syncAdminCredentialArtifacts(strings.TrimSpace(user.Email), payload.NewPassword, newHash); err != nil {
			user.PasswordHash = oldHash
			writeError(w, http.StatusInternalServerError, "Admin credential artifacts could not be synchronized.")
			return
		}
	}

	if err := s.saveRuntimeStateLocked(); err != nil {
		user.PasswordHash = oldHash
		rollbackEnv()
		writeError(w, http.StatusInternalServerError, "Password update could not be persisted.")
		return
	}

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

func (s *service) handleDatabaseList(w http.ResponseWriter, r *http.Request, engine string) {
	if _, err := runtimeDatabaseList(engine); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.syncRuntimeDatabaseStateLocked(engine); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	items := []DatabaseRecord{}
	if engine == "mariadb" {
		items = append(items, s.state.MariaDBs...)
	} else {
		items = append(items, s.state.PostgresDBs...)
	}
	if principal.Role != "admin" {
		filtered := make([]DatabaseRecord, 0, len(items))
		for _, item := range items {
			if s.principalCanAccessDatabaseLocked(principal, item) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
}

func (s *service) handleDatabaseUsers(w http.ResponseWriter, r *http.Request, engine string) {
	if _, err := runtimeDatabaseUsers(engine); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.syncRuntimeDatabaseStateLocked(engine); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	users := []DatabaseUser{}
	dbs := []DatabaseRecord{}
	if engine == "mariadb" {
		users = append(users, s.state.MariaUsers...)
		dbs = append(dbs, s.state.MariaDBs...)
	} else {
		users = append(users, s.state.PostgresUsers...)
		dbs = append(dbs, s.state.PostgresDBs...)
	}
	if principal.Role != "admin" {
		allowedDBs := map[string]struct{}{}
		for _, item := range dbs {
			if s.principalCanAccessDatabaseLocked(principal, item) {
				allowedDBs[item.Name] = struct{}{}
			}
		}
		filtered := make([]DatabaseUser, 0, len(users))
		for _, item := range users {
			if item.LinkedDBName == "" {
				continue
			}
			if _, ok := allowedDBs[item.LinkedDBName]; ok {
				filtered = append(filtered, item)
			}
		}
		users = filtered
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: publicDBUsers(users)})
}

func (s *service) handleRemoteAccessList(w http.ResponseWriter, r *http.Request, engine string) {
	if _, err := runtimeRemoteAccessList(engine); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.syncRuntimeDatabaseStateLocked(engine); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	rules := []RemoteAccessRule{}
	dbs := []DatabaseRecord{}
	if engine == "mariadb" {
		rules = append(rules, s.state.MariaRemoteRules...)
		dbs = append(dbs, s.state.MariaDBs...)
	} else {
		rules = append(rules, s.state.PostgresRemoteRules...)
		dbs = append(dbs, s.state.PostgresDBs...)
	}
	if principal.Role != "admin" {
		allowedDBs := map[string]struct{}{}
		for _, item := range dbs {
			if s.principalCanAccessDatabaseLocked(principal, item) {
				allowedDBs[item.Name] = struct{}{}
			}
		}
		filtered := make([]RemoteAccessRule, 0, len(rules))
		for _, item := range rules {
			if _, ok := allowedDBs[item.DBName]; ok {
				filtered = append(filtered, item)
			}
		}
		rules = filtered
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: rules})
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

	owner := sanitizeName(strings.TrimSpace(payload.Owner))
	if owner == "" && normalizeDomain(payload.SiteDomain) != "" {
		s.mu.RLock()
		if site := s.findWebsiteLocked(normalizeDomain(payload.SiteDomain)); site != nil {
			owner = sanitizeName(firstNonEmpty(site.Owner, site.User))
		}
		s.mu.RUnlock()
	}
	if owner == "" {
		owner = s.resolveRequestedOwner(r)
	}
	s.mu.Lock()
	if err := s.enforceOwnerDatabasesLimitLocked(owner); err != nil {
		s.mu.Unlock()
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	s.mu.Unlock()
	if err := createRuntimeDatabase(engine, dbName, dbUser, dbPass); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	db := DatabaseRecord{
		Name:       dbName,
		Size:       "0 MB",
		Tables:     0,
		Engine:     engine,
		Owner:      owner,
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
			DBHost:   normalizeDBHost(user.Host),
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
	payload.DBName = sanitizeDBName(payload.DBName)
	payload.DBUser = sanitizeName(payload.DBUser)
	payload.DBHost = normalizeDBHost(payload.DBHost)
	if payload.Domain == "" || payload.DBName == "" || payload.DBUser == "" {
		writeError(w, http.StatusBadRequest, "Domain, db name and db user are required.")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	payload.Engine = normalizeEngine(payload.Engine)
	payload.LinkedAt = time.Now().UTC().Unix()
	filtered := s.state.DBLinks[:0]
	for _, item := range s.state.DBLinks {
		if item.Domain == payload.Domain &&
			normalizeEngine(item.Engine) == payload.Engine &&
			item.DBName == payload.DBName &&
			item.DBUser == payload.DBUser &&
			normalizeDBHost(item.DBHost) == payload.DBHost {
			continue
		}
		filtered = append(filtered, item)
	}
	s.state.DBLinks = append(filtered, payload)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "DB link created.", Data: payload})
}

func (s *service) handleDBLinksDelete(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	engine := normalizeEngine(r.URL.Query().Get("engine"))
	dbName := sanitizeDBName(r.URL.Query().Get("db_name"))
	dbUser := sanitizeName(r.URL.Query().Get("db_user"))
	dbHostRaw := strings.TrimSpace(r.URL.Query().Get("db_host"))
	dbHost := ""
	if dbHostRaw != "" {
		dbHost = normalizeDBHost(dbHostRaw)
	}
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
		sameUser := dbUser == "" || sanitizeName(item.DBUser) == dbUser
		sameHost := dbHost == "" || normalizeDBHost(item.DBHost) == dbHost
		if !removed && item.Domain == domain && item.DBName == dbName && sameEngine && sameUser && sameHost {
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
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}
	if domain != "" && !s.requireDomainAccess(w, r, domain) {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	var items []DomainAlias
	for _, alias := range s.state.Aliases {
		if domain != "" && alias.Domain != domain {
			continue
		}
		if principal.Role != "admin" && !s.canAccessDomainLocked(principal, alias.Domain) {
			continue
		}
		items = append(items, alias)
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
}

func (s *service) handleMetrics(w http.ResponseWriter) {
	metrics := collectHostMetrics(s.startedAt)
	serverIP := detectPrimaryIPv4()
	if strings.TrimSpace(serverIP) == "" {
		serverIP = "127.0.0.1"
	}

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
			"server_ip":      serverIP,
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
	scheduled, err := executeServiceActionFromPanel(name, action)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if scheduled {
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Service action scheduled."})
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

	s.mu.RLock()
	currentPort := s.state.GatewayPort
	s.mu.RUnlock()

	if payload.Port == currentPort {
		firewallActions := []string{}
		warnings := []string{}
		if payload.OpenFirewall {
			if err := openFirewallPort(payload.Port); err != nil {
				warnings = append(warnings, fmt.Sprintf("Firewall update failed for tcp/%d: %v", payload.Port, err))
			} else {
				firewallActions = append(firewallActions, fmt.Sprintf("Allow tcp/%d on firewall", payload.Port))
			}
		}
		warnings = append(warnings, "Gateway already uses this port.")
		writeJSON(w, http.StatusOK, apiResponse{
			Status:  "success",
			Message: "Gateway port unchanged.",
			Data: map[string]interface{}{
				"gateway_addr":      fmt.Sprintf(":%d", payload.Port),
				"firewall_actions":  firewallActions,
				"warnings":          warnings,
				"restart_scheduled": false,
				"restart_applied":   false,
				"edge_synced":       false,
			},
		})
		return
	}

	result, err := applyPanelPortChange(payload.Port, payload.OpenFirewall)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	s.state.GatewayPort = payload.Port
	if err := s.saveRuntimeStateLocked(); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Runtime state persistence failed: %v", err))
	}
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: "Gateway port updated.",
		Data: map[string]interface{}{
			"gateway_addr":      fmt.Sprintf(":%d", payload.Port),
			"firewall_actions":  result.FirewallActions,
			"warnings":          result.Warnings,
			"restart_scheduled": result.RestartScheduled,
			"restart_applied":   result.RestartApplied,
			"edge_synced":       result.EdgeSynced,
		},
	})
}

func (s *service) handlePanelReverseDomainGet(w http.ResponseWriter) {
	s.mu.RLock()
	gatewayPort := s.state.GatewayPort
	s.mu.RUnlock()

	config := loadPanelEdgeConfig()
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"enabled":          config.Enabled,
			"domain":           config.Domain,
			"vhost_conf_path":  config.VhostConfigPath,
			"gateway_upstream": fmt.Sprintf("127.0.0.1:%d", gatewayPort),
		},
	})
}

func (s *service) handlePanelReverseDomainSet(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Enabled         bool   `json:"enabled"`
		Domain          string `json:"domain"`
		VhostConfigPath string `json:"vhost_conf_path"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid panel reverse domain payload.")
		return
	}

	domain := normalizeDomain(strings.TrimSpace(payload.Domain))
	if payload.Enabled {
		if domain == "" {
			writeError(w, http.StatusBadRequest, "Domain is required when reverse domain mode is enabled.")
			return
		}
		if !isValidDomainName(domain) {
			writeError(w, http.StatusBadRequest, "A valid reverse domain is required.")
			return
		}
	}

	vhostConfigPath := strings.TrimSpace(payload.VhostConfigPath)
	if vhostConfigPath == "" {
		vhostConfigPath = defaultPanelEdgeVhostConfigPath
	}

	s.mu.RLock()
	gatewayPort := s.state.GatewayPort
	s.mu.RUnlock()

	result, err := applyPanelEdgeConfigChange(panelEdgeConfig{
		Enabled:         payload.Enabled,
		Domain:          domain,
		VhostConfigPath: vhostConfigPath,
	}, gatewayPort)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: "Panel reverse domain settings updated.",
		Data: map[string]interface{}{
			"enabled":          payload.Enabled,
			"domain":           domain,
			"vhost_conf_path":  vhostConfigPath,
			"gateway_upstream": fmt.Sprintf("127.0.0.1:%d", gatewayPort),
			"edge_synced":      result.EdgeSynced,
			"warnings":         result.Warnings,
		},
	})
}

func (s *service) handleSecurityStatus(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	role := "user"
	if principal, ok := principalFromContext(r.Context()); ok {
		role = normalizeRole(principal.Role)
	}
	clientIP := serviceClientIP(r)
	if !s.allowSecurityStatusRequest(role, clientIP, now) {
		writeError(w, http.StatusTooManyRequests, "Too many security status requests. Please retry shortly.")
		return
	}

	snapshot := s.cachedSecuritySnapshot(now)
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
			"ddos_guard":               snapshot.DDoSGuardActive,
			"ssh_key_manager":          snapshot.SSHKeyManager,
			"firewall_active":          snapshot.FirewallActive,
			"firewall_manager":         snapshot.FirewallManager,
			"firewall_open_ports":      snapshot.FirewallOpenPorts,
			"apache_backend_available": snapshot.ApacheBackendAvailable,
			"mail_domain_available":    snapshot.MailDomainAvailable,
			"detected_mail_stack":      snapshot.DetectedMailStack,
			"detected_web_stack":       snapshot.DetectedWebStack,
			"cloudlinux_available":     snapshot.CloudLinuxAvailable,
			"cloudlinux_enabled":       snapshot.CloudLinuxEnabled,
			"detected_cloudlinux":      snapshot.DetectedCloudLinux,
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

func (s *service) handleFirewallRuleDeleteAction(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		IPAddress string `json:"ip_address"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid firewall payload.")
		return
	}
	ip := strings.TrimSpace(payload.IPAddress)
	if ip == "" {
		writeError(w, http.StatusBadRequest, "IP address is required.")
		return
	}
	if err := deleteFirewallRuntimeRule(ip); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Firewall rule deleted."})
}

func (s *service) handleFirewallPortRulesList(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: listFirewallRuntimePortRules()})
}

func (s *service) handleFirewallPortRuleCreate(w http.ResponseWriter, r *http.Request) {
	var payload FirewallPortRule
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid firewall port payload.")
		return
	}

	if err := addFirewallRuntimePortRule(payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Firewall port rule added."})
}

func (s *service) handleFirewallPortRuleDelete(w http.ResponseWriter, r *http.Request) {
	portValue := strings.TrimSpace(r.URL.Query().Get("port"))
	port, err := strconv.Atoi(portValue)
	if err != nil || port <= 0 || port > 65535 {
		writeError(w, http.StatusBadRequest, "Valid port is required.")
		return
	}

	protocol := strings.TrimSpace(r.URL.Query().Get("protocol"))
	if protocol == "" {
		writeError(w, http.StatusBadRequest, "Protocol is required.")
		return
	}

	block, err := strconv.ParseBool(strings.TrimSpace(r.URL.Query().Get("block")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Block flag is required.")
		return
	}

	rule := FirewallPortRule{
		Port:     port,
		Protocol: protocol,
		Block:    block,
	}
	if err := deleteFirewallRuntimePortRule(rule); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Firewall port rule deleted."})
}

func (s *service) handleFirewallPortRuleDeleteAction(w http.ResponseWriter, r *http.Request) {
	var payload FirewallPortRule
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid firewall port payload.")
		return
	}
	if err := deleteFirewallRuntimePortRule(payload); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Firewall port rule deleted."})
}

func (s *service) handleSSHKeysList(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok || principal.Role != "admin" {
		writeError(w, http.StatusForbidden, "Only admin can manage SSH keys.")
		return
	}
	user := strings.TrimSpace(r.URL.Query().Get("user"))
	if user == "" {
		user = "root"
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: listAuthorizedKeys(user)})
}

func (s *service) handleSSHKeyCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok || principal.Role != "admin" {
		writeError(w, http.StatusForbidden, "Only admin can manage SSH keys.")
		return
	}
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
	principal, ok := principalFromContext(r.Context())
	if !ok || principal.Role != "admin" {
		writeError(w, http.StatusForbidden, "Only admin can manage SSH keys.")
		return
	}
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
	if !isValidDomainName(domain) {
		writeError(w, http.StatusBadRequest, "A valid domain is required.")
		return
	}
	if !s.requireDomainAccess(w, r, domain) {
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
		Domain  string `json:"domain"`
		Email   string `json:"email,omitempty"`
		Webroot string `json:"webroot,omitempty"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid SSL payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	if !isValidDomainName(domain) {
		writeError(w, http.StatusBadRequest, "A valid domain is required.")
		return
	}
	if !s.requireDomainAccess(w, r, domain) {
		return
	}
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
	siteOwner := ""
	if site := s.findWebsiteLocked(domain); site != nil {
		siteOwner = sanitizeName(firstNonEmpty(site.Owner, site.User))
	}
	if siteOwner == "" {
		siteOwner = s.defaultOwnerLocked()
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
	if !s.requireDomainAccess(w, r, normalizeDomain(payload.Domain)) {
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

func (s *service) resolveParentUsernameLocked(username, parent string) (string, error) {
	parent = sanitizeName(parent)
	username = sanitizeName(username)
	if parent == "" {
		return "", nil
	}
	if username != "" && parent == username {
		return "", fmt.Errorf("Parent user cannot be same as user.")
	}
	parentUser := s.findUserLocked(parent)
	if parentUser == nil {
		return "", fmt.Errorf("Parent user was not found.")
	}
	parentRole := normalizeRole(parentUser.Role)
	if parentRole != "admin" && parentRole != "reseller" {
		return "", fmt.Errorf("Parent user must be admin or reseller.")
	}
	if username != "" && s.wouldCreateParentCycleLocked(username, parent) {
		return "", fmt.Errorf("Parent relationship creates a cycle.")
	}
	return parent, nil
}

func (s *service) wouldCreateParentCycleLocked(username, candidateParent string) bool {
	username = sanitizeName(username)
	candidateParent = sanitizeName(candidateParent)
	if username == "" || candidateParent == "" {
		return false
	}
	current := candidateParent
	visited := map[string]struct{}{}
	for current != "" {
		if current == username {
			return true
		}
		if _, seen := visited[current]; seen {
			return true
		}
		visited[current] = struct{}{}
		user := s.findUserLocked(current)
		if user == nil {
			return false
		}
		current = sanitizeName(user.ParentUsername)
	}
	return false
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

	resolvedPassword := strings.TrimSpace(password)
	autoProvisioned := resolvedPassword == ""
	if autoProvisioned {
		resolvedPassword = generateSecret(18)
	}
	s.state.Users = append(s.state.Users, PanelUser{
		ID:           s.state.NextUserID,
		Username:     key,
		Name:         strings.Title(key),
		Email:        email,
		Role:         normalizeRole(role),
		Package:      firstNonEmpty(pkg, "default"),
		Active:       !autoProvisioned,
		PasswordHash: mustHashPassword(resolvedPassword),
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
	if site.MailDomain {
		s.ensureMailArtifactsLocked(site)
	}
	s.ensureDNSArtifactsLocked(site.Domain, site.MailDomain)
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

	s.upsertDNSRecordLocked(normalizedDomain, DNSRecord{RecordType: "A", Name: "@", Content: serverIP, TTL: 3600})
	s.upsertDNSRecordLocked(normalizedDomain, DNSRecord{RecordType: "A", Name: "www", Content: serverIP, TTL: 3600})
	s.upsertDNSRecordLocked(normalizedDomain, DNSRecord{RecordType: "A", Name: "ftp", Content: serverIP, TTL: 3600})
	s.upsertDNSRecordLocked(normalizedDomain, DNSRecord{RecordType: "A", Name: "panel", Content: serverIP, TTL: 3600})
	s.upsertDNSRecordLocked(normalizedDomain, DNSRecord{RecordType: "TXT", Name: "@", Content: buildSPFRecord(), TTL: 3600})

	if mailDomain {
		s.upsertDNSRecordLocked(normalizedDomain, DNSRecord{RecordType: "A", Name: "mail", Content: serverIP, TTL: 3600})
		s.upsertDNSRecordLocked(normalizedDomain, DNSRecord{RecordType: "MX", Name: "@", Content: fmt.Sprintf("mail.%s", normalizedDomain), TTL: 3600})
		s.upsertDNSRecordLocked(normalizedDomain, DNSRecord{
			RecordType: "TXT",
			Name:       "_dmarc",
			Content:    buildDMARCRecord("quarantine", "mailto:postmaster@"+normalizedDomain, "", "r", "r", 100),
			TTL:        3600,
		})
		if record, ok := s.modules.MailDKIM[normalizedDomain]; ok && strings.TrimSpace(record.PublicKey) != "" {
			s.upsertDNSRecordLocked(normalizedDomain, DNSRecord{
				RecordType: "TXT",
				Name:       firstNonEmpty(strings.TrimSpace(record.Selector), "selector1") + "._domainkey",
				Content:    strings.TrimSpace(record.PublicKey),
				TTL:        3600,
			})
		}
	}

	s.recalcDNSZoneLocked(normalizedDomain)
	if s.modules.DefaultNameservers.NS1 == "" && s.modules.DefaultNameservers.NS2 == "" {
		s.modules.DefaultNameservers = DefaultNameservers{
			NS1: fmt.Sprintf("ns1.%s", normalizedDomain),
			NS2: fmt.Sprintf("ns2.%s", normalizedDomain),
		}
	}
	_ = syncPowerDNSZone(normalizedDomain, s.modules.DNSRecords[normalizedDomain], s.modules.DefaultNameservers.NS1, s.modules.DefaultNameservers.NS2)
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

func transferHomeBelongsToDomain(homeDir, domain string) bool {
	normalizedDomain := normalizeDomain(domain)
	if normalizedDomain == "" {
		return false
	}
	normalizedHome := normalizeVirtualPath(homeDir)
	root := normalizeVirtualPath(fmt.Sprintf("/home/%s", normalizedDomain))
	return normalizedHome == root || strings.HasPrefix(normalizedHome, root+"/")
}

func transferAccountBelongsToDomain(account TransferAccount, domain string) bool {
	if normalizeDomain(account.Domain) == normalizeDomain(domain) {
		return true
	}
	return transferHomeBelongsToDomain(account.HomeDir, domain)
}

func transferUsernameExists(items []TransferAccount, username string) bool {
	key := sanitizeName(username)
	if key == "" {
		return false
	}
	for _, item := range items {
		if sanitizeName(item.Username) == key {
			return true
		}
	}
	return false
}

func buildDefaultFTPUsername(domain string, existing []TransferAccount) string {
	base := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, normalizeDomain(domain))
	base = strings.ToLower(strings.TrimSpace(base))
	if base == "" {
		base = "ftpuser"
	}
	if len(base) > 24 {
		base = base[:24]
	}
	for i := 0; i < 100; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s_%d", base, i+1)
		}
		if !transferUsernameExists(existing, candidate) {
			return candidate
		}
	}
	return sanitizeName(fmt.Sprintf("%s_%s", base, generateSecret(4)))
}

func primaryFTPUsernameForDomain(domain string) string {
	return buildDefaultFTPUsername(domain, nil)
}

func inferTransferDomainFromHomeDir(homeDir string) string {
	normalizedHome := normalizeVirtualPath(homeDir)
	parts := strings.Split(strings.TrimPrefix(normalizedHome, "/"), "/")
	if len(parts) >= 2 && parts[0] == "home" {
		return normalizeDomain(parts[1])
	}
	return ""
}

func (s *service) websiteOwnerForDomainLocked(domain string) string {
	site := s.findWebsiteLocked(domain)
	if site == nil {
		return ""
	}
	return sanitizeName(firstNonEmpty(site.Owner, site.User))
}

func normalizeFTPAccountMetadata(account TransferAccount) TransferAccount {
	if account.Domain == "" {
		account.Domain = inferTransferDomainFromHomeDir(account.HomeDir)
	}
	if account.Domain != "" && sanitizeName(account.Username) == primaryFTPUsernameForDomain(account.Domain) {
		account.Primary = true
	}
	return account
}

func (s *service) ensureDefaultFTPAccountLocked(site Website) {
	normalizedDomain := normalizeDomain(site.Domain)
	if normalizedDomain == "" {
		return
	}
	homeDir := domainDocroot(normalizedDomain)
	ownerHint := sanitizeName(firstNonEmpty(site.Owner, site.User))
	runtimeItems := append([]TransferAccount(nil), s.modules.FTPUsers...)
	if liveItems, err := runtimeTransferAccounts("ftp"); err == nil {
		runtimeItems = mergeTransferMetadata(liveItems, runtimeItems)
	} else {
		log.Printf("ftp account discovery warning for %s: %v", normalizedDomain, err)
	}
	for i := range runtimeItems {
		runtimeItems[i] = normalizeFTPAccountMetadata(runtimeItems[i])
	}
	s.modules.FTPUsers = runtimeItems
	primaryUsername := primaryFTPUsernameForDomain(normalizedDomain)
	for _, item := range runtimeItems {
		if (item.Primary && transferAccountBelongsToDomain(item, normalizedDomain)) || sanitizeName(item.Username) == sanitizeName(primaryUsername) {
			account := item
			account.Domain = normalizedDomain
			account.HomeDir = homeDir
			account.Primary = true
			if account.CreatedAt == 0 {
				account.CreatedAt = time.Now().UTC().Unix()
			}
			s.modules.FTPUsers = append(removeTransferAccountByUsername(s.modules.FTPUsers, account.Username), account)
			return
		}
	}

	account := TransferAccount{
		Username:  buildDefaultFTPUsername(normalizedDomain, runtimeItems),
		Domain:    normalizedDomain,
		HomeDir:   homeDir,
		Primary:   true,
		CreatedAt: time.Now().UTC().Unix(),
	}
	password := generateSecret(20)
	if err := createRuntimeTransferAccount("ftp", account.Username, password, account.HomeDir, ownerHint); err != nil {
		log.Printf("ftp auto-provision failed for %s: %v", normalizedDomain, err)
		return
	}
	s.modules.FTPUsers = append(removeTransferAccountByUsername(s.modules.FTPUsers, account.Username), account)
}

func (s *service) removeDNSArtifactsLocked(domain string) {
	_ = removePowerDNSZone(domain)
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

func (s *service) removeFTPArtifactsLocked(domain string) {
	normalizedDomain := normalizeDomain(domain)
	if normalizedDomain == "" {
		return
	}

	candidates := append([]TransferAccount(nil), s.modules.FTPUsers...)
	if runtimeItems, err := runtimeTransferAccounts("ftp"); err == nil {
		candidates = mergeTransferMetadata(runtimeItems, candidates)
	}

	toDelete := map[string]struct{}{}
	for _, account := range candidates {
		if transferAccountBelongsToDomain(account, normalizedDomain) {
			toDelete[sanitizeName(account.Username)] = struct{}{}
		}
	}

	for username := range toDelete {
		if username == "" {
			continue
		}
		_ = deleteRuntimeTransferAccount("ftp", username)
	}

	filtered := s.modules.FTPUsers[:0]
	for _, account := range s.modules.FTPUsers {
		username := sanitizeName(account.Username)
		if _, ok := toDelete[username]; ok {
			continue
		}
		if transferAccountBelongsToDomain(account, normalizedDomain) {
			continue
		}
		filtered = append(filtered, account)
	}
	s.modules.FTPUsers = filtered
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
	s.removeFTPArtifactsLocked(domain)

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

func issueToken(user PanelUser, permissions []string, rolePolicyID, rolePolicyName string) (string, error) {
	now := time.Now().UTC()
	claims := jwtClaims{
		Email:        user.Email,
		Name:         firstNonEmpty(user.Name, user.Username),
		Role:         normalizeRole(user.Role),
		Username:     sanitizeName(user.Username),
		Permissions:  append([]string(nil), permissions...),
		RolePolicyID: strings.TrimSpace(rolePolicyID),
		RolePolicy:   strings.TrimSpace(rolePolicyName),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.Email,
			Issuer:    envOr("AURAPANEL_JWT_ISSUER", "aurapanel-gateway"),
			Audience:  jwt.ClaimStrings{envOr("AURAPANEL_JWT_AUDIENCE", "aurapanel-ui")},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(defaultJWTSessionTTL)),
		},
	}
	secret := jwtSecret()
	if len(secret) < 32 {
		return "", fmt.Errorf("jwt secret is not configured")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func jwtSecret() string {
	secret := strings.TrimSpace(os.Getenv("AURAPANEL_JWT_SECRET"))
	if secret != "" {
		return secret
	}
	if devSimulationEnabled() {
		return "aurapanel_dev_only_secret_change_me"
	}
	return ""
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
	user.Permissions = nil
	user.RolePolicy = ""
	return user
}

func buildAuthUserPayload(user PanelUser, permissions []string, rolePolicyID, rolePolicyName string) PanelUser {
	item := publicUser(user)
	item.RolePolicyID = strings.TrimSpace(rolePolicyID)
	item.RolePolicy = strings.TrimSpace(rolePolicyName)
	item.Permissions = append([]string(nil), permissions...)
	return item
}

func (s *service) publicUsersLocked(users []PanelUser) []PanelUser {
	out := make([]PanelUser, 0, len(users))
	for _, user := range users {
		out = append(out, s.publicUserLocked(user))
	}
	return out
}

func (s *service) publicUserLocked(user PanelUser) PanelUser {
	policyID, policyName, permissions := s.resolveUserACLLocked(user)
	return buildAuthUserPayload(user, permissions, policyID, policyName)
}

func (s *service) resolveUserACLLocked(user PanelUser) (string, string, []string) {
	policyID := strings.TrimSpace(user.RolePolicyID)
	if policyID == "" {
		for _, assignment := range s.modules.ACLAssignments {
			if sanitizeName(assignment.Username) == sanitizeName(user.Username) {
				policyID = strings.TrimSpace(assignment.PolicyID)
				break
			}
		}
	}
	if policyID == "" {
		return "", "", []string{}
	}
	for _, policy := range s.modules.ACLPolicies {
		if strings.TrimSpace(policy.ID) == policyID {
			return policyID, strings.TrimSpace(policy.Name), append([]string(nil), policy.Permissions...)
		}
	}
	return "", "", []string{}
}

func (s *service) policyExistsLocked(policyID string) bool {
	policyID = strings.TrimSpace(policyID)
	if policyID == "" {
		return false
	}
	for _, item := range s.modules.ACLPolicies {
		if strings.TrimSpace(item.ID) == policyID {
			return true
		}
	}
	return false
}

func (s *service) setACLAssignmentLocked(username, policyID string) {
	username = sanitizeName(username)
	policyID = strings.TrimSpace(policyID)
	if username == "" {
		return
	}

	updated := make([]ACLAssignment, 0, len(s.modules.ACLAssignments))
	for _, item := range s.modules.ACLAssignments {
		if sanitizeName(item.Username) == username {
			continue
		}
		updated = append(updated, item)
	}
	if policyID != "" {
		updated = append(updated, ACLAssignment{
			Username:  username,
			PolicyID:  policyID,
			UpdatedAt: time.Now().UTC().Unix(),
		})
	}
	s.modules.ACLAssignments = updated
}

func (s *service) reconcileUserRolePoliciesLocked() {
	for i := range s.state.Users {
		user := &s.state.Users[i]
		if strings.TrimSpace(user.RolePolicyID) != "" {
			s.setACLAssignmentLocked(user.Username, user.RolePolicyID)
			continue
		}
		for _, assignment := range s.modules.ACLAssignments {
			if sanitizeName(assignment.Username) == sanitizeName(user.Username) {
				user.RolePolicyID = strings.TrimSpace(assignment.PolicyID)
				break
			}
		}
	}
}

func (s *service) ensureOwnerConsistencyLocked() {
	if len(s.state.Users) == 0 {
		return
	}
	ownerFound := false
	for i := range s.state.Users {
		user := &s.state.Users[i]
		if user.IsOwner {
			user.Role = "admin"
			user.Active = true
			ownerFound = true
		}
	}
	if ownerFound {
		return
	}
	for i := range s.state.Users {
		user := &s.state.Users[i]
		if sanitizeName(user.Username) == "admin" && normalizeRole(user.Role) == "admin" {
			user.IsOwner = true
			user.Active = true
			return
		}
	}
	for i := range s.state.Users {
		user := &s.state.Users[i]
		if normalizeRole(user.Role) == "admin" {
			user.IsOwner = true
			user.Active = true
			return
		}
	}
}

func (s *service) ensureUserHierarchyLocked() {
	for i := range s.state.Users {
		user := &s.state.Users[i]
		user.ParentUsername = sanitizeName(user.ParentUsername)
		if normalizeRole(user.Role) == "admin" {
			user.ParentUsername = ""
			continue
		}
		if user.ParentUsername == "" || user.ParentUsername == sanitizeName(user.Username) {
			user.ParentUsername = ""
			continue
		}
		parent := s.findUserLocked(user.ParentUsername)
		if parent == nil {
			user.ParentUsername = ""
			continue
		}
		parentRole := normalizeRole(parent.Role)
		if parentRole != "admin" && parentRole != "reseller" {
			user.ParentUsername = ""
			continue
		}
		if s.wouldCreateParentCycleLocked(user.Username, user.ParentUsername) {
			user.ParentUsername = ""
		}
	}
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

func isValidDomainName(value string) bool {
	domain := normalizeDomain(value)
	if domain == "" || len(domain) > 253 {
		return false
	}
	if strings.Contains(domain, "/") || strings.Contains(domain, "\\") || strings.Contains(domain, "..") {
		return false
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			isLetter := c >= 'a' && c <= 'z'
			isDigit := c >= '0' && c <= '9'
			if !isLetter && !isDigit && c != '-' {
				return false
			}
		}
	}
	return true
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

func splitCSV(value string) []string {
	parts := strings.Split(strings.TrimSpace(value), ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
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

func normalizeDBHost(value string) string {
	host := strings.ToLower(strings.TrimSpace(value))
	if host == "" {
		return "localhost"
	}
	return host
}

func envOr(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDurationSeconds(key string, fallback time.Duration) time.Duration {
	seconds := envInt(key, 0)
	if seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
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

func adminGatewayEnvPath() string {
	return firstNonEmpty(strings.TrimSpace(os.Getenv("AURAPANEL_GATEWAY_ENV_PATH")), "/etc/aurapanel/aurapanel.env")
}

func adminServiceEnvPath() string {
	return firstNonEmpty(strings.TrimSpace(os.Getenv("AURAPANEL_SERVICE_ENV_PATH")), "/etc/aurapanel/aurapanel-service.env")
}

func adminInitialPasswordPath() string {
	return firstNonEmpty(strings.TrimSpace(os.Getenv("AURAPANEL_INITIAL_PASSWORD_FILE")), "/opt/aurapanel/logs/initial_password.txt")
}

func writeAdminPasswordFile(password string) error {
	path := adminInitialPasswordPath()
	password = strings.TrimSpace(password)
	if password == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(password+"\n"), 0o600)
}

func syncAdminCredentialArtifacts(email, password, passwordHash string) error {
	email = firstNonEmpty(strings.TrimSpace(email), defaultAdminEmail)
	password = strings.TrimSpace(password)
	passwordHash = strings.TrimSpace(passwordHash)
	if passwordHash == "" {
		return fmt.Errorf("admin password hash is required")
	}

	updates := map[string]string{
		"AURAPANEL_ADMIN_EMAIL":           email,
		"AURAPANEL_ADMIN_PASSWORD":        password,
		"AURAPANEL_ADMIN_PASSWORD_BCRYPT": passwordHash,
	}
	for _, path := range []string{adminGatewayEnvPath(), adminServiceEnvPath()} {
		if err := writeEnvFileValues(path, updates); err != nil {
			return err
		}
	}
	if err := writeAdminPasswordFile(password); err != nil {
		return err
	}
	_ = os.Setenv("AURAPANEL_ADMIN_EMAIL", email)
	_ = os.Setenv("AURAPANEL_ADMIN_PASSWORD", password)
	_ = os.Setenv("AURAPANEL_ADMIN_PASSWORD_BCRYPT", passwordHash)
	return nil
}

func readTrimmedFile(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func loadAdminSeedCredentials() (string, string) {
	envAdminEmail := strings.TrimSpace(os.Getenv("AURAPANEL_ADMIN_EMAIL"))
	envAdminHash := strings.TrimSpace(os.Getenv("AURAPANEL_ADMIN_PASSWORD_BCRYPT"))
	envAdminPassword := strings.TrimSpace(os.Getenv("AURAPANEL_ADMIN_PASSWORD"))

	adminEmail := firstNonEmpty(
		envAdminEmail,
		readEnvFileValue(adminGatewayEnvPath(), "AURAPANEL_ADMIN_EMAIL"),
		defaultAdminEmail,
	)

	if envAdminHash != "" {
		return adminEmail, envAdminHash
	}
	if envAdminPassword != "" {
		return adminEmail, mustHashPassword(envAdminPassword)
	}

	adminHash := strings.TrimSpace(readEnvFileValue(adminGatewayEnvPath(), "AURAPANEL_ADMIN_PASSWORD_BCRYPT"))
	if adminHash != "" {
		return adminEmail, adminHash
	}

	adminPassword := firstNonEmpty(
		readEnvFileValue(adminGatewayEnvPath(), "AURAPANEL_ADMIN_PASSWORD"),
		readTrimmedFile(adminInitialPasswordPath()),
	)
	if strings.TrimSpace(adminPassword) == "" {
		adminPassword = generateSecret(24)
		_ = writeAdminPasswordFile(adminPassword)
		log.Printf("warning: admin credentials were missing; generated a secure bootstrap password in %s", adminInitialPasswordPath())
	}

	return adminEmail, mustHashPassword(adminPassword)
}

func decodeJSON(r *http.Request, dst interface{}) error {
	defer r.Body.Close()
	limited := io.LimitReader(r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("invalid JSON payload")
	}
	return nil
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
