package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleVhostListFiltersByPrincipalOwnership(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.state.Users = append(svc.state.Users, PanelUser{
		ID:       10,
		Username: "user1",
		Email:    "user1@example.com",
		Name:     "User One",
		Role:     "user",
		Active:   true,
	})
	svc.state.Websites = []Website{
		{Domain: "owned.example.com", Owner: "user1", User: "user1", Email: "webmaster@owned.example.com", Status: "active"},
		{Domain: "other.example.com", Owner: "user2", User: "user2", Email: "webmaster@other.example.com", Status: "active"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vhost/list", nil)
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "user1@example.com",
		Role:     "user",
		Username: "user1",
		Name:     "User One",
	}))
	rec := httptest.NewRecorder()
	svc.handleVhostList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data []Website `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].Domain != "owned.example.com" {
		t.Fatalf("expected only owned domain, got %+v", payload.Data)
	}
}

func TestHandleVhostListIncludesResellerChildTenantSites(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.state.Users = append(svc.state.Users,
		PanelUser{ID: 20, Username: "agency", Email: "agency@example.com", Name: "Agency", Role: "reseller", Active: true},
		PanelUser{ID: 21, Username: "tenant1", Email: "tenant1@example.com", Name: "Tenant One", Role: "user", ParentUsername: "agency", Active: true},
		PanelUser{ID: 22, Username: "tenant2", Email: "tenant2@example.com", Name: "Tenant Two", Role: "user", Active: true},
	)
	svc.state.Websites = []Website{
		{Domain: "child.example.com", Owner: "tenant1", User: "tenant1", Email: "webmaster@child.example.com", Status: "active"},
		{Domain: "other.example.com", Owner: "tenant2", User: "tenant2", Email: "webmaster@other.example.com", Status: "active"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vhost/list", nil)
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "agency@example.com",
		Role:     "reseller",
		Username: "agency",
		Name:     "Agency",
	}))
	rec := httptest.NewRecorder()
	svc.handleVhostList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data []Website `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].Domain != "child.example.com" {
		t.Fatalf("expected only child tenant domain, got %+v", payload.Data)
	}
}

