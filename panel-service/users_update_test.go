package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleUsersUpdateUpdatesFields(t *testing.T) {
	t.Setenv("AURAPANEL_STATE_FILE", filepath.Join(t.TempDir(), "panel-service-state.json"))

	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.state.Users = append(svc.state.Users, PanelUser{
		ID:           2,
		Username:     "alice",
		Name:         "Alice",
		Email:        "alice@example.com",
		Role:         "user",
		Package:      "default",
		Active:       true,
		PasswordHash: mustHashPassword("alicepass"),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/update", strings.NewReader(`{"username":"alice","name":"Alice Updated","email":"alice.updated@example.com","role":"reseller","package":"reseller-starter","active":false}`))
	rec := httptest.NewRecorder()

	svc.handleUsersUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	updated := svc.findUserLocked("alice")
	if updated == nil {
		t.Fatalf("expected updated user")
	}
	if updated.Name != "Alice Updated" {
		t.Fatalf("unexpected name: %s", updated.Name)
	}
	if updated.Email != "alice.updated@example.com" {
		t.Fatalf("unexpected email: %s", updated.Email)
	}
	if updated.Role != "reseller" {
		t.Fatalf("unexpected role: %s", updated.Role)
	}
	if updated.Package != "reseller-starter" {
		t.Fatalf("unexpected package: %s", updated.Package)
	}
	if updated.Active {
		t.Fatalf("expected user to be inactive")
	}
}

func TestHandleUsersUpdateRejectsRemovingLastActiveAdmin(t *testing.T) {
	t.Setenv("AURAPANEL_STATE_FILE", filepath.Join(t.TempDir(), "panel-service-state.json"))

	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/update", strings.NewReader(`{"username":"admin","role":"user","active":false}`))
	rec := httptest.NewRecorder()

	svc.handleUsersUpdate(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	admin := svc.findUserLocked("admin")
	if admin == nil {
		t.Fatalf("admin user missing")
	}
	if admin.Role != "admin" || !admin.Active {
		t.Fatalf("admin user should remain active admin")
	}
}

func TestHandleUsersUpdateSetsParentUsername(t *testing.T) {
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
			ID:           3,
			Username:     "alice",
			Name:         "Alice",
			Email:        "alice@example.com",
			Role:         "user",
			Package:      "default",
			Active:       true,
			PasswordHash: mustHashPassword("alicepass"),
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/update", strings.NewReader(`{"username":"alice","parent_username":"agency"}`))
	rec := httptest.NewRecorder()

	svc.handleUsersUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	updated := svc.findUserLocked("alice")
	if updated == nil {
		t.Fatalf("expected updated user")
	}
	if updated.ParentUsername != "agency" {
		t.Fatalf("expected parent_username=agency, got %q", updated.ParentUsername)
	}
}
