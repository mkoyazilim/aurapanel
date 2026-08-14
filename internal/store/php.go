package store

import (
	"context"
	"database/sql"
	"fmt"
)

// InsertPHPVersion, php_versions kaydı ekler (kurulum script'i tarafından).
func (s *Store) InsertPHPVersion(ctx context.Context, version, binaryPath string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO php_versions (version, binary_path, status) VALUES (?, ?, 'available')`,
		version, binaryPath)
	if err != nil {
		return 0, fmt.Errorf("php version insert: %w", err)
	}
	return res.LastInsertId()
}

// GetPHPVersion, ID'ye karşılık gelen PHP sürümünü döndürür.
func (s *Store) GetPHPVersion(ctx context.Context, id int64) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT version FROM php_versions WHERE id = ?`, id).Scan(&v)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("php sürümü yok: id=%d", id)
	}
	if err != nil {
		return "", fmt.Errorf("php version get: %w", err)
	}
	return v, nil
}
