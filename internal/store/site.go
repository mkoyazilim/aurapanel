package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Site, sites tablosundaki tek kayıt (şema v1).
// FeatureFlags ve Limits JSON metin tutulur; panel tarafı yapılandırır.
type Site struct {
	ID                string        `json:"id"`
	Name              string        `json:"name"`
	LinuxUser         string        `json:"linux_user"`
	HomeDir           string        `json:"home_dir"`
	Status            string        `json:"status"`
	FeatureFlags      string        `json:"feature_flags"`
	Limits            string        `json:"limits"`
	PHPVersionID      sql.NullInt64 `json:"php_version_id"`
	SecurityProfileID sql.NullInt64 `json:"security_profile_id"`
	CreatedAt         string        `json:"created_at"`
	UpdatedAt         string        `json:"updated_at"`
}

const siteColumns = `id, name, linux_user, home_dir, status, feature_flags, limits,
	php_version_id, security_profile_id, created_at, updated_at`

// GenerateSiteID, domain adına dayalı benzersiz bir site kimliği üretir ("mkoyazilim", "example" gibi).
func (s *Store) GenerateSiteID(ctx context.Context, domain string) (string, error) {
	parts := strings.Split(domain, ".")
	base := parts[0]
	
	var clean []rune
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			clean = append(clean, r)
		} else if r >= 'A' && r <= 'Z' {
			clean = append(clean, r + 32)
		}
	}
	base = string(clean)
	if base == "" {
		base = "site"
	}
	if len(base) > 15 {
		base = base[:15]
	}

	for i := 1; i <= 1000; i++ {
		candidate := base
		if i > 1 {
			candidate = fmt.Sprintf("%s%d", base, i)
		}
		var exists int
		err := s.db.QueryRowContext(ctx, `SELECT 1 FROM sites WHERE id = ?`, candidate).Scan(&exists)
		if err == sql.ErrNoRows {
			return candidate, nil
		}
		if err != nil {
			return "", fmt.Errorf("site id sorgusu: %w", err)
		}
	}
	return "", fmt.Errorf("benzersiz site id üretilemedi")
}

// InsertSite, yeni site kaydı ekler.
func (s *Store) InsertSite(ctx context.Context, st Site) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sites
		(id, name, linux_user, home_dir, status, feature_flags, limits,
		 php_version_id, security_profile_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		st.ID, st.Name, st.LinuxUser, st.HomeDir, st.Status, st.FeatureFlags, st.Limits,
		st.PHPVersionID, st.SecurityProfileID)
	if err != nil {
		return fmt.Errorf("site insert: %w", err)
	}
	return nil
}

// GetSite, site kaydını döndürür.
func (s *Store) GetSite(ctx context.Context, id string) (*Site, error) {
	var st Site
	err := s.db.QueryRowContext(ctx, `SELECT `+siteColumns+` FROM sites WHERE id = ?`, id).Scan(
		&st.ID, &st.Name, &st.LinuxUser, &st.HomeDir, &st.Status, &st.FeatureFlags, &st.Limits,
		&st.PHPVersionID, &st.SecurityProfileID, &st.CreatedAt, &st.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("site get: %w", err)
	}
	return &st, nil
}

// SetSiteStatus, site durumunu günceller (creating|active|suspended|deleting|failed).
func (s *Store) SetSiteStatus(ctx context.Context, id, status string) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE sites SET status = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`,
		status, id); err != nil {
		return fmt.Errorf("site status: %w", err)
	}
	return nil
}

// UpdateSiteLimits, site kaynak limitlerini (JSON) günceller.
func (s *Store) UpdateSiteLimits(ctx context.Context, id, limitsJSON string) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE sites SET limits = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`,
		limitsJSON, id); err != nil {
		return fmt.Errorf("site limits: %w", err)
	}
	return nil
}

// UpdateSiteFeatureFlags, site özellik bayraklarını (JSON) günceller.
func (s *Store) UpdateSiteFeatureFlags(ctx context.Context, id, flagsJSON string) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE sites SET feature_flags = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`,
		flagsJSON, id); err != nil {
		return fmt.Errorf("site feature flags: %w", err)
	}
	return nil
}

// DeleteSite, site kaydını siler (yalnızca başarılı teardown sonrası).
func (s *Store) DeleteSite(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sites WHERE id = ?`, id); err != nil {
		return fmt.Errorf("site delete: %w", err)
	}
	return nil
}

// ListSites, tüm siteleri döndürür.
func (s *Store) ListSites(ctx context.Context) ([]Site, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+siteColumns+` FROM sites ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("site list: %w", err)
	}
	defer rows.Close()

	out := []Site{}
	for rows.Next() {
		var st Site
		if err := rows.Scan(&st.ID, &st.Name, &st.LinuxUser, &st.HomeDir, &st.Status, &st.FeatureFlags,
			&st.Limits, &st.PHPVersionID, &st.SecurityProfileID, &st.CreatedAt, &st.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}
