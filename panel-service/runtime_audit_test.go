package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type hijackableResponseWriter struct {
	header http.Header
	conn   net.Conn
	rw     *bufio.ReadWriter
}

func (w *hijackableResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *hijackableResponseWriter) Write(data []byte) (int, error) {
	return len(data), nil
}

func (w *hijackableResponseWriter) WriteHeader(statusCode int) {}

func (w *hijackableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, w.rw, nil
}

func TestStatusCapturingResponseWriterSupportsHijack(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	writer := &hijackableResponseWriter{
		conn: serverConn,
		rw:   bufio.NewReadWriter(bufio.NewReader(strings.NewReader("")), bufio.NewWriter(&bytes.Buffer{})),
	}
	wrapped := &statusCapturingResponseWriter{ResponseWriter: writer, status: http.StatusOK}

	conn, _, err := wrapped.Hijack()
	if err != nil {
		t.Fatalf("Hijack returned error: %v", err)
	}
	if conn != serverConn {
		t.Fatalf("Hijack returned unexpected conn")
	}
}

func TestHandleWebsiteAdvancedConfigGetDoesNotMutateState(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/websites/advanced-config?domain=example.com", nil)
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "admin@server.com",
		Role:     "admin",
		Username: "admin",
		Name:     "Admin",
	}))
	rec := httptest.NewRecorder()
	svc.handleWebsiteAdvancedConfigGet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", rec.Code)
	}
	if got := len(svc.state.AdvancedConfig); got != 0 {
		t.Fatalf("advanced config map mutated during GET: %d entries", got)
	}

	var payload struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data["open_basedir"] != true {
		t.Fatalf("expected default open_basedir=true, got %#v", payload.Data["open_basedir"])
	}
}

