package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleUsersCreatePersistsRolePolicyAssignment(t *testing.T) {
	t.Setenv("AURAPANEL_STATE_FILE", filepath.Join(t.TempDir(), "panel-service-state.json"))

	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()
	svc.modules.ACLPolicies = append(svc.modules.ACLPolicies, ACLPolicy{
		ID:          "policy-editor",
		Name:        "Editor",
		Description: "Editor policy",
		Permissions: []string{"websites.manage", "files.manage"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/create", strings.NewReader(`{
		"username":"editor1",
		"email":"editor1@example.com",
		"password":"Strong!123",
		"role":"user",
		"package":"default",
		"role_policy_id":"policy-editor"
	}`))
	rec := httptest.NewRecorder()

	svc.handleUsersCreate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	user := svc.findUserLocked("editor1")
	if user == nil {
		t.Fatalf("user not created")
	}
	if user.RolePolicyID != "policy-editor" {
		t.Fatalf("unexpected role policy id: %q", user.RolePolicyID)
	}

	if len(svc.modules.ACLAssignments) != 1 {
		t.Fatalf("expected one acl assignment, got %d", len(svc.modules.ACLAssignments))
	}
	assignment := svc.modules.ACLAssignments[0]
	if assignment.Username != "editor1" || assignment.PolicyID != "policy-editor" {
		t.Fatalf("unexpected acl assignment: %+v", assignment)
	}
}
