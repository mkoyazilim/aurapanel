package store

import (
	"context"
	"database/sql"
	"fmt"
)

// User, users kaydı.
type User struct {
	ID                 int64
	Username           string
	PasswordHash       string
	RoleID             int64
	TOTPSecretEnc      sql.NullString
	TOTPEnabled        bool
	MustChangePassword bool
	Status             string
	LastLoginAt        sql.NullString
	ParentID           sql.NullInt64
	CreatedAt          string
	UpdatedAt          sql.NullString
}

var userColumns = "id, username, password_hash, role_id, totp_secret_enc, must_change_password, status, last_login_at, parent_id, created_at, updated_at"

// InsertUser, yeni kullanıcı ekler.
func (s *Store) InsertUser(ctx context.Context, u User) (int64, error) {
	must := 0
	if u.MustChangePassword {
		must = 1
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO users
		(username, password_hash, role_id, must_change_password, status, parent_id)
		VALUES (?, ?, ?, ?, ?, ?)`,
		u.Username, u.PasswordHash, u.RoleID, must, u.Status, u.ParentID)
	if err != nil {
		return 0, fmt.Errorf("user insert: %w", err)
	}
	return res.LastInsertId()
}

// GetUserByUsername, kullanıcı adına göre kaydı döndürür (yoksa nil).
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	u, err := s.getUser(ctx, `SELECT `+userColumns+` FROM users WHERE username = ?`, username)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, nil
	}
	u.TOTPEnabled = u.TOTPSecretEnc.Valid && u.TOTPSecretEnc.String != ""
	return u, nil
}

// GetUserByID, kullanıcı ID'sine göre kaydı döndürür.
func (s *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	u, err := s.getUser(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, nil
	}
	u.TOTPEnabled = u.TOTPSecretEnc.Valid && u.TOTPSecretEnc.String != ""
	return u, nil
}

func (s *Store) getUser(ctx context.Context, query string, args ...any) (*User, error) {
	var u User
	var must int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.RoleID, &u.TOTPSecretEnc,
		&must, &u.Status, &u.LastLoginAt, &u.ParentID, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("user get: %w", err)
	}
	u.MustChangePassword = must == 1
	return &u, nil
}

// CountUsers, toplam kullanıcı sayısını döndürür (bootstrap kontrolü).
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// ListUsers, kullanıcıları listeler. parentID > 0 ise sadece o kullanıcının alt kullanıcılarını getirir.
func (s *Store) ListUsers(ctx context.Context, parentID int64) ([]User, error) {
	query := `SELECT ` + userColumns + ` FROM users`
	var args []any
	if parentID > 0 {
		query += ` WHERE parent_id = ?`
		args = append(args, parentID)
	}
	query += ` ORDER BY id`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("user list: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
			var must int
			if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.RoleID, &u.TOTPSecretEnc,
				&must, &u.Status, &u.LastLoginAt, &u.ParentID, &u.CreatedAt, &u.UpdatedAt); err != nil {
				return nil, err
			}
			u.MustChangePassword = must == 1
		out = append(out, u)
	}
	return out, nil
}

// DeleteUser, kullanıcıyı ve bağımlılıklarını (kendi sitesi yoksa) siler.
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}

// UpdateUserPassword, parola hash'ini günceller ve zorunlu değişimi kapatır.
func (s *Store) UpdateUserPassword(ctx context.Context, id int64, passwordHash string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ?,
		must_change_password = 0,
		updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`,
		passwordHash, id); err != nil {
		return fmt.Errorf("user password: %w", err)
	}
	return nil
}

// UpdateUserUsername, kullanıcının kullanıcı adını günceller.
func (s *Store) UpdateUserUsername(ctx context.Context, id int64, username string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET username = ?,
		updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`,
		username, id); err != nil {
		return fmt.Errorf("user username update: %w", err)
	}
	return nil
}

// SetUserTOTPSecret, TOTP sırrını (encrypted-at-rest) kaydeder.
func (s *Store) SetUserTOTPSecret(ctx context.Context, id int64, secretEnc string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET totp_secret_enc = ?,
		updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`,
		secretEnc, id); err != nil {
		return fmt.Errorf("user totp: %w", err)
	}
	return nil
}

