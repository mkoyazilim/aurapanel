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

// GetPHPVersionID, sürüm dizgisine karşılık gelen ID'yi döndürür.
func (s *Store) GetPHPVersionID(ctx context.Context, version string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM php_versions WHERE version = ?`, version).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("php sürümü kayıtlı değil: %s", version)
	}
	if err != nil {
		return 0, fmt.Errorf("php version id: %w", err)
	}
	return id, nil
}

// PHPVersion, php_versions kaydı.
type PHPVersion struct {
	ID         int64
	Version    string
	BinaryPath string
	Status     string
}

// ListPHPVersions, kayıtlı tüm PHP sürümlerini döndürür.
func (s *Store) ListPHPVersions(ctx context.Context) ([]PHPVersion, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, version, binary_path, status FROM php_versions ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("php version list: %w", err)
	}
	defer rows.Close()

	out := []PHPVersion{}
	for rows.Next() {
		var v PHPVersion
		if err := rows.Scan(&v.ID, &v.Version, &v.BinaryPath, &v.Status); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// SetSitePHPVersion, sitenin PHP sürümünü günceller (desired state).
func (s *Store) SetSitePHPVersion(ctx context.Context, siteID string, phpVersionID int64) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE sites SET php_version_id = ?,
		updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`,
		phpVersionID, siteID); err != nil {
		return fmt.Errorf("site php version: %w", err)
	}
	return nil
}

// UpsertPHPool, sitenin PHP pool kaydını oluşturur/günceller.
func (s *Store) UpsertPHPool(ctx context.Context, siteID string, phpVersionID int64, settingsJSON string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM php_pools WHERE site_id = ?`, siteID).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE php_pools SET php_version_id = ?, settings = ?,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE site_id = ?`,
			phpVersionID, settingsJSON, siteID); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `INSERT INTO php_pools
			(site_id, php_version_id, uid, gid, cgroup, settings)
			VALUES (?, ?, 'www-'||?, 'www-'||?, 'sites/'||?, ?)`,
			siteID, phpVersionID, siteID, siteID, siteID, settingsJSON); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetPHPool, sitenin pool kaydını döndürür (yoksa ok=false).
func (s *Store) GetPHPool(ctx context.Context, siteID string) (phpVersionID int64, settings string, ok bool, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT php_version_id, settings FROM php_pools WHERE site_id = ?`, siteID).
		Scan(&phpVersionID, &settings)
	if err == sql.ErrNoRows {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, fmt.Errorf("php pool get: %w", err)
	}
	return phpVersionID, settings, true, nil
}

// UpdatePHPoolSettings, pool settings JSON'unu günceller.
func (s *Store) UpdatePHPoolSettings(ctx context.Context, siteID, settingsJSON string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE php_pools SET settings = ?,
		updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE site_id = ?`,
		settingsJSON, siteID); err != nil {
		return fmt.Errorf("php pool settings: %w", err)
	}
	return nil
}
