// Package config, AuraPanel yapılandırmasını yükler ve doğrular.
package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config, panelin tüm yapılandırmasını tutar.
type Config struct {
	Listen   Listen   `yaml:"listen"`
	Database Database `yaml:"database"`
	MariaDB  MariaDB  `yaml:"mariadb"`
	PowerDNS PowerDNS `yaml:"powerdns"`
	Log      Log      `yaml:"log"`
	Paths    Paths    `yaml:"paths"`
}

// PowerDNS, PowerDNS API yapılandırması.
type PowerDNS struct {
	Enabled  bool   `yaml:"enabled"`
	Endpoint string `yaml:"endpoint"`
	APIKey   string `yaml:"api_key"`
	ServerID string `yaml:"server_id"` // varsayılan "localhost"
}

// MariaDB, site DB'leri için yönetim bağlantısı (unix socket).
type MariaDB struct {
	Socket   string `yaml:"socket"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// Listen, panel API sunucusunun dinleme ayarları (ARCHITECTURE §9.4).
type Listen struct {
	// Address: "127.0.0.1:8080" veya "unix:/run/aurapanel/panel.sock"
	Address string `yaml:"address"`
	// Mode: "private" (varsayılan) veya "public".
	Mode string `yaml:"mode"`
}

// Database, SQLite metadata ayarları.
type Database struct {
	Path string `yaml:"path"`
}

// Log, slog yapılandırması.
type Log struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Paths, panelin yönettiği dizinler (ARCHITECTURE §7.1).
type Paths struct {
	DataDir   string `yaml:"data_dir"`
	SitesRoot string `yaml:"sites_root"`
	BackupDir string `yaml:"backup_dir"`
	TrashDir  string `yaml:"trash_dir"`
}

// Default, ilkelerle uyumlu güvenli varsayılanları döndürür:
// panel varsayılan olarak yalnızca loopback dinler.
func Default() *Config {
	return &Config{
		Listen:   Listen{Address: "127.0.0.1:8080", Mode: "private"},
		Database: Database{Path: "/var/lib/aurapanel/aurapanel.db"},
		MariaDB:  MariaDB{Socket: "/var/run/mysqld/mysqld.sock", User: "aurapanel"},
		Log:      Log{Level: "info", Format: "json"},
		Paths: Paths{
			DataDir:   "/var/lib/aurapanel",
			SitesRoot: "/srv/aurapanel/sites",
			BackupDir: "/srv/aurapanel/backups",
			TrashDir:  "/var/lib/aurapanel/trash",
		},
	}
}

// Load, path'teki YAML dosyasını varsayılanların üzerine uygular.
// path boşsa /etc/aurapanel/aurapanel.yaml denenir; dosya yoksa
// varsayılanlarla devam edilir (dosya varsa doğrulama zorunludur).
func Load(path string) (*Config, error) {
	cfg := Default()
	if path == "" {
		path = "/etc/aurapanel/aurapanel.yaml"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("config okunamadı %s: %w", path, err)
	}
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("config parse hatası %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate, değerlerin izin verilen kümede olduğunu denetler.
func (c *Config) Validate() error {
	switch c.Listen.Mode {
	case "private", "public":
	default:
		return fmt.Errorf("listen.mode geçersiz: %q (private|public)", c.Listen.Mode)
	}
	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("log.level geçersiz: %q", c.Log.Level)
	}
	switch c.Log.Format {
	case "json", "text":
	default:
		return fmt.Errorf("log.format geçersiz: %q", c.Log.Format)
	}
	if c.Database.Path == "" {
		return errors.New("database.path boş olamaz")
	}
	return nil
}
