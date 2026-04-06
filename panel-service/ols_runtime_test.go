package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRenderOLSManagedListenerMapBlockKeepsExampleFallback(t *testing.T) {
	block := renderOLSManagedListenerMapBlock([]olsManagedSite{
		{
			Site: Website{Domain: "aurapanel.info"},
			Aliases: []string{
				"aurapanel.info",
				"www.aurapanel.info",
			},
		},
	})

	if !strings.Contains(block, "map                      AuraPanel_aurapanel_info aurapanel.info, www.aurapanel.info") {
		t.Fatalf("managed site mapping missing from listener block: %s", block)
	}
	if !strings.Contains(block, "map                      Example *") {
		t.Fatalf("example fallback mapping missing from listener block: %s", block)
	}
}

func TestSiteSystemOwnerSanitizesWebsiteOwner(t *testing.T) {
	owner := siteSystemOwner(Website{Owner: " Demo Owner "})
	if owner != "demo_owner" {
		t.Fatalf("expected sanitized system owner, got %q", owner)
	}
}

func TestRenderOLSVhostConfigUsesOwnerExtProcessorAndHomeLogs(t *testing.T) {
	config := renderOLSVhostConfig(olsManagedSite{
		Site: Website{
			Domain:     "example.com",
			Owner:      "Demo Owner",
			PHPVersion: "8.3",
		},
		Config: defaultWebsiteAdvancedConfig(),
	})

	if !strings.Contains(config, "extUser                 demo_owner") {
		t.Fatalf("expected extUser to follow site owner, got:\n%s", config)
	}
	if !strings.Contains(config, "extGroup                demo_owner") {
		t.Fatalf("expected extGroup to follow site owner, got:\n%s", config)
	}
	if !strings.Contains(config, "/home/example.com/logs/example.com.access_log") {
		t.Fatalf("expected site access log path under /home/<domain>/logs, got:\n%s", config)
	}
	if !strings.Contains(config, "/home/example.com/logs/example.com.error_log") {
		t.Fatalf("expected site error log path under /home/<domain>/logs, got:\n%s", config)
	}
}

func TestReloadOpenLiteSpeedWithHooksAcceptsSuccessfulTransitionAfterReloadError(t *testing.T) {
	phase := 0
	calls := []string{}

	err := reloadOpenLiteSpeedWithHooks(
		func(_ string, args ...string) (string, error) {
			calls = append(calls, args[0])
			if args[0] == "reload" {
				return "", errors.New("[ERROR] litespeed is not running.")
			}
			return "", nil
		},
		func() string {
			if phase == 0 {
				return "100"
			}
			return "200"
		},
		func() bool {
			return phase > 0
		},
		func(time.Duration) {
			phase++
		},
	)
	if err != nil {
		t.Fatalf("expected transition-based reload recovery, got %v", err)
	}
	if len(calls) != 1 || calls[0] != "reload" {
		t.Fatalf("expected only reload command, got %v", calls)
	}
}

func TestReloadOpenLiteSpeedWithHooksFallsBackToRestart(t *testing.T) {
	calls := []string{}

	err := reloadOpenLiteSpeedWithHooks(
		func(_ string, args ...string) (string, error) {
			calls = append(calls, args[0])
			if args[0] == "reload" {
				return "", errors.New("[ERROR] litespeed is not running.")
			}
			return "", nil
		},
		func() string {
			return "100"
		},
		func() bool {
			return false
		},
		func(time.Duration) {},
	)
	if err != nil {
		t.Fatalf("expected restart fallback to succeed, got %v", err)
	}
	if got := strings.Join(calls, ","); got != "reload,restart" {
		t.Fatalf("expected reload then restart, got %s", got)
	}
}

func TestReloadOpenLiteSpeedWithHooksReturnsCombinedErrorWhenRecoveryFails(t *testing.T) {
	err := reloadOpenLiteSpeedWithHooks(
		func(_ string, args ...string) (string, error) {
			if args[0] == "reload" {
				return "", errors.New("[ERROR] litespeed is not running.")
			}
			return "", errors.New("[ERROR] restart failed.")
		},
		func() string {
			return "100"
		},
		func() bool {
			return false
		},
		func(time.Duration) {},
	)
	if err == nil {
		t.Fatalf("expected reload failure")
	}
	message := err.Error()
	if !strings.Contains(message, "openlitespeed reload failed") {
		t.Fatalf("expected reload failure prefix, got %q", message)
	}
	if !strings.Contains(message, "restart failed") {
		t.Fatalf("expected restart failure details, got %q", message)
	}
}

func TestWriteOLSHTAccessFilePreservesExistingWhenOverwriteDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".htaccess")
	original := "RewriteEngine On\nRewriteRule ^ index.php [L]\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("seed .htaccess: %v", err)
	}

	if err := writeOLSHTAccessFile(path, "RewriteEngine On", false); err != nil {
		t.Fatalf("writeOLSHTAccessFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .htaccess: %v", err)
	}
	if string(got) != original {
		t.Fatalf("expected file to stay unchanged, got %q", string(got))
	}
}