// SetUserLastLogin, son giriş zamanını günceller.
func (s *Store) SetUserLastLogin(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET
		last_login_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`, id); err != nil {
		return fmt.Errorf("user last login: %w", err)
	}
	return nil
}

// Session, sessions kaydı.
type Session struct {
	ID        string
	UserID    int64
	IP        string
	UserAgent string
	CSRFToken string
	ExpiresAt string
	CreatedAt string
}

// InsertSession, yeni oturum ekler.
func (s *Store) InsertSession(ctx context.Context, se Session) error {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO sessions
		(id, user_id, ip, user_agent, csrf_token, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
		se.ID, se.UserID, se.IP, se.UserAgent, se.CSRFToken, se.ExpiresAt); err != nil {
		return fmt.Errorf("session insert: %w", err)
	}
	return nil
}

// GetSession, oturumu döndürür (yoksa nil). Süresi dolmuşsa silinir.
func (s *Store) GetSession(ctx context.Context, id string) (*Session, error) {
	var se Session
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, ip, user_agent, csrf_token, expires_at, created_at
		FROM sessions WHERE id = ?`, id).Scan(
		&se.ID, &se.UserID, &se.IP, &se.UserAgent, &se.CSRFToken, &se.ExpiresAt, &se.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("session get: %w", err)
	}
	return &se, nil
}

// DeleteSession, oturumu siler.
func (s *Store) DeleteSession(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("session delete: %w", err)
	}
	return nil
}

// DeleteExpiredSessions, süresi dolmuş oturumları temizler.
func (s *Store) DeleteExpiredSessions(ctx context.Context, nowISO string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, nowISO)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PATToken, pat_tokens kaydı.
type PATToken struct {
	ID         int64
	UserID     int64
	Name       string
	TokenHash  string
	CreatedAt  string
	ExpiresAt  sql.NullString
	LastUsedAt sql.NullString
}

// InsertPATToken, yeni PAT ekler.
func (s *Store) InsertPATToken(ctx context.Context, p PATToken) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO pat_tokens (user_id, name, token_hash, expires_at)
		VALUES (?, ?, ?, ?)`, p.UserID, p.Name, p.TokenHash, p.ExpiresAt)
	if err != nil {
		return 0, fmt.Errorf("pat insert: %w", err)
	}
	return res.LastInsertId()
}

// GetPATTokenByHash, PAT'yi hash'e göre döndürür (yoksa nil).
func (s *Store) GetPATTokenByHash(ctx context.Context, tokenHash string) (*PATToken, error) {
	var p PATToken
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, name, token_hash, created_at, expires_at, last_used_at
		FROM pat_tokens WHERE token_hash = ?`, tokenHash).Scan(
		&p.ID, &p.UserID, &p.Name, &p.TokenHash, &p.CreatedAt, &p.ExpiresAt, &p.LastUsedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("pat get: %w", err)
	}
	return &p, nil
}

// ListPATTokensByUser, kullanıcının PAT'lerini döndürür.
func (s *Store) ListPATTokensByUser(ctx context.Context, userID int64) ([]PATToken, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, name, token_hash, created_at, expires_at, last_used_at
		FROM pat_tokens WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, fmt.Errorf("pat list: %w", err)
	}
	defer rows.Close()

	out := []PATToken{}
	for rows.Next() {
		var p PATToken
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.TokenHash, &p.CreatedAt, &p.ExpiresAt, &p.LastUsedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeletePATToken, PAT'yi siler.
func (s *Store) DeletePATToken(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM pat_tokens WHERE id = ?`, id); err != nil {
		return fmt.Errorf("pat delete: %w", err)
	}
	return nil
}

// TouchPATToken, PAT'nin son kullanımını günceller.
func (s *Store) TouchPATToken(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE pat_tokens SET
		last_used_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`, id); err != nil {
		return fmt.Errorf("pat touch: %w", err)
	}
	return nil
}

// GetRoleName, rol adını döndürür.
func (s *Store) GetRoleName(ctx context.Context, roleID int64) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `SELECT name FROM roles WHERE id = ?`, roleID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("role get: %w", err)
	}
	return name, nil
}

// GetRoleIDByName, rol adına göre ID döndürür.
func (s *Store) GetRoleIDByName(ctx context.Context, name string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = ?`, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("role id: %w", err)
	}
	return id, nil
}
