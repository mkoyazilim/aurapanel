package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleUsersListFiltersByResellerScope(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.state.Users = append(svc.state.Users,
		PanelUser{ID: 10, Username: "agency", Email: "agency@example.com", Role: "reseller", Active: true},
		PanelUser{ID: 11, Username: "tenant1", Email: "tenant1@example.com", Role: "user", ParentUsername: "agency", Active: true},
		PanelUser{ID: 12, Username: "outsider", Email: "outsider@example.com", Role: "user", Active: true},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/list", nil)
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "agency@example.com",
		Role:     "reseller",
		Username: "agency",
		Name:     "Agency",
	}))
	rec := httptest.NewRecorder()
	svc.handleUsersList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data []PanelUser `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	seen := map[string]struct{}{}
	for _, item := range payload.Data {
		seen[item.Username] = struct{}{}
	}
	if _, ok := seen["agency"]; !ok {
		t.Fatalf("expected reseller self account in list")
	}
	if _, ok := seen["tenant1"]; !ok {
		t.Fatalf("expected child tenant account in list")
	}
	if _, ok := seen["outsider"]; ok {
		t.Fatalf("did not expect outsider user in reseller scope")
	}
	if _, ok := seen["admin"]; ok {
		t.Fatalf("did not expect admin user in reseller scope")
	}
}

func TestHandleUsersCreateRejectsResellerForeignParent(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.state.Users = append(svc.state.Users,
		PanelUser{ID: 10, Username: "agency", Email: "agency@example.com", Role: "reseller", Active: true, PasswordHash: mustHashPassword("agency")},
		PanelUser{ID: 11, Username: "otherreseller", Email: "other@example.com", Role: "reseller", Active: true, PasswordHash: mustHashPassword("other")},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/create", strings.NewReader(`{"username":"tenantx","email":"tenantx@example.com","password":"Strong!123","role":"user","package":"default","parent_username":"otherreseller"}`))
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "agency@example.com",
		Role:     "reseller",
		Username: "agency",
		Name:     "Agency",
	}))
	rec := httptest.NewRecorder()
	svc.handleUsersCreate(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleUsersCreateDefaultsParentForReseller(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.state.Users = append(svc.state.Users,
		PanelUser{ID: 10, Username: "agency", Email: "agency@example.com", Role: "reseller", Active: true, PasswordHash: mustHashPassword("agency")},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/create", strings.NewReader(`{"username":"tenantnew","email":"tenantnew@example.com","password":"Strong!123","role":"user","package":"default"}`))
	req = req.WithContext(context.WithValue(req.Context(), servicePrincipalContextKey, servicePrincipal{
		Email:    "agency@example.com",
		Role:     "reseller",
		Username: "agency",
		Name:     "Agency",
	}))
	rec := httptest.NewRecorder()
	svc.handleUsersCreate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	created := svc.findUserLocked("tenantnew")
	if created == nil {
		t.Fatalf("expected created user")
	}
	if created.ParentUsername != "agency" {
		t.Fatalf("expected parent_username=agency, got %q", created.ParentUsername)
	}
}
