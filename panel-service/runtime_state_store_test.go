package main

import "testing"

func TestRuntimeStateBackendDefaultsToFileWhenExplicitPathProvided(t *testing.T) {
	t.Setenv("AURAPANEL_STATE_BACKEND", "")
	t.Setenv("AURAPANEL_STATE_FILE", "/tmp/custom-state.json")
	if got := runtimeStateBackend(); got != "file" {
		t.Fatalf("expected file backend when explicit state file is set, got %q", got)
	}
}

func TestRuntimeStateBackendDefaultsToAutoWithoutOverrides(t *testing.T) {
	t.Setenv("AURAPANEL_STATE_BACKEND", "")
	t.Setenv("AURAPANEL_STATE_FILE", "")
	if got := runtimeStateBackend(); got != "auto" {
		t.Fatalf("expected auto backend, got %q", got)
	}
}

func TestRuntimeStateBackendRespectsMariaDBOverride(t *testing.T) {
	t.Setenv("AURAPANEL_STATE_BACKEND", "mariadb")
	t.Setenv("AURAPANEL_STATE_FILE", "/tmp/custom-state.json")
	if got := runtimeStateBackend(); got != "mariadb" {
		t.Fatalf("expected mariadb backend override, got %q", got)
	}
}
