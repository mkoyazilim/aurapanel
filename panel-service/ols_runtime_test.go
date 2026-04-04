package main

import (
	"errors"
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
