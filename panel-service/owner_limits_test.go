package main

import "testing"

func TestEnforceOwnerDomainsLimitLocked_AdminBypassesLimits(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()

	// default package has Domains=3 but admin must be unlimited.
	svc.state.Websites = []Website{
		{Domain: "one.test", Owner: "admin"},
		{Domain: "two.test", Owner: "admin"},
		{Domain: "three.test", Owner: "admin"},
		{Domain: "four.test", Owner: "admin"},
	}
	if err := svc.enforceOwnerDomainsLimitLocked("admin"); err != nil {
		t.Fatalf("admin should bypass domain limits, got error: %v", err)
	}
}

func TestEnforceOwnerDomainsLimitLocked_UserStillLimited(t *testing.T) {
	svc := &service{
		startedAt: seedTime(),
		state:     seedState(),
		modules:   seedModuleState(),
	}
	svc.bootstrapModules()

	svc.state.Users = append(svc.state.Users, PanelUser{
		ID:       2,
		Username: "alice",
		Email:    "alice@example.com",
		Role:     "user",
		Package:  "default",
		Active:   true,
	})
	svc.state.Websites = []Website{
		{Domain: "one.test", Owner: "alice"},
		{Domain: "two.test", Owner: "alice"},
		{Domain: "three.test", Owner: "alice"},
	}

	if err := svc.enforceOwnerDomainsLimitLocked("alice"); err == nil {
		t.Fatalf("expected non-admin user to be limited by package domains")
	}
}

