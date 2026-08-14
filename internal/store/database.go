package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Database, databases kaydı.
type Database struct {
	ID        int64  `json:"id"`
	SiteID    string `json:"site_id"`
	Name      string `json:"name"`
	Charset   string `json:"charset"`
	CreatedAt string `json:"created_at"`
}

// InsertDatabase, yeni DB kaydı ekler.
func (s *Store) InsertDatabase(ctx context.Context, d Database) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO databases (site_id, name, charset) VALUES (?, ?, ?)`,
		d.SiteID, d.Name, d.Charset)
	if err != nil {
		return 0, fmt.Errorf("db insert: %w", err)
	}
	return res.LastInsertId()
}

// GetDatabaseByName, adına göre kaydı döndürür (yoksa nil).
func (s *Store) GetDatabaseByName(ctx context.Context, name string) (*Database, error) {
	var d Database
	err := s.db.QueryRowContext(ctx,
		`SELECT id, site_id, name, charset, created_at FROM databases WHERE name = ?`, name).
		Scan(&d.ID, &d.SiteID, &d.Name, &d.Charset, &d.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db get: %w", err)
	}
	return &d, nil
}

// GetDatabaseByID, ID'ye göre kaydı döndürür (yoksa nil).
func (s *Store) GetDatabaseByID(ctx context.Context, id int64) (*Database, error) {
	var d Database
	err := s.db.QueryRowContext(ctx,
		`SELECT id, site_id, name, charset, created_at FROM databases WHERE id = ?`, id).
		Scan(&d.ID, &d.SiteID, &d.Name, &d.Charset, &d.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db get by id: %w", err)
	}
	return &d, nil
}

// ListDatabasesBySite, sitenin DB'lerini döndürür.
func (s *Store) ListDatabasesBySite(ctx context.Context, siteID string) ([]Database, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, site_id, name, charset, created_at FROM databases WHERE site_id = ? ORDER BY id`, siteID)
	if err != nil {
		return nil, fmt.Errorf("db list: %w", err)
	}
	defer rows.Close()

	out := []Database{}
	for rows.Next() {
		var d Database
		if err := rows.Scan(&d.ID, &d.SiteID, &d.Name, &d.Charset, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DeleteDatabase, DB kaydını siler.
func (s *Store) DeleteDatabase(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM databases WHERE id = ?`, id); err != nil {
		return fmt.Errorf("db delete: %w", err)
	}
	return nil
}

// DatabaseUser, database_users kaydı (password_enc = encrypted-at-rest).
type DatabaseUser struct {
	ID          int64  `json:"id"`
	SiteID      string `json:"site_id"`
	Username    string `json:"username"`
	PasswordEnc string `json:"-"`
	Host        string `json:"host"`
	CreatedAt   string `json:"created_at"`
}

// InsertDatabaseUser, yeni DB kullanıcısı ekler.
func (s *Store) InsertDatabaseUser(ctx context.Context, u DatabaseUser) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO database_users (site_id, username, password_enc, host) VALUES (?, ?, ?, ?)`,
		u.SiteID, u.Username, u.PasswordEnc, u.Host)
	if err != nil {
		return 0, fmt.Errorf("db user insert: %w", err)
	}
	return res.LastInsertId()
}

// GetDatabaseUserByName, kullanıcı adına göre kaydı döndürür (yoksa nil).
func (s *Store) GetDatabaseUserByName(ctx context.Context, username string) (*DatabaseUser, error) {
	var u DatabaseUser
	err := s.db.QueryRowContext(ctx,
		`SELECT id, site_id, username, password_enc, host, created_at FROM database_users WHERE username = ?`, username).
		Scan(&u.ID, &u.SiteID, &u.Username, &u.PasswordEnc, &u.Host, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db user get: %w", err)
	}
	return &u, nil
}

// ListDatabaseUsersBySite, sitenin DB kullanıcılarını döndürür.
func (s *Store) ListDatabaseUsersBySite(ctx context.Context, siteID string) ([]DatabaseUser, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, site_id, username, password_enc, host, created_at FROM database_users WHERE site_id = ? ORDER BY id`, siteID)
	if err != nil {
		return nil, fmt.Errorf("db user list: %w", err)
	}
	defer rows.Close()

	out := []DatabaseUser{}
	for rows.Next() {
		var u DatabaseUser
		if err := rows.Scan(&u.ID, &u.SiteID, &u.Username, &u.PasswordEnc, &u.Host, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateDatabaseUserPassword, şifrelenmiş parolayı günceller.
func (s *Store) UpdateDatabaseUserPassword(ctx context.Context, id int64, passwordEnc string) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE database_users SET password_enc = ? WHERE id = ?`, passwordEnc, id); err != nil {
		return fmt.Errorf("db user password: %w", err)
	}
	return nil
}

// DeleteDatabaseUser, kullanıcı kaydını siler.
func (s *Store) DeleteDatabaseUser(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM database_users WHERE id = ?`, id); err != nil {
		return fmt.Errorf("db user delete: %w", err)
	}
	return nil
}

// AdminerGate, adminer_gates kaydı.
type AdminerGate struct {
	ID         int64
	SiteID     string
	DatabaseID sql.NullInt64
	TokenHash  string
	ExpiresAt  string
	CreatedAt  string
}

// InsertAdminerGate, gate kaydı ekler (token_hash).
func (s *Store) InsertAdminerGate(ctx context.Context, g AdminerGate) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO adminer_gates (site_id, database_id, token_hash, expires_at) VALUES (?, ?, ?, ?)`,
		g.SiteID, g.DatabaseID, g.TokenHash, g.ExpiresAt)
	if err != nil {
		return 0, fmt.Errorf("gate insert: %w", err)
	}
	return res.LastInsertId()
}

// GetAdminerGateByHash, token hash'ine göre kaydı döndürür (yoksa nil).
func (s *Store) GetAdminerGateByHash(ctx context.Context, tokenHash string) (*AdminerGate, error) {
	var g AdminerGate
	err := s.db.QueryRowContext(ctx,
		`SELECT id, site_id, database_id, token_hash, expires_at, created_at FROM adminer_gates WHERE token_hash = ?`, tokenHash).
		Scan(&g.ID, &g.SiteID, &g.DatabaseID, &g.TokenHash, &g.ExpiresAt, &g.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gate get: %w", err)
	}
	return &g, nil
}

// DeleteAdminerGate, gate kaydını siler.
func (s *Store) DeleteAdminerGate(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM adminer_gates WHERE id = ?`, id); err != nil {
		return fmt.Errorf("gate delete: %w", err)
	}
	return nil
}

// DeleteExpiredAdminerGates, süresi dolmuş gate'leri siler (temizlik).
func (s *Store) DeleteExpiredAdminerGates(ctx context.Context, nowISO string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM adminer_gates WHERE expires_at < ?`, nowISO)
	if err != nil {
		return 0, fmt.Errorf("gate cleanup: %w", err)
	}
	return res.RowsAffected()
}
