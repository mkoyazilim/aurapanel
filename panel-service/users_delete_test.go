package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleUsersDeletePersistsState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "panel-service-state.json")
	t.Setenv("AURAPANEL_STATE_FILE", statePath)

	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.state.Users = append(svc.state.Users, PanelUser{
		ID:           2,
		Username:     "demo",
		Name:         "Demo User",
		Email:        "demo@example.com",
		Role:         "user",
		Package:      "default",
		Active:       true,
		PasswordHash: mustHashPassword("demopass"),
	})
	svc.state.Websites = append(svc.state.Websites, Website{
		Domain: "demo.example.com",
		Owner:  "demo",
		User:   "demo",
		Status: "active",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/delete", strings.NewReader(`{"username":"demo"}`))
	rec := httptest.NewRecorder()

	svc.handleUsersDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if user := svc.findUserLocked("demo"); user != nil {
		t.Fatalf("demo user should be removed from runtime state")
	}
	if got := svc.state.Websites[0].Owner; got != "admin" {
		t.Fatalf("website owner should be reassigned to admin, got %q", got)
	}
	if got := svc.state.Websites[0].User; got != "admin" {
		t.Fatalf("website user should be reassigned to admin, got %q", got)
	}

	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("expected persisted state file: %v", err)
	}
	var persisted persistedRuntimeState
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("decode persisted state: %v", err)
	}
	for _, user := range persisted.State.Users {
		if user.Username == "demo" {
			t.Fatalf("demo user should not exist in persisted state")
		}
	}
}

func TestHandleUsersDeleteRejectsAdmin(t *testing.T) {
	t.Setenv("AURAPANEL_STATE_FILE", filepath.Join(t.TempDir(), "panel-service-state.json"))

	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/delete", strings.NewReader(`{"username":"admin"}`))
	rec := httptest.NewRecorder()

	svc.handleUsersDelete(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if admin := svc.findUserLocked("admin"); admin == nil {
		t.Fatalf("admin user should remain in runtime state")
	}
}

func TestHandleUsersDeleteClearsChildParentReference(t *testing.T) {
	t.Setenv("AURAPANEL_STATE_FILE", filepath.Join(t.TempDir(), "panel-service-state.json"))

	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.state.Users = append(svc.state.Users,
		PanelUser{
			ID:           2,
			Username:     "agency",
			Name:         "Agency",
			Email:        "agency@example.com",
			Role:         "reseller",
			Package:      "reseller-starter",
			Active:       true,
			PasswordHash: mustHashPassword("agencypass"),
		},
		PanelUser{
			ID:             3,
			Username:       "tenant1",
			Name:           "Tenant One",
			Email:          "tenant1@example.com",
			Role:           "user",
			Package:        "default",
			ParentUsername: "agency",
			Active:         true,
			PasswordHash:   mustHashPassword("tenantpass"),
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/delete", strings.NewReader(`{"username":"agency"}`))
	rec := httptest.NewRecorder()

	svc.handleUsersDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	child := svc.findUserLocked("tenant1")
	if child == nil {
		t.Fatalf("child user should remain")
	}
	if child.ParentUsername != "" {
		t.Fatalf("expected child parent to be cleared, got %q", child.ParentUsername)
	}
}