func TestWriteOLSHTAccessFileOverwritesWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".htaccess")
	if err := os.WriteFile(path, []byte("old rules\n"), 0o644); err != nil {
		t.Fatalf("seed .htaccess: %v", err)
	}

	if err := writeOLSHTAccessFile(path, "RewriteEngine On\nRewriteRule ^ index.php [L]", true); err != nil {
		t.Fatalf("writeOLSHTAccessFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .htaccess: %v", err)
	}
	want := "RewriteEngine On\nRewriteRule ^ index.php [L]\n"
	if string(got) != want {
		t.Fatalf("expected %q, got %q", want, string(got))
	}
}

func TestWriteOLSHTAccessFileDefaultsWhenRulesEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".htaccess")

	if err := writeOLSHTAccessFile(path, "   ", true); err != nil {
		t.Fatalf("writeOLSHTAccessFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .htaccess: %v", err)
	}
	if string(got) != "RewriteEngine On\n" {
		t.Fatalf("expected default rewrite content, got %q", string(got))
	}
}

func TestSeedOLSManagedDocrootContentSeedsOnlyEmptyDocroot(t *testing.T) {
	docroot := t.TempDir()

	if err := seedOLSManagedDocrootContent(docroot, "example.com", "RewriteEngine On"); err != nil {
		t.Fatalf("seedOLSManagedDocrootContent: %v", err)
	}

	htaccessPath := filepath.Join(docroot, ".htaccess")
	htaccessRaw, err := os.ReadFile(htaccessPath)
	if err != nil {
		t.Fatalf("read .htaccess: %v", err)
	}
	if string(htaccessRaw) != "RewriteEngine On\n" {
		t.Fatalf("unexpected .htaccess content: %q", string(htaccessRaw))
	}

	indexPath := filepath.Join(docroot, "index.html")
	indexRaw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if !strings.Contains(string(indexRaw), "example.com") {
		t.Fatalf("expected placeholder domain in index.html, got %q", string(indexRaw))
	}
}

func TestSeedOLSManagedDocrootContentSkipsNonEmptyDocroot(t *testing.T) {
	docroot := t.TempDir()
	if err := os.WriteFile(filepath.Join(docroot, "index.php"), []byte("<?php echo 'ok';"), 0o644); err != nil {
		t.Fatalf("seed index.php: %v", err)
	}
	originalHTAccess := "RewriteEngine On\nRewriteRule ^ index.php [L]\n"
	if err := os.WriteFile(filepath.Join(docroot, ".htaccess"), []byte(originalHTAccess), 0o644); err != nil {
		t.Fatalf("seed .htaccess: %v", err)
	}

	if err := seedOLSManagedDocrootContent(docroot, "example.com", "RewriteEngine On\nRewriteRule ^ app.php [L]"); err != nil {
		t.Fatalf("seedOLSManagedDocrootContent: %v", err)
	}

	if fileExists(filepath.Join(docroot, "index.html")) {
		t.Fatalf("index.html should not be created for non-empty docroot")
	}
	htaccessRaw, err := os.ReadFile(filepath.Join(docroot, ".htaccess"))
	if err != nil {
		t.Fatalf("read .htaccess: %v", err)
	}
	if string(htaccessRaw) != originalHTAccess {
		t.Fatalf("existing .htaccess should stay untouched, got %q", string(htaccessRaw))
	}
}

func TestShouldOverwriteOLSHTAccess(t *testing.T) {
	if shouldOverwriteOLSHTAccess("") {
		t.Fatalf("empty rules should not overwrite existing .htaccess")
	}
	if shouldOverwriteOLSHTAccess("RewriteEngine On") {
		t.Fatalf("default rewrite bootstrap should not overwrite existing .htaccess")
	}
	if !shouldOverwriteOLSHTAccess("RewriteEngine On\nRewriteRule ^ index.php [L]") {
		t.Fatalf("custom rewrite rules should overwrite existing .htaccess")
	}
}

func TestOLSManagedMarkersHealthy(t *testing.T) {
	content := `listener Default{
    map                      Example *
    # AURAPANEL MAPS BEGIN
    map                      AuraPanel_demo demo.example
    # AURAPANEL MAPS END
}
listener AuraPanelSSL{
    map                      Example *
    # AURAPANEL MAPS BEGIN
    map                      AuraPanel_demo demo.example
    # AURAPANEL MAPS END
}
# AURAPANEL VHOSTS BEGIN
virtualHost AuraPanel_demo{
    vhRoot                   /home/demo.example/
}
# AURAPANEL VHOSTS END
module cache {
}`

	if !olsManagedMarkersHealthy(content) {
		t.Fatalf("expected markers to be healthy")
	}
}

func TestOLSManagedMarkersHealthyDetectsDrift(t *testing.T) {
	content := `listener Default{
    map                      Example *
    # AURAPANEL MAPS BEGIN
    map                      AuraPanel_demo demo.example
}
# AURAPANEL VHOSTS BEGIN
virtualHost AuraPanel_demo{
    vhRoot                   /home/demo.example/
}
# AURAPANEL VHOSTS END
module cache {
}`

	if olsManagedMarkersHealthy(content) {
		t.Fatalf("expected marker drift to be detected")
	}
}
