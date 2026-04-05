package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRewriteRulesForDocrootAcceptsLaravelFrontController(t *testing.T) {
	docroot := t.TempDir()
	writeTestFile(t, docroot, "index.php", "<?php echo 'ok';")

	rules := "RewriteEngine On\nRewriteRule ^(.*)$ index.php [L]"
	if err := validateRewriteRulesForDocroot(docroot, rules); err != nil {
		t.Fatalf("expected valid laravel rewrite, got %v", err)
	}
}

func TestValidateRewriteRulesForDocrootRejectsCatchAllToDirWithoutIndex(t *testing.T) {
	docroot := t.TempDir()
	writeTestDir(t, docroot, "core/public")

	rules := "RewriteEngine On\nRewriteRule ^(.*)$ core/public/$1 [L]"
	err := validateRewriteRulesForDocroot(docroot, rules)
	if err == nil {
		t.Fatalf("expected validation error for directory target without index")
	}
	if !strings.Contains(err.Error(), "rewrite hedef klasorunde index yok") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRewriteRulesForDocrootRejectsMissingTarget(t *testing.T) {
	docroot := t.TempDir()
	writeTestFile(t, docroot, "index.php", "<?php echo 'ok';")

	rules := "RewriteEngine On\nRewriteRule ^(.*)$ missing/index.php [L]"
	err := validateRewriteRulesForDocroot(docroot, rules)
	if err == nil {
		t.Fatalf("expected validation error for missing target")
	}
	if !strings.Contains(err.Error(), "rewrite hedefi bulunamadi") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRewriteRulesForDocrootAllowsNonCatchAllDirRewrite(t *testing.T) {
	docroot := t.TempDir()
	writeTestDir(t, docroot, "core/public")

	rules := "RewriteEngine On\nRewriteRule ^assets/(.*)$ core/public/$1 [L]"
	if err := validateRewriteRulesForDocroot(docroot, rules); err != nil {
		t.Fatalf("expected scoped rewrite to pass, got %v", err)
	}
}

func writeTestDir(t *testing.T, root, rel string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}
