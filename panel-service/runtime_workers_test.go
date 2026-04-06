package main

import "testing"

func TestFilterRuntimeOrphanTemporaryDBUsers(t *testing.T) {
	discovered := []dbToolTempUser{
		{Engine: "mariadb", Username: "apsso_alpha"},
		{Engine: "mariadb", Username: "apsso_beta"},
		{Engine: "postgresql", Username: "apsso_pg_gamma"},
	}
	tracked := map[string]dbToolTempUser{
		dbToolTempUserKey("mariadb", "apsso_alpha"): {
			Engine:   "mariadb",
			Username: "apsso_alpha",
		},
	}

	orphan := filterRuntimeOrphanTemporaryDBUsers(discovered, tracked)
	if len(orphan) != 2 {
		t.Fatalf("expected 2 orphan users, got %d", len(orphan))
	}

	found := map[string]bool{}
	for _, item := range orphan {
		found[dbToolTempUserKey(item.Engine, item.Username)] = true
	}
	if !found[dbToolTempUserKey("mariadb", "apsso_beta")] {
		t.Fatalf("expected apsso_beta to be orphan")
	}
	if !found[dbToolTempUserKey("postgresql", "apsso_pg_gamma")] {
		t.Fatalf("expected apsso_pg_gamma to be orphan")
	}
}

func TestIsRuntimeTemporaryDBUser(t *testing.T) {
	if !isRuntimeTemporaryDBUser("apsso_abc123") {
		t.Fatalf("expected apsso_* usernames to be temporary")
	}
	if !isRuntimeTemporaryDBUser("apsso_pg_x1y2z3") {
		t.Fatalf("expected apsso_pg_* usernames to be temporary")
	}
	if isRuntimeTemporaryDBUser("app_user") {
		t.Fatalf("non apsso user should not be temporary")
	}
}
