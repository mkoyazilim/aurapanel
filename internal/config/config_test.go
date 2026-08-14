package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Dosya yoksa güvenli varsayılanlarla devam edilmeli.
func TestLoadMissingFileUsesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "yok.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen.Mode != "private" {
		t.Errorf("varsayılan mod private olmalı, %q geldi", cfg.Listen.Mode)
	}
	if cfg.Listen.Address == "" || cfg.Database.Path == "" {
		t.Error("varsayılan adres/db yolu boş olamaz")
	}
}

// YAML yalnızca yazılan alanları geçersiz kılmalı; diğerleri varsayılan kalmalı.
func TestLoadOverridesOnlyStatedFields(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "aurapanel.yaml")
	content := "listen:\n  address: \"0.0.0.0:8443\"\n  mode: public\ndatabase:\n  path: /tmp/a.db\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen.Address != "0.0.0.0:8443" || cfg.Listen.Mode != "public" {
		t.Errorf("listen geçersiz kılınmadı: %+v", cfg.Listen)
	}
	if cfg.Database.Path != "/tmp/a.db" {
		t.Errorf("db path geçersiz kılınmadı: %q", cfg.Database.Path)
	}
	if cfg.Log.Format != "json" || cfg.Log.Level != "info" {
		t.Errorf("belirtilmeyen alanlar varsayılan kalmadı: %+v", cfg.Log)
	}
}

// Geçersiz mod/format değerleri reddedilmeli.
func TestValidateRejectsBadValues(t *testing.T) {
	cfg := Default()
	cfg.Listen.Mode = "herkese-acik"
	if err := cfg.Validate(); err == nil {
		t.Fatal("geçersiz listen.mode kabul edildi")
	}
	cfg = Default()
	cfg.Log.Format = "xml"
	if err := cfg.Validate(); err == nil {
		t.Fatal("geçersiz log.format kabul edildi")
	}
}
