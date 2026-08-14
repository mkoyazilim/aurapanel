package store

import (
	"context"
	"database/sql"
	"fmt"
)

// SSLCert, ssl_certificates kaydı.
type SSLCert struct {
	ID            int64
	SiteID        string
	DomainID      sql.NullInt64
	Issuer        string
	CertPath      sql.NullString
	KeyPath       sql.NullString
	NotBefore     sql.NullString
	NotAfter      sql.NullString
	AutoRenew     int64
	LastRenewedAt sql.NullString
	CreatedAt     string
}

// InsertSSLCert, yeni sertifika kaydı ekler.
func (s *Store) InsertSSLCert(ctx context.Context, c SSLCert) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO ssl_certificates
		(site_id, domain_id, issuer, cert_path, key_path, not_before, not_after, auto_renew)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.SiteID, c.DomainID, c.Issuer, c.CertPath, c.KeyPath, c.NotBefore, c.NotAfter, c.AutoRenew)
	if err != nil {
		return 0, fmt.Errorf("ssl cert insert: %w", err)
	}
	return res.LastInsertId()
}

// GetSSLCertByDomain, domainID'ye ait aktif sertifika kaydını döndürür.
func (s *Store) GetSSLCertByDomain(ctx context.Context, domainID int64) (*SSLCert, error) {
	var c SSLCert
	err := s.db.QueryRowContext(ctx, `SELECT id, site_id, domain_id, issuer, cert_path, key_path,
		not_before, not_after, auto_renew, last_renewed_at, created_at
		FROM ssl_certificates WHERE domain_id = ? ORDER BY id DESC LIMIT 1`, domainID).
		Scan(&c.ID, &c.SiteID, &c.DomainID, &c.Issuer, &c.CertPath, &c.KeyPath,
			&c.NotBefore, &c.NotAfter, &c.AutoRenew, &c.LastRenewedAt, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ssl cert get: %w", err)
	}
	return &c, nil
}

// ListSSLCertsExpiringBefore, auto_renew açık ve süresi verilen zamandan
// önce dolan sertifikaları döndürür (ISO 8601, lexicographic karşılaştırma).
func (s *Store) ListSSLCertsExpiringBefore(ctx context.Context, isoTime string) ([]SSLCert, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, site_id, domain_id, issuer, cert_path, key_path,
		not_before, not_after, auto_renew, last_renewed_at, created_at
		FROM ssl_certificates WHERE auto_renew = 1 AND not_after IS NOT NULL AND not_after < ?
		ORDER BY not_after`, isoTime)
	if err != nil {
		return nil, fmt.Errorf("ssl cert expiring: %w", err)
	}
	defer rows.Close()

	out := []SSLCert{}
	for rows.Next() {
		var c SSLCert
		if err := rows.Scan(&c.ID, &c.SiteID, &c.DomainID, &c.Issuer, &c.CertPath, &c.KeyPath,
			&c.NotBefore, &c.NotAfter, &c.AutoRenew, &c.LastRenewedAt, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateSSLCertAfterRenew, yenileme sonrası zamanları günceller.
func (s *Store) UpdateSSLCertAfterRenew(ctx context.Context, id int64, notBefore, notAfter string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE ssl_certificates
		SET not_before = ?, not_after = ?, last_renewed_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
		WHERE id = ?`, notBefore, notAfter, id); err != nil {
		return fmt.Errorf("ssl cert renew update: %w", err)
	}
	return nil
}

// DeleteSSLCert, sertifika kaydını siler.
func (s *Store) DeleteSSLCert(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM ssl_certificates WHERE id = ?`, id); err != nil {
		return fmt.Errorf("ssl cert delete: %w", err)
	}
	return nil
}

// SetDomainSSL, domainin SSL durumunu günceller.
func (s *Store) SetDomainSSL(ctx context.Context, domainID int64, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE domains SET ssl_enabled = ? WHERE id = ?`, v, domainID); err != nil {
		return fmt.Errorf("domain ssl: %w", err)
	}
	return nil
}
