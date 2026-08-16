package store

import (
	"context"
	"database/sql"
	"fmt"
)

type MailAccount struct {
	Email        string `json:"email"`
	Domain       string `json:"domain"`
	PasswordHash string `json:"-"`
	QuotaMB      int    `json:"quota_mb"`
	CreatedAt    string `json:"created_at"`
}

type MailDomain struct {
	Domain    string `json:"domain"`
	SiteID    string `json:"site_id"`
	CreatedAt string `json:"created_at"`
}

func (s *Store) EnsureMailDomain(ctx context.Context, domain, siteID string) error {
	_, err := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO mail_domains (domain, site_id) VALUES (?, ?)", domain, siteID)
	return err
}

func (s *Store) ListMailAccounts(ctx context.Context, siteID string) ([]MailAccount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.email, a.domain, a.quota_mb, a.created_at
		FROM mail_accounts a
		JOIN mail_domains d ON a.domain = d.domain
		WHERE d.site_id = ?
	`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []MailAccount
	for rows.Next() {
		var a MailAccount
		if err := rows.Scan(&a.Email, &a.Domain, &a.QuotaMB, &a.CreatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	if accounts == nil {
		accounts = []MailAccount{}
	}
	return accounts, nil
}

func (s *Store) CreateMailAccount(ctx context.Context, a MailAccount) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO mail_accounts (email, domain, password_hash, quota_mb) VALUES (?, ?, ?, ?)",
		a.Email, a.Domain, a.PasswordHash, a.QuotaMB)
	return err
}

func (s *Store) DeleteMailAccount(ctx context.Context, email string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM mail_accounts WHERE email = ?", email)
	return err
}

func (s *Store) UpdateMailPassword(ctx context.Context, email, passwordHash string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE mail_accounts SET password_hash = ? WHERE email = ?", passwordHash, email)
	return err
}

func (s *Store) GetMailDomains(ctx context.Context) ([]MailDomain, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT domain, site_id, created_at FROM mail_domains`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []MailDomain
	for rows.Next() {
		var d MailDomain
		if err := rows.Scan(&d.Domain, &d.SiteID, &d.CreatedAt); err != nil {
			return nil, err
		}
		domains = append(domains, d)
	}
	if domains == nil {
		domains = []MailDomain{}
	}
	return domains, nil
}

func (s *Store) GetMailAccountsByDomain(ctx context.Context, domain string) ([]MailAccount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT email, domain, password_hash, quota_mb, created_at
		FROM mail_accounts WHERE domain = ?
	`, domain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []MailAccount
	for rows.Next() {
		var a MailAccount
		if err := rows.Scan(&a.Email, &a.Domain, &a.PasswordHash, &a.QuotaMB, &a.CreatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	if accounts == nil {
		accounts = []MailAccount{}
	}
	return accounts, nil
}

func (s *Store) GetAllMailAccounts(ctx context.Context) ([]MailAccount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT email, domain, password_hash, quota_mb, created_at
		FROM mail_accounts
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []MailAccount
	for rows.Next() {
		var a MailAccount
		if err := rows.Scan(&a.Email, &a.Domain, &a.PasswordHash, &a.QuotaMB, &a.CreatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	if accounts == nil {
		accounts = []MailAccount{}
	}
	return accounts, nil
}

func (s *Store) SaveDKIMKey(ctx context.Context, domain, privateKey, publicKey string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mail_dkim (domain, private_key, public_key) VALUES (?, ?, ?)
		ON CONFLICT(domain) DO UPDATE SET private_key = excluded.private_key, public_key = excluded.public_key
	`, domain, privateKey, publicKey)
	return err
}

func (s *Store) GetDKIMKey(ctx context.Context, domain string) (priv, pub string, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT private_key, public_key FROM mail_dkim WHERE domain = ?`, domain,
	).Scan(&priv, &pub)
	if err == sql.ErrNoRows {
		return "", "", fmt.Errorf("no DKIM key for domain %q", domain)
	}
	return
}
