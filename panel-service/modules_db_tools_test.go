package main

import (
	"net/http/httptest"
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
