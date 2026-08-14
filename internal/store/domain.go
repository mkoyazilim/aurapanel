package store

import (
	"context"
	"fmt"
)

// Domain, domains tablosundaki tek kayıt (main|sub|alias).
type Domain struct {
	ID          int64
	SiteID      string
	Domain      string
	Kind        string
	SSLenabled  int64
}

// InsertDomain, yeni domain kaydı ekler.
func (s *Store) InsertDomain(ctx context.Context, d Domain) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO domains (site_id, domain, kind, ssl_enabled) VALUES (?, ?, ?, ?)`,
		d.SiteID, d.Domain, d.Kind, d.SSLenabled)
	if err != nil {
		return 0, fmt.Errorf("domain insert: %w", err)
	}
	return res.LastInsertId()
}

// ListDomainsBySite, bir sitenin tüm domain kayıtlarını döndürür.
func (s *Store) ListDomainsBySite(ctx context.Context, siteID string) ([]Domain, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, site_id, domain, kind, ssl_enabled FROM domains WHERE site_id = ? ORDER BY id`, siteID)
	if err != nil {
		return nil, fmt.Errorf("domain list: %w", err)
	}
	defer rows.Close()

	out := []Domain{}
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.SiteID, &d.Domain, &d.Kind, &d.SSLenabled); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
