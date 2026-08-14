package store

import (
	"context"
	"database/sql"
	"fmt"
)

// ResellerQuota, reseller_quotas tablosundaki tek kayıt.
type ResellerQuota struct {
	ID             int64
	ResellerID     int64
	MaxSites       int
	MaxDatabases   int
	MaxDiskGB      int
	MaxBandwidthGB int
	CreatedAt      string
	UpdatedAt      sql.NullString
}

// ResellerUsage, bir reseller'ın anlık kaynak kullanımı.
type ResellerUsage struct {
	Sites     int `json:"sites"`
	Databases int `json:"databases"`
}

// UpsertResellerQuota, reseller kotasını ekler veya günceller.
func (s *Store) UpsertResellerQuota(ctx context.Context, q ResellerQuota) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO reseller_quotas
			(reseller_id, max_sites, max_databases, max_disk_gb, max_bandwidth_gb, updated_at)
		VALUES (?, ?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ','now'))`,
		q.ResellerID, q.MaxSites, q.MaxDatabases, q.MaxDiskGB, q.MaxBandwidthGB,
	)
	if err != nil {
		return fmt.Errorf("reseller quota upsert: %w", err)
	}
	return nil
}

// GetResellerQuota, reseller kotasını döndürür; kayıt yoksa nil, nil döner.
func (s *Store) GetResellerQuota(ctx context.Context, resellerID int64) (*ResellerQuota, error) {
	var q ResellerQuota
	err := s.db.QueryRowContext(ctx, `
		SELECT id, reseller_id, max_sites, max_databases, max_disk_gb, max_bandwidth_gb,
		       created_at, updated_at
		FROM reseller_quotas
		WHERE reseller_id = ?`, resellerID,
	).Scan(
		&q.ID, &q.ResellerID, &q.MaxSites, &q.MaxDatabases, &q.MaxDiskGB, &q.MaxBandwidthGB,
		&q.CreatedAt, &q.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reseller quota get: %w", err)
	}
	return &q, nil
}

// DeleteResellerQuota, reseller kotasını siler.
func (s *Store) DeleteResellerQuota(ctx context.Context, resellerID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM reseller_quotas WHERE reseller_id = ?`, resellerID)
	if err != nil {
		return fmt.Errorf("reseller quota delete: %w", err)
	}
	return nil
}

// ListResellers, rol adı 'reseller' olan tüm kullanıcıları döndürür.
func (s *Store) ListResellers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+userColumns+` FROM users
		 WHERE role_id = (SELECT id FROM roles WHERE name = 'reseller')
		 ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("reseller list: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		var must int
		if err := rows.Scan(
			&u.ID, &u.Username, &u.PasswordHash, &u.RoleID, &u.TOTPSecretEnc,
			&must, &u.Status, &u.LastLoginAt, &u.ParentID, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("reseller scan: %w", err)
		}
		u.MustChangePassword = must == 1
		u.TOTPEnabled = u.TOTPSecretEnc.Valid && u.TOTPSecretEnc.String != ""
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reseller rows: %w", err)
	}
	return out, nil
}

// GetResellerUsage, reseller'a ait site sayısını döndürür.
// MVP: databases alanı sites ile aynı tutulur (databases tablosu henüz yok).
func (s *Store) GetResellerUsage(ctx context.Context, resellerID int64) (ResellerUsage, error) {
	var sites int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sites WHERE user_id = ?`, resellerID,
	).Scan(&sites)
	if err != nil {
		return ResellerUsage{}, fmt.Errorf("reseller usage: %w", err)
	}
	return ResellerUsage{Sites: sites, Databases: sites}, nil
}
