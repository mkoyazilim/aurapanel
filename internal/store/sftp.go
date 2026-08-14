package store

import (
	"context"
	"database/sql"
	"fmt"
)

// SFTPAccount, sftp_accounts kaydı.
type SFTPAccount struct {
	ID        int64
	SiteID    string
	Username  string
	JailPath  string
	Status    string
	CreatedAt string
}

// InsertSFTPAccount, yeni hesap ekler.
func (s *Store) InsertSFTPAccount(ctx context.Context, a SFTPAccount) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO sftp_accounts (site_id, username, jail_path, status) VALUES (?, ?, ?, ?)`,
		a.SiteID, a.Username, a.JailPath, a.Status)
	if err != nil {
		return 0, fmt.Errorf("sftp insert: %w", err)
	}
	return res.LastInsertId()
}

// GetSFTPAccountByName, hesap adına göre kaydı döndürür (yoksa nil).
func (s *Store) GetSFTPAccountByName(ctx context.Context, username string) (*SFTPAccount, error) {
	var a SFTPAccount
	err := s.db.QueryRowContext(ctx,
		`SELECT id, site_id, username, jail_path, status, created_at FROM sftp_accounts WHERE username = ?`, username).
		Scan(&a.ID, &a.SiteID, &a.Username, &a.JailPath, &a.Status, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sftp get: %w", err)
	}
	return &a, nil
}

// ListSFTPAccountsBySite, sitenin hesaplarını döndürür.
func (s *Store) ListSFTPAccountsBySite(ctx context.Context, siteID string) ([]SFTPAccount, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, site_id, username, jail_path, status, created_at FROM sftp_accounts WHERE site_id = ? ORDER BY id`, siteID)
	if err != nil {
		return nil, fmt.Errorf("sftp list: %w", err)
	}
	defer rows.Close()

	out := []SFTPAccount{}
	for rows.Next() {
		var a SFTPAccount
		if err := rows.Scan(&a.ID, &a.SiteID, &a.Username, &a.JailPath, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListAllSFTPAccounts, tüm hesapları döndürür (config üretimi için).
func (s *Store) ListAllSFTPAccounts(ctx context.Context) ([]SFTPAccount, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, site_id, username, jail_path, status, created_at FROM sftp_accounts ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("sftp list all: %w", err)
	}
	defer rows.Close()

	out := []SFTPAccount{}
	for rows.Next() {
		var a SFTPAccount
		if err := rows.Scan(&a.ID, &a.SiteID, &a.Username, &a.JailPath, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DeleteSFTPAccount, hesabı siler.
func (s *Store) DeleteSFTPAccount(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sftp_accounts WHERE id = ?`, id); err != nil {
		return fmt.Errorf("sftp delete: %w", err)
	}
	return nil
}
