package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ExtDNSProvider, harici DNS sağlayıcı credential kaydı.
type ExtDNSProvider struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Provider    string         `json:"provider"` // "cloudflare" | "route53"
	Credentials string         `json:"-"`        // şifreli blob; JSON'a yansıtılmaz
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   sql.NullString `json:"updated_at,omitempty"`
}

// ExtDNSSyncLog, senkron log kaydı.
type ExtDNSSyncLog struct {
	ID         int64  `json:"id"`
	ProviderID int64  `json:"provider_id"`
	Direction  string `json:"direction"` // "push" | "pull"
	Action     string `json:"action"`    // "sync" | "conflict"
	Detail     string `json:"detail"`
	CreatedAt  string `json:"created_at"`
}

// InsertExtDNSProvider, yeni sağlayıcı ekler; ID'yi döndürür.
func (s *Store) InsertExtDNSProvider(ctx context.Context, p ExtDNSProvider) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO ext_dns_providers (name, provider, credentials) VALUES (?, ?, ?)`,
		p.Name, p.Provider, p.Credentials)
	if err != nil {
		return 0, fmt.Errorf("insert ext_dns_provider: %w", err)
	}
	return res.LastInsertId()
}

// GetExtDNSProvider, ID ile sağlayıcı döndürür.
func (s *Store) GetExtDNSProvider(ctx context.Context, id int64) (*ExtDNSProvider, error) {
	var p ExtDNSProvider
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, provider, credentials, created_at, updated_at FROM ext_dns_providers WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.Provider, &p.Credentials, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ext_dns_provider: %w", err)
	}
	return &p, nil
}

// ListExtDNSProviders, tüm sağlayıcıları listeler (credential olmadan).
func (s *Store) ListExtDNSProviders(ctx context.Context) ([]ExtDNSProvider, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, provider, created_at, updated_at FROM ext_dns_providers ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list ext_dns_providers: %w", err)
	}
	defer rows.Close()
	var out []ExtDNSProvider
	for rows.Next() {
		var p ExtDNSProvider
		if err := rows.Scan(&p.ID, &p.Name, &p.Provider, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// UpdateExtDNSProviderCredentials, şifreli credential'ı günceller.
func (s *Store) UpdateExtDNSProviderCredentials(ctx context.Context, id int64, encCreds string) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	_, err := s.db.ExecContext(ctx,
		`UPDATE ext_dns_providers SET credentials = ?, updated_at = ? WHERE id = ?`,
		encCreds, now, id)
	return err
}

// DeleteExtDNSProvider, sağlayıcıyı siler.
func (s *Store) DeleteExtDNSProvider(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM ext_dns_providers WHERE id = ?`, id)
	return err
}

// InsertExtDNSSyncLog, senkron log kaydı ekler.
func (s *Store) InsertExtDNSSyncLog(ctx context.Context, l ExtDNSSyncLog) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO ext_dns_sync_log (provider_id, direction, action, detail) VALUES (?, ?, ?, ?)`,
		l.ProviderID, l.Direction, l.Action, l.Detail)
	return err
}

// ListExtDNSSyncLog, senkron log listesi döndürür. providerID=0 ise hepsi.
func (s *Store) ListExtDNSSyncLog(ctx context.Context, providerID int64, limit int) ([]ExtDNSSyncLog, error) {
	if limit <= 0 {
		limit = 50
	}
	var (
		rows interface {
			Next() bool
			Scan(...any) error
			Close() error
		}
		err error
	)
	if providerID == 0 {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, provider_id, direction, action, detail, created_at FROM ext_dns_sync_log ORDER BY id DESC LIMIT ?`, limit)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, provider_id, direction, action, detail, created_at FROM ext_dns_sync_log WHERE provider_id = ? ORDER BY id DESC LIMIT ?`,
			providerID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list ext_dns_sync_log: %w", err)
	}
	defer rows.Close()
	var out []ExtDNSSyncLog
	for rows.Next() {
		var l ExtDNSSyncLog
		if err := rows.Scan(&l.ID, &l.ProviderID, &l.Direction, &l.Action, &l.Detail, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}