func TestRequireDomainAccessBlocksNonOwner(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.state.Websites = []Website{
		{Domain: "owner.example.com", Owner: "owner", User: "owner", Email: "owner@example.com", Status: "active"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitor/logs/site?domain=owner.example.com", nil)
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "intruder@example.com",
		Role:     "user",
		Username: "intruder",
		Name:     "Intruder",
	}))
	rec := httptest.NewRecorder()
	svc.handleSiteLogs(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner access, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNonAdminRoutePolicyBlocksAdminOnlyRoutes(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/list", nil)
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "user@example.com",
		Role:     "user",
		Username: "user",
		Name:     "User",
	}))
	rec := httptest.NewRecorder()

	if ok := svc.nonAdminRoutePolicy(rec, req); ok {
		t.Fatalf("expected policy to block admin-only route")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestNonAdminRoutePolicyAllowsResellerUsersRoutes(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/list", nil)
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "reseller@example.com",
		Role:     "reseller",
		Username: "reseller",
		Name:     "Reseller",
	}))
	rec := httptest.NewRecorder()

	if ok := svc.nonAdminRoutePolicy(rec, req); !ok {
		t.Fatalf("expected policy to allow reseller users route, got status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNonAdminRoutePolicyAllowsProvisionForMatchingOwner(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	body := `{"domain":"newtenant.example.com","owner":"user","user":"user"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vhost/create", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "user@example.com",
		Role:     "user",
		Username: "user",
		Name:     "User",
	}))
	rec := httptest.NewRecorder()

	if ok := svc.nonAdminRoutePolicy(rec, req); !ok {
		t.Fatalf("expected policy to allow self-owned provisioning, got status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNonAdminRoutePolicyAllowsResellerProvisionForChildOwner(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.state.Users = append(svc.state.Users,
		PanelUser{ID: 50, Username: "agency", Email: "agency@example.com", Name: "Agency", Role: "reseller", Active: true},
		PanelUser{ID: 51, Username: "tenant1", Email: "tenant1@example.com", Name: "Tenant One", Role: "user", ParentUsername: "agency", Active: true},
	)
	body := `{"domain":"newtenant.example.com","owner":"tenant1","user":"tenant1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vhost/create", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "agency@example.com",
		Role:     "reseller",
		Username: "agency",
		Name:     "Agency",
	}))
	rec := httptest.NewRecorder()

	if ok := svc.nonAdminRoutePolicy(rec, req); !ok {
		t.Fatalf("expected policy to allow child-tenant provisioning, got status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNonAdminRoutePolicyBlocksResellerProvisionForForeignOwner(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.state.Users = append(svc.state.Users,
		PanelUser{ID: 60, Username: "agency", Email: "agency@example.com", Name: "Agency", Role: "reseller", Active: true},
		PanelUser{ID: 61, Username: "tenant1", Email: "tenant1@example.com", Name: "Tenant One", Role: "user", ParentUsername: "agency", Active: true},
		PanelUser{ID: 62, Username: "external", Email: "external@example.com", Name: "External", Role: "user", Active: true},
	)
	body := `{"domain":"foreign.example.com","owner":"external","user":"external"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vhost/create", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "agency@example.com",
		Role:     "reseller",
		Username: "agency",
		Name:     "Agency",
	}))
	rec := httptest.NewRecorder()

	if ok := svc.nonAdminRoutePolicy(rec, req); ok {
		t.Fatalf("expected policy to block foreign owner provisioning")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNonAdminRoutePolicyBlocksAIToolsRoutes(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/tools/status", nil)
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "user@example.com",
		Role:     "user",
		Username: "user",
		Name:     "User",
	}))
	rec := httptest.NewRecorder()

	if ok := svc.nonAdminRoutePolicy(rec, req); ok {
		t.Fatalf("expected policy to block AI tools route")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestNonAdminRoutePolicyAllowsFileRoutes(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/list", strings.NewReader(`{"path":"/home"}`))
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "user@example.com",
		Role:     "user",
		Username: "user",
		Name:     "User",
	}))
	rec := httptest.NewRecorder()

	if ok := svc.nonAdminRoutePolicy(rec, req); !ok {
		t.Fatalf("expected policy to allow file route, got status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNonAdminRoutePolicyAllowsDBToolSSORouteWithoutDomain(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/db/tools/phpmyadmin/sso", strings.NewReader(`{"ttl_seconds":120}`))
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "user@example.com",
		Role:     "user",
		Username: "user",
		Name:     "User",
	}))
	rec := httptest.NewRecorder()

	if ok := svc.nonAdminRoutePolicy(rec, req); !ok {
		t.Fatalf("expected policy to allow db tools sso route, got status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNonAdminFilePathOwnershipCheck(t *testing.T) {
	t.Setenv("AURAPANEL_ALLOWED_PATHS", "/home,/var/www,/usr/local/lsws,/etc/letsencrypt,/var/log,/opt/aurapanel")

	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.state.Users = append(svc.state.Users, PanelUser{
		ID:       44,
		Username: "user1",
		Email:    "user1@example.com",
		Name:     "User One",
		Role:     "user",
		Active:   true,
	})
	svc.state.Websites = []Website{
		{Domain: "owned.example.com", Owner: "user1", User: "user1", Email: "user1@example.com", Status: "active"},
		{Domain: "other.example.com", Owner: "user2", User: "user2", Email: "user2@example.com", Status: "active"},
	}

	principal := servicePrincipal{
		Email:    "user1@example.com",
		Role:     "user",
		Username: "user1",
		Name:     "User One",
	}

	if !svc.nonAdminCanAccessManagedFilePath(principal, "/home/owned.example.com/public_html/index.php") {
		t.Fatalf("expected owned path to be allowed")
	}
	if svc.nonAdminCanAccessManagedFilePath(principal, "/home/other.example.com/public_html/index.php") {
		t.Fatalf("expected foreign path to be denied")
	}
}

func TestServiceAuthMiddlewareRejectsMissingProxyTokenInProduction(t *testing.T) {
	t.Setenv("AURAPANEL_DEV_SIMULATION", "")
	t.Setenv("AURAPANEL_INTERNAL_PROXY_TOKEN", "0123456789abcdef0123456789abcdef")

	protected := serviceAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status/metrics", nil)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestServiceAuthMiddlewareAcceptsValidInternalHeaders(t *testing.T) {
	t.Setenv("AURAPANEL_DEV_SIMULATION", "")
	t.Setenv("AURAPANEL_INTERNAL_PROXY_TOKEN", "0123456789abcdef0123456789abcdef")

	protected := serviceAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := principalFromContext(r.Context()); !ok {
			t.Fatalf("expected principal in context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status/metrics", nil)
	req.Header.Set("X-Aura-Proxy-Token", "0123456789abcdef0123456789abcdef")
	req.Header.Set("X-Aura-Auth-Email", "user@example.com")
	req.Header.Set("X-Aura-Auth-Role", "user")
	req.Header.Set("X-Aura-Auth-Name", "User")
	req.Header.Set("X-Aura-Auth-Username", "user")
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}
}
