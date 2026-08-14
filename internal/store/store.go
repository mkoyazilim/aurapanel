// Package store, SQLite metadata katmanını yönetir (ARCHITECTURE §4.1).
//
// Tek yazıcı kuralı: SQLite'a yalnızca AuraPanel backend'i yazar.
// WAL mode + foreign keys + busy timeout + gömülü migration sistemi.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite" // pure-Go sürücü; CGO yasak (ARCHITECTURE §2)
)

var ErrNotFound = errors.New("kayıt bulunamadı")

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store, panelin SQLite erişim katmanı.
type Store struct {
	db *sql.DB
}

// Open, veritabanını WAL/FK/busy_timeout pragma'larıyla açar.
// SQLite tek yazıcı olduğundan bağlantı havuzu 1 ile sınırlanır.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("db dizini oluşturulamadı %s: %w", dir, err)
		}
	}
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db açılamadı: %w", err)
	}
	db.SetMaxOpenConns(1)
	return &Store{db: db}, nil
}

// Close, veritabanını kapatır.
func (s *Store) Close() error { return s.db.Close() }

// Ping, bağlantıyı doğrular.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Migrate, gömülü migration'ları sürüm sırasıyla uygular.
// Her migration tek transaction içinde çalışır; tekrar çalıştırma güvenlidir.
func (s *Store) Migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		name       TEXT NOT NULL,
		applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`); err != nil {
		return fmt.Errorf("schema_migrations: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("migration dizini okunamadı: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		var version int
		if _, err := fmt.Sscanf(name, "%d_", &version); err != nil {
			return fmt.Errorf("migration adı geçersiz: %s", name)
		}
		var applied int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&applied); err != nil {
			return fmt.Errorf("migration durumu %s: %w", name, err)
		}
		if applied > 0 {
			continue
		}
		sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("migration okunamadı %s: %w", name, err)
		}
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("migration tx %s: %w", name, err)
		}
		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?, ?)`, version, name); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration kaydı %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration commit %s: %w", name, err)
		}
	}
	return nil
}
