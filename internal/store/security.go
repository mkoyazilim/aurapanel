package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SecurityProfile, bir sitenin güvenlik profili.
type SecurityProfile struct {
	SiteID  string `json:"site_id"`
	Profile string `json:"profile"` // "minimal" | "balanced" | "hardened"
}

// GetSecurityProfile, sitenin güvenlik profilini döndürür.
// Kayıt yoksa varsayılan "minimal" döner.
func (s *Store) GetSecurityProfile(ctx context.Context, siteID string) (string, error) {
	var profile string
	err := s.db.QueryRowContext(ctx,
		`SELECT profile FROM security_profiles WHERE site_id=?`, siteID).Scan(&profile)
	if errors.Is(err, sql.ErrNoRows) {
		return "minimal", nil
	}
	if err != nil {
		return "", fmt.Errorf("security profile get: %w", err)
	}
	return profile, nil
}

// SetSecurityProfile, sitenin güvenlik profilini ayarlar (upsert).
func (s *Store) SetSecurityProfile(ctx context.Context, siteID, profile string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO security_profiles (site_id, profile) VALUES (?, ?)
		 ON CONFLICT(site_id) DO UPDATE SET profile=excluded.profile`,
		siteID, profile)
	if err != nil {
		return fmt.Errorf("security profile set: %w", err)
	}
	return nil
}
