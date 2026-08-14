package store

import (
	"context"
	"database/sql"
	"fmt"
)

// SetSetting, system_settings anahtarını yazar (JSON değer metni).
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO system_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value,
		updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')`, key, value); err != nil {
		return fmt.Errorf("setting set: %w", err)
	}
	return nil
}

// GetSetting, system_settings anahtarını döndürür (yoksa ok=false).
func (s *Store) GetSetting(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM system_settings WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("setting get: %w", err)
	}
	return v, true, nil
}
