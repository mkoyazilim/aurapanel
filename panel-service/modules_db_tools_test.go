package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServicePublicOriginPrefersPanelEdgeDomain(t *testing.T) {
	t.Setenv("AURAPANEL_PANEL_EDGE_SINGLE_DOMAIN", "true")
	t.Setenv("AURAPANEL_PANEL_EDGE_DOMAIN", "panel.example.com")

	req := httptest.NewRequest("GET", "http://panel.example.com:8090/api/v1/db/tools/phpmyadmin/sso/consume?token=x", nil)
	req.Host = "panel.example.com:8090"

	origin := servicePublicOrigin(req)
	if origin != "https://panel.example.com" {
		t.Fatalf("expected edge origin, got %q", origin)
	}
}

func TestResolveDBToolBaseURLDropsGatewayPortForPublicHost(t *testing.T) {
	t.Setenv("AURAPANEL_PANEL_EDGE_SINGLE_DOMAIN", "false")
	t.Setenv("AURAPANEL_PANEL_EDGE_DOMAIN", "")
	t.Setenv("AURAPANEL_PHPMYADMIN_BASE_URL", "/phpmyadmin/index.php")

	req := httptest.NewRequest("GET", "http://panel.example.com:8090/api/v1/db/tools/phpmyadmin/sso/consume?token=x", nil)
	req.Host = "panel.example.com:8090"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "panel.example.com:8090")

	target := resolveDBToolBaseURL(req, "phpmyadmin")
	if target != "https://panel.example.com/phpmyadmin/index.php" {
		t.Fatalf("expected public db tool url without gateway port, got %q", target)
	}
}

func TestResolveDBToolBaseURLKeepsLoopbackPortForDev(t *testing.T) {
	t.Setenv("AURAPANEL_PANEL_EDGE_SINGLE_DOMAIN", "false")
	t.Setenv("AURAPANEL_PANEL_EDGE_DOMAIN", "")
	t.Setenv("AURAPANEL_PHPMYADMIN_BASE_URL", "/phpmyadmin/index.php")

	req := httptest.NewRequest("GET", "http://127.0.0.1:8090/api/v1/db/tools/phpmyadmin/sso/consume?token=x", nil)
	req.Host = "127.0.0.1:8090"

	target := resolveDBToolBaseURL(req, "phpmyadmin")
	if target != "http://127.0.0.1:8090/phpmyadmin/index.php" {
		t.Fatalf("expected loopback url to keep port, got %q", target)
	}
}

func TestResolveDomainDBLinkPrefersNewestLink(t *testing.T) {
	svc := &service{
		state: appState{
			DBLinks: []WebsiteDBLink{
				{Domain: "example.com", Engine: "mariadb", DBName: "old_db", DBUser: "old_user", LinkedAt: 10},
				{Domain: "example.com", Engine: "mariadb", DBName: "new_db", DBUser: "new_user", LinkedAt: 20},
			},
		},
	}

	link, err := svc.resolveDomainDBLink("example.com", "mariadb")
	if err != nil {
		t.Fatalf("resolveDomainDBLink returned error: %v", err)
	}
	if link.DBName != "new_db" || link.DBUser != "new_user" {
		t.Fatalf("expected newest DB link, got name=%q user=%q", link.DBName, link.DBUser)
	}
}

func TestResolveDomainDBLinkFallsBackToDatabaseMetadata(t *testing.T) {
	svc := &service{
		state: appState{
			MariaDBs: []DatabaseRecord{
				{Name: "site_db", SiteDomain: "example.com"},
			},
			MariaUsers: []DatabaseUser{
				{Username: "site_user", LinkedDBName: "site_db", Host: "localhost"},
			},
		},
	}

	link, err := svc.resolveDomainDBLink("example.com", "mariadb")
	if err != nil {
		t.Fatalf("resolveDomainDBLink returned error: %v", err)
	}
	if link.DBName != "site_db" || link.DBUser != "site_user" {
		t.Fatalf("expected fallback link from metadata, got name=%q user=%q", link.DBName, link.DBUser)
	}
}

func TestResolvePrincipalMariaDBScopeFiltersOwnedDatabases(t *testing.T) {
	svc := &service{
		state: appState{
			Users: []PanelUser{
				{Username: "tenant", Email: "tenant@example.com", Role: "user", Active: true},
				{Username: "other", Email: "other@example.com", Role: "user", Active: true},
			},
			Websites: []Website{
				{Domain: "tenant.example.com", Owner: "tenant", User: "tenant", Email: "tenant@example.com"},
				{Domain: "other.example.com", Owner: "other", User: "other", Email: "other@example.com"},
			},
			MariaDBs: []DatabaseRecord{
				{Name: "tenant_db", SiteDomain: "tenant.example.com", Owner: "tenant", Engine: "mariadb"},
				{Name: "other_db", SiteDomain: "other.example.com", Owner: "other", Engine: "mariadb"},
				{Name: "tenant_misc", Owner: "tenant", Engine: "mariadb"},
			},
			DBLinks: []WebsiteDBLink{
				{Domain: "tenant.example.com", Engine: "mariadb", DBName: "tenant_db", DBUser: "tenant_user", LinkedAt: 10},
				{Domain: "tenant.example.com", Engine: "mariadb", DBName: "tenant_misc", DBUser: "tenant_misc_user", LinkedAt: 20},
			},
		},
	}

	names, primary, err := svc.resolvePrincipalMariaDBScope(servicePrincipal{
		Email:    "tenant@example.com",
		Role:     "user",
		Username: "tenant",
		Name:     "Tenant",
	})
	if err != nil {
		t.Fatalf("resolvePrincipalMariaDBScope returned error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 scoped databases, got %d (%v)", len(names), names)
	}
	if primary != "tenant_misc" {
		t.Fatalf("expected newest linked database as primary, got %q", primary)
	}

	found := map[string]bool{}
	for _, name := range names {
		found[name] = true
	}
	if !found["tenant_db"] || !found["tenant_misc"] {
		t.Fatalf("expected tenant_db and tenant_misc in scope, got %v", names)
	}
	if found["other_db"] {
		t.Fatalf("did not expect other_db in scope, got %v", names)
	}
}

