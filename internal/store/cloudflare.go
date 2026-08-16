package store

import (
	"context"
	"database/sql"
)

// CloudflareAccount, panele bağlı global Cloudflare hesabıdır.
// Tek kayıt (id=1); yoksa boş struct döner.
type CloudflareAccount struct {
	Email    string `json:"email"`
	APIToken string `json:"api_token"` // encrypted-at-rest değil; master key ile şifrelenmelidir (MVP: plaintext)
}

// CloudflareSettings, siteye özel zone bağlamasıdır.
type CloudflareSettings struct {
	SiteID       string `json:"site_id"`
	APIToken     string `json:"api_token"`
	ZoneID       string `json:"zone_id"`
	ProxyEnabled bool   `json:"proxy_enabled"`
}

// ── Global Hesap ─────────────────────────────────────────────────────────────

func (s *Store) GetCloudflareAccount(ctx context.Context) (*CloudflareAccount, error) {
	var a CloudflareAccount
	err := s.db.QueryRowContext(ctx,
		"SELECT value FROM system_settings WHERE key = 'cf_email'").Scan(&a.Email)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	_ = s.db.QueryRowContext(ctx,
		"SELECT value FROM system_settings WHERE key = 'cf_api_token'").Scan(&a.APIToken)
	if a.Email == "" && a.APIToken == "" {
		return nil, nil
	}
	return &a, nil
}

func (s *Store) SaveCloudflareAccount(ctx context.Context, a *CloudflareAccount) error {
	for _, kv := range [][2]string{{"cf_email", a.Email}, {"cf_api_token", a.APIToken}} {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO system_settings (key, value) VALUES (?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			kv[0], kv[1])
		if err != nil {
			return err
		}
	}
	return nil
}

// ── Site Zone Bağlaması ───────────────────────────────────────────────────────

func (s *Store) GetCloudflareSettings(ctx context.Context, siteID string) (*CloudflareSettings, error) {
	var c CloudflareSettings
	err := s.db.QueryRowContext(ctx,
		"SELECT site_id, api_token, zone_id, proxy_enabled FROM cloudflare_settings WHERE site_id = ?",
		siteID).Scan(&c.SiteID, &c.APIToken, &c.ZoneID, &c.ProxyEnabled)
	if err == sql.ErrNoRows {
		return nil, nil
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
		 ON CONFLICT(site_id) DO UPDATE SET api_token = excluded.api_token, zone_id = excluded.zone_id, proxy_enabled = excluded.proxy_enabled`,
		c.SiteID, c.APIToken, c.ZoneID, c.ProxyEnabled)
	return err
}

func (s *Store) DeleteCloudflareSettings(ctx context.Context, siteID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM cloudflare_settings WHERE site_id = ?", siteID)
	return err
}
