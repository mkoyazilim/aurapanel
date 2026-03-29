package controllers

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestLoadAdminCredentialsReadsGatewayEnvFile(t *testing.T) {
	tmp := t.TempDir()
	envPath := filepath.Join(tmp, "aurapanel.env")
	if err := os.WriteFile(envPath, []byte("AURAPANEL_ADMIN_EMAIL=ops@example.com\nAURAPANEL_ADMIN_PASSWORD=rotated-secret\n"), 0o600); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	t.Setenv("AURAPANEL_ADMIN_EMAIL", "")
	t.Setenv("AURAPANEL_ADMIN_PASSWORD", "")
	t.Setenv("AURAPANEL_ADMIN_PASSWORD_BCRYPT", "")
	t.Setenv("AURAPANEL_GATEWAY_ENV_PATH", envPath)
	t.Setenv("AURAPANEL_INITIAL_PASSWORD_FILE", filepath.Join(tmp, "missing.txt"))

	creds, err := loadAdminCredentials()
	if err != nil {
		t.Fatalf("loadAdminCredentials returned error: %v", err)
	}
	if creds.email != "ops@example.com" {
		t.Fatalf("expected email from env file, got %q", creds.email)
	}
	if creds.passwordText != "rotated-secret" {
		t.Fatalf("expected password from env file, got %q", creds.passwordText)
	}
}

func TestLoadAdminCredentialsPrefersGatewayFilePasswordOverProcessEnv(t *testing.T) {
	tmp := t.TempDir()
	envPath := filepath.Join(tmp, "aurapanel.env")
	rawHash, err := bcrypt.GenerateFromPassword([]byte("old-secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword returned error: %v", err)
	}
	fileHash := string(rawHash)
	content := "AURAPANEL_ADMIN_EMAIL=ops@example.com\n" +
		"AURAPANEL_ADMIN_PASSWORD=old-secret\n" +
		"AURAPANEL_ADMIN_PASSWORD_BCRYPT=" + fileHash + "\n"
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	t.Setenv("AURAPANEL_ADMIN_EMAIL", "")
	t.Setenv("AURAPANEL_ADMIN_PASSWORD", "new-secret")
	t.Setenv("AURAPANEL_ADMIN_PASSWORD_BCRYPT", "")
	t.Setenv("AURAPANEL_GATEWAY_ENV_PATH", envPath)
	t.Setenv("AURAPANEL_INITIAL_PASSWORD_FILE", filepath.Join(tmp, "missing.txt"))

	creds, err := loadAdminCredentials()
	if err != nil {
		t.Fatalf("loadAdminCredentials returned error: %v", err)
	}
	if creds.passwordHash == "" {
		t.Fatalf("expected gateway env file hash to be used")
	}
	if creds.passwordText != "" {
		t.Fatalf("expected plaintext password to be ignored when hash exists, got %q", creds.passwordText)
	}
	if !verifyPassword("old-secret", creds) {
		t.Fatalf("expected gateway env file hash to authenticate successfully")
	}
	if verifyPassword("new-secret", creds) {
		t.Fatalf("expected process env plaintext password to be ignored when file hash exists")
	}
}