func TestHandleOLSTuningSetStagesWithoutApply(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()

	body := strings.NewReader(`{"max_connections":22000,"max_ssl_connections":21000,"conn_timeout_secs":120,"keep_alive_timeout_secs":10,"max_keep_alive_requests":5000,"gzip_compression":true,"static_cache_enabled":false,"static_cache_max_age_secs":120}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ols/tuning", body)
	rec := httptest.NewRecorder()
	svc.handleOLSTuningSet(rec, req, false)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", rec.Code, rec.Body.String())
	}
	if !svc.modules.OLSTuningPending {
		t.Fatalf("expected staged OLS config to remain pending")
	}
	if svc.modules.OLSConfig.MaxConnections != 22000 {
		t.Fatalf("expected staged max_connections to be persisted, got %d", svc.modules.OLSConfig.MaxConnections)
	}

	var payload struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data["pending"] != true {
		t.Fatalf("expected pending=true in response, got %#v", payload.Data["pending"])
	}
}

func TestHandleOLSTuningSetAcceptsFlexiblePayloadTypes(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()

	body := strings.NewReader(`{"max_connections":"24000","max_ssl_connections":"23000","conn_timeout_secs":"90","keep_alive_timeout_secs":"7","max_keep_alive_requests":"5500","gzip_compression":"true","static_cache_enabled":"0","static_cache_max_age_secs":"1800"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ols/tuning", body)
	rec := httptest.NewRecorder()
	svc.handleOLSTuningSet(rec, req, false)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", rec.Code, rec.Body.String())
	}
	if svc.modules.OLSConfig.MaxConnections != 24000 {
		t.Fatalf("unexpected max_connections: %d", svc.modules.OLSConfig.MaxConnections)
	}
	if svc.modules.OLSConfig.StaticCacheEnabled {
		t.Fatalf("expected static cache disabled after flexible payload parse")
	}
}

func TestHandleOLSTuningSetPartialPayloadPreservesExistingValues(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.modules.OLSConfig = OLSTuningConfig{
		MaxConnections:       10000,
		MaxSSLConnections:    9000,
		ConnTimeoutSecs:      200,
		KeepAliveTimeoutSecs: 4,
		MaxKeepAliveRequests: 4000,
		GzipCompression:      false,
		StaticCacheEnabled:   true,
		StaticCacheMaxAgeSec: 1200,
	}

	body := strings.NewReader(`{"max_connections":"28000"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ols/tuning", body)
	rec := httptest.NewRecorder()
	svc.handleOLSTuningSet(rec, req, false)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", rec.Code, rec.Body.String())
	}
	if svc.modules.OLSConfig.MaxConnections != 28000 {
		t.Fatalf("max_connections was not updated: %d", svc.modules.OLSConfig.MaxConnections)
	}
	if svc.modules.OLSConfig.MaxSSLConnections != 9000 {
		t.Fatalf("max_ssl_connections should stay unchanged, got %d", svc.modules.OLSConfig.MaxSSLConnections)
	}
	if svc.modules.OLSConfig.GzipCompression {
		t.Fatalf("gzip_compression should stay unchanged=false")
	}
}

func TestHandleCloudLinuxActionsReturnsCatalog(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()

	rec := httptest.NewRecorder()
	svc.handleCloudLinuxActions(rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Data struct {
			Actions []map[string]interface{} `json:"actions"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data.Actions) == 0 {
		t.Fatalf("expected non-empty CloudLinux action catalog")
	}
}

func TestHandleCloudLinuxActionRunDryRunCreatesAuditEntry(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cloudlinux/actions/run", strings.NewReader(`{"action":"cagefs_force_update","dry_run":true}`))
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "admin@server.com",
		Role:     "admin",
		Username: "admin",
		Name:     "Admin",
	}))
	rec := httptest.NewRecorder()
	svc.handleCloudLinuxActionRun(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Data struct {
			Action string `json:"action"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.Action != "cagefs_force_update" {
		t.Fatalf("unexpected action in response: %s", payload.Data.Action)
	}
	if payload.Data.Status != "dry_run" && payload.Data.Status != "blocked" {
		t.Fatalf("unexpected dry-run status: %s", payload.Data.Status)
	}
	if len(svc.modules.CloudLinuxActions) != 1 {
		t.Fatalf("expected 1 CloudLinux audit entry, got %d", len(svc.modules.CloudLinuxActions))
	}
	if svc.modules.CloudLinuxActions[0].Action != "cagefs_force_update" {
		t.Fatalf("unexpected audit action: %s", svc.modules.CloudLinuxActions[0].Action)
	}
}

func TestHandleCloudLinuxProfilesReturnsPackageProfiles(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.state.Packages = []Package{
		{
			ID:          1,
			Name:        "Starter",
			PlanType:    "hosting",
			CPULimit:    0,
			RamMB:       0,
			IOLimit:     0,
			DiskGB:      10,
			BandwidthGB: 100,
			Domains:     1,
			Databases:   2,
			Emails:      5,
		},
		{
			ID:          2,
			Name:        "Reseller Pro",
			PlanType:    "reseller",
			CPULimit:    350,
			RamMB:       4096,
			IOLimit:     30,
			DiskGB:      100,
			BandwidthGB: 500,
			Domains:     0,
			Databases:   0,
			Emails:      0,
		},
	}
	svc.state.Websites = []Website{
		{Domain: "starter.test", Package: "Starter"},
		{Domain: "reseller.test", Package: "Reseller Pro"},
	}
	svc.state.Users = append(svc.state.Users,
		PanelUser{ID: 20, Username: "starter_user", Role: "user", Package: "Starter"},
		PanelUser{ID: 21, Username: "reseller_user", Role: "reseller", Package: "Reseller Pro"},
	)

	rec := httptest.NewRecorder()
	svc.handleCloudLinuxProfiles(rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Data struct {
			Summary struct {
				TotalPackages        int `json:"total_packages"`
				ProfilesWithDefaults int `json:"profiles_with_defaults"`
			} `json:"summary"`
			Profiles []struct {
				PackageName       string `json:"package_name"`
				UsedCPUDefault    bool   `json:"used_cpu_default"`
				UsedMemoryDefault bool   `json:"used_memory_default"`
				UsedIODefault     bool   `json:"used_io_default"`
				Readiness         string `json:"readiness"`
			} `json:"profiles"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Data.Summary.TotalPackages != 2 {
		t.Fatalf("expected total_packages=2, got %d", payload.Data.Summary.TotalPackages)
	}
	if len(payload.Data.Profiles) != 2 {
		t.Fatalf("expected 2 profile rows, got %d", len(payload.Data.Profiles))
	}
	if payload.Data.Summary.ProfilesWithDefaults < 1 {
		t.Fatalf("expected at least one profile with defaults")
	}

	startedProfileFound := false
	for _, item := range payload.Data.Profiles {
		if strings.EqualFold(item.PackageName, "Starter") {
			startedProfileFound = true
			if !item.UsedCPUDefault || !item.UsedMemoryDefault || !item.UsedIODefault {
				t.Fatalf("expected starter profile to use defaults")
			}
		}
		if strings.TrimSpace(item.Readiness) == "" {
			t.Fatalf("expected non-empty readiness state")
		}
	}
	if !startedProfileFound {
		t.Fatalf("expected starter profile row in response")
	}
}

func TestHandleCloudLinuxRolloutPlanReturnsScopedUsers(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.state.Packages = []Package{
		{ID: 1, Name: "Starter", PlanType: "hosting", CPULimit: 100, RamMB: 1024, IOLimit: 10, Domains: 1},
		{ID: 2, Name: "Agency", PlanType: "reseller", CPULimit: 300, RamMB: 4096, IOLimit: 30, Domains: 20},
	}
	svc.state.Users = append(svc.state.Users,
		PanelUser{ID: 50, Username: "starter_user", Role: "user", Package: "Starter"},
		PanelUser{ID: 51, Username: "agency_owner", Role: "reseller", Package: "Agency"},
	)
	svc.state.Websites = append(svc.state.Websites,
		Website{Domain: "one.test", Owner: "starter_user", Package: "Starter"},
		Website{Domain: "two.test", Owner: "agency_owner", Package: "Agency"},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cloudlinux/rollout/plan?package=Starter", nil)
	rec := httptest.NewRecorder()
	svc.handleCloudLinuxRolloutPlan(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Data struct {
			Summary struct {
				ScopedUsers int    `json:"scoped_users"`
				Package     string `json:"package_filter"`
			} `json:"summary"`
			Users []struct {
				Username    string `json:"username"`
				PackageName string `json:"package_name"`
				CommandHint string `json:"command_hint"`
			} `json:"users"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.Summary.ScopedUsers != 1 {
		t.Fatalf("expected scoped_users=1, got %d", payload.Data.Summary.ScopedUsers)
	}
	if !strings.EqualFold(payload.Data.Summary.Package, "Starter") {
		t.Fatalf("expected package filter Starter, got %q", payload.Data.Summary.Package)
	}
	if len(payload.Data.Users) != 1 {
		t.Fatalf("expected 1 rollout user, got %d", len(payload.Data.Users))
	}
	if !strings.EqualFold(payload.Data.Users[0].Username, "starter_user") {
		t.Fatalf("unexpected rollout username: %s", payload.Data.Users[0].Username)
	}
	if !strings.Contains(payload.Data.Users[0].CommandHint, "lvectl set-user") {
		t.Fatalf("expected lvectl command hint, got %q", payload.Data.Users[0].CommandHint)
	}
}

func TestHandleCloudLinuxRolloutApplyBlocksWhenApplyDisabled(t *testing.T) {
	t.Setenv("AURAPANEL_CLOUDLINUX_APPLY_ENABLED", "false")

	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.state.Users = append(svc.state.Users, PanelUser{
		ID:       70,
		Username: "rollout-user",
		Role:     "user",
		Package:  "default",
		Active:   true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cloudlinux/rollout/apply", strings.NewReader(`{"dry_run":false,"confirm":"APPLY_CLOUDLINUX"}`))
	rec := httptest.NewRecorder()
	svc.handleCloudLinuxRolloutApply(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 when apply mode disabled, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleCloudLinuxRolloutApplyDryRunCreatesAudit(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.state.Users = append(svc.state.Users, PanelUser{
		ID:       71,
		Username: "preview-user",
		Role:     "user",
		Package:  "default",
		Active:   true,
	})
	svc.state.Websites = append(svc.state.Websites, Website{
		Domain:  "preview.test",
		Owner:   "preview-user",
		User:    "preview-user",
		Package: "default",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cloudlinux/rollout/apply", strings.NewReader(`{"dry_run":true,"only_ready":false,"max_users":10}`))
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "admin@server.com",
		Role:     "admin",
		Username: "admin",
		Name:     "Admin",
	}))
	rec := httptest.NewRecorder()
	svc.handleCloudLinuxRolloutApply(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", rec.Code, rec.Body.String())
	}
	if len(svc.modules.CloudLinuxRollouts) != 1 {
		t.Fatalf("expected 1 rollout audit entry, got %d", len(svc.modules.CloudLinuxRollouts))
	}
	if !svc.modules.CloudLinuxRollouts[0].DryRun {
		t.Fatalf("expected dry-run audit entry")
	}

	var payload struct {
		Data struct {
			DryRun  bool `json:"dry_run"`
			Results []struct {
				Username string `json:"username"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Data.DryRun {
		t.Fatalf("expected dry_run=true in response")
	}
	if len(payload.Data.Results) == 0 {
		t.Fatalf("expected rollout preview results")
	}
}

func TestHandleCloudLinuxRolloutHistoryReturnsEntries(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.modules.CloudLinuxRollouts = []cloudLinuxRolloutAuditEntry{
		{
			ID:          "old",
			Status:      "dry_run",
			RequestedAt: 10,
			FinishedAt:  12,
		},
		{
			ID:          "new",
			Status:      "success",
			RequestedAt: 30,
			FinishedAt:  40,
		},
	}

	rec := httptest.NewRecorder()
	svc.handleCloudLinuxRolloutHistory(rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data) != 2 {
		t.Fatalf("expected 2 history rows, got %d", len(payload.Data))
	}
	if payload.Data[0].ID != "new" {
		t.Fatalf("expected newest entry first, got %s", payload.Data[0].ID)
	}
}

func TestHandleVhostCreateRollsBackStateOnProvisionFailure(t *testing.T) {
	if fileExists(olsHTTPDConfigPath) {
		t.Skip("OpenLiteSpeed runtime detected on host; rollback failure path is not deterministic here")
	}

	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()

	initialWebsites := len(svc.state.Websites)
	initialUsers := len(svc.state.Users)

	body := strings.NewReader(`{"domain":"rollback-check.example","owner":"ghostuser","php_version":"8.3"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vhost/create", body)
	rec := httptest.NewRecorder()
	svc.handleVhostCreate(rec, req)

	if rec.Code == http.StatusOK {
		t.Skip("website provisioning succeeded on this host; rollback path was not exercised")
	}
	if len(svc.state.Websites) != initialWebsites {
		t.Fatalf("website state leaked after failed provisioning")
	}
	if len(svc.state.Users) != initialUsers {
		t.Fatalf("user state leaked after failed provisioning")
	}
	if user := svc.findUserLocked("ghostuser"); user != nil {
		t.Fatalf("ghostuser still exists after rollback")
	}
}

func TestRuntimeStatePersistsUserPasswordHashes(t *testing.T) {
	t.Setenv("AURAPANEL_STATE_FILE", filepath.Join(t.TempDir(), "panel-service-state.json"))

	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()

	expectedHash := mustHashPassword("super-secret")
	svc.state.Users = append(svc.state.Users, PanelUser{
		ID:           99,
		Username:     "tenant1",
		Name:         "Tenant One",
		Email:        "tenant1@example.com",
		Role:         "user",
		Package:      "default",
		Sites:        0,
		Active:       true,
		TwoFAEnabled: false,
		PasswordHash: expectedHash,
	})

	if err := svc.saveRuntimeStateLocked(); err != nil {
		t.Fatalf("saveRuntimeStateLocked: %v", err)
	}

	restored := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	if err := restored.loadRuntimeState(); err != nil {
		t.Fatalf("loadRuntimeState: %v", err)
	}

	user := restored.findUserLocked("tenant1")
	if user == nil {
		t.Fatalf("expected tenant user after reload")
	}
	if user.PasswordHash != expectedHash {
		t.Fatalf("expected persisted password hash to survive reload")
	}
}

func TestLoadRuntimeStateRehydratesSeedAdminPasswordHash(t *testing.T) {
	t.Setenv("AURAPANEL_STATE_FILE", filepath.Join(t.TempDir(), "panel-service-state.json"))
	t.Setenv("AURAPANEL_ADMIN_EMAIL", "admin@server.com")
	t.Setenv("AURAPANEL_ADMIN_PASSWORD", "rehydrated-secret")

	state := seedState()
	state.Users[0].PasswordHash = ""

	raw, err := json.Marshal(persistedRuntimeState{
		State:   state,
		Modules: seedModuleState(),
	})
	if err != nil {
		t.Fatalf("marshal runtime state: %v", err)
	}
	if err := os.WriteFile(runtimeStatePath(), raw, 0o600); err != nil {
		t.Fatalf("write runtime state: %v", err)
	}

	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	if err := svc.loadRuntimeState(); err != nil {
		t.Fatalf("loadRuntimeState: %v", err)
	}

	admin := svc.findUserLocked("admin")
	if admin == nil {
		t.Fatalf("expected admin user after reload")
	}
	if strings.TrimSpace(admin.PasswordHash) == "" {
		t.Fatalf("expected admin password hash to be rehydrated")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte("rehydrated-secret")); err != nil {
		t.Fatalf("expected rehydrated admin password hash to match env password: %v", err)
	}
}

func seedTime() time.Time {
	return time.Unix(0, 0).UTC()
}
