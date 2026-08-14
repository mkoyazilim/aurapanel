package store

import (
	"context"
	"database/sql"
)

type CloudflareSettings struct {
	SiteID       string `json:"site_id"`
	APIToken     string `json:"api_token"`
	ZoneID       string `json:"zone_id"`
	ProxyEnabled bool   `json:"proxy_enabled"`
}

func (s *Store) GetCloudflareSettings(ctx context.Context, siteID string) (*CloudflareSettings, error) {
	var c CloudflareSettings
	err := s.db.QueryRowContext(ctx,
		"SELECT site_id, api_token, zone_id, proxy_enabled FROM cloudflare_settings WHERE site_id = ?",
		siteID).Scan(&c.SiteID, &c.APIToken, &c.ZoneID, &c.ProxyEnabled)
	if err == sql.ErrNoRows {
		return nil, nil // Not found is not an error, just no settings
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) SaveCloudflareSettings(ctx context.Context, c *CloudflareSettings) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cloudflare_settings (site_id, api_token, zone_id, proxy_enabled) 
		 VALUES (?, ?, ?, ?) 
		 ON DUPLICATE KEY UPDATE api_token = VALUES(api_token), zone_id = VALUES(zone_id), proxy_enabled = VALUES(proxy_enabled)`,
		c.SiteID, c.APIToken, c.ZoneID, c.ProxyEnabled)
	return err
}

func (s *Store) DeleteCloudflareSettings(ctx context.Context, siteID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM cloudflare_settings WHERE site_id = ?", siteID)
	return err
}
