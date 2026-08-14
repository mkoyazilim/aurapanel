package store

import (
	"context"
)

type MailAccount struct {
	Email        string `json:"email"`
	Domain       string `json:"domain"`
	PasswordHash string `json:"-"`
	QuotaMB      int    `json:"quota_mb"`
	CreatedAt    string `json:"created_at"`
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