func TestResolvePrincipalMariaDBScopeErrorsWhenNoAccessibleDatabase(t *testing.T) {
	svc := &service{
		state: appState{
			Users: []PanelUser{
				{Username: "other", Email: "other@example.com", Role: "user", Active: true},
			},
			Websites: []Website{
				{Domain: "other.example.com", Owner: "other", User: "other", Email: "other@example.com"},
			},
			MariaDBs: []DatabaseRecord{
				{Name: "other_db", SiteDomain: "other.example.com", Owner: "other", Engine: "mariadb"},
			},
		},
	}

	_, _, err := svc.resolvePrincipalMariaDBScope(servicePrincipal{
		Email:    "tenant@example.com",
		Role:     "user",
		Username: "tenant",
		Name:     "Tenant",
	})
	if err == nil {
		t.Fatalf("expected error when no scoped mariadb database exists")
	}
}

func TestResolvePrincipalPostgresScopeFiltersOwnedDatabases(t *testing.T) {
	svc := &service{
		state: appState{
			Users: []PanelUser{
				{Username: "tenant", Email: "tenant@example.com", Role: "user", Active: true},
				{Username: "other", Email: "other@example.com", Role: "user", Active: true},
			},
			Websites: []Website{
				{Domain: "tenant.example.com", Owner: "tenant", User: "tenant", Email: "tenant@example.com"},
				{Domain: "other.example.com", Owner: "other", User: "other", Email: "other@example.com"},
			},
			PostgresDBs: []DatabaseRecord{
				{Name: "tenant_pg", SiteDomain: "tenant.example.com", Owner: "tenant", Engine: "postgresql"},
				{Name: "other_pg", SiteDomain: "other.example.com", Owner: "other", Engine: "postgresql"},
				{Name: "tenant_misc_pg", Owner: "tenant", Engine: "postgresql"},
			},
			DBLinks: []WebsiteDBLink{
				{Domain: "tenant.example.com", Engine: "postgresql", DBName: "tenant_pg", DBUser: "tenant_pg_user", LinkedAt: 10},
				{Domain: "tenant.example.com", Engine: "postgresql", DBName: "tenant_misc_pg", DBUser: "tenant_misc_pg_user", LinkedAt: 20},
			},
		},
	}

	names, primary, err := svc.resolvePrincipalPostgresScope(servicePrincipal{
		Email:    "tenant@example.com",
		Role:     "user",
		Username: "tenant",
		Name:     "Tenant",
	})
	if err != nil {
		t.Fatalf("resolvePrincipalPostgresScope returned error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 scoped postgres databases, got %d (%v)", len(names), names)
	}
	if primary != "tenant_misc_pg" {
		t.Fatalf("expected newest linked postgres database as primary, got %q", primary)
	}

	found := map[string]bool{}
	for _, name := range names {
		found[name] = true
	}
	if !found["tenant_pg"] || !found["tenant_misc_pg"] {
		t.Fatalf("expected tenant_pg and tenant_misc_pg in scope, got %v", names)
	}
	if found["other_pg"] {
		t.Fatalf("did not expect other_pg in scope, got %v", names)
	}
}

func TestResolvePrincipalPostgresScopeErrorsWhenNoAccessibleDatabase(t *testing.T) {
	svc := &service{
		state: appState{
			Users: []PanelUser{
				{Username: "other", Email: "other@example.com", Role: "user", Active: true},
			},
			Websites: []Website{
				{Domain: "other.example.com", Owner: "other", User: "other", Email: "other@example.com"},
			},
			PostgresDBs: []DatabaseRecord{
				{Name: "other_pg", SiteDomain: "other.example.com", Owner: "other", Engine: "postgresql"},
			},
		},
	}

	_, _, err := svc.resolvePrincipalPostgresScope(servicePrincipal{
		Email:    "tenant@example.com",
		Role:     "user",
		Username: "tenant",
		Name:     "Tenant",
	})
	if err == nil {
		t.Fatalf("expected error when no scoped postgres database exists")
	}
}

func TestPGAdminScopedEmailForPrincipalDeterministic(t *testing.T) {
	principal := servicePrincipal{
		Email:    "tenant@example.com",
		Username: "tenant",
		Role:     "user",
		Name:     "Tenant",
	}
	first := pgAdminScopedEmailForPrincipal(principal)
	second := pgAdminScopedEmailForPrincipal(principal)

	if first == "" || second == "" {
		t.Fatalf("expected deterministic non-empty scoped email")
	}
	if first != second {
		t.Fatalf("expected deterministic scoped email, got %q and %q", first, second)
	}
	if !strings.HasPrefix(first, "apsso_") {
		t.Fatalf("expected apsso prefix, got %q", first)
	}
	if !strings.Contains(first, "@") {
		t.Fatalf("expected valid email format, got %q", first)
	}
}
