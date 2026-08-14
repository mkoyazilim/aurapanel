package store

import (
	"context"
	"fmt"
)

// Backup, backups kaydı.
type Backup struct {
	ID         int64   `json:"id"`
	SiteID     string  `json:"site_id"`
	Kind       string  `json:"kind"`
	Storage    string  `json:"storage"`
	Location   string  `json:"location"`
	Encrypted  int64   `json:"encrypted"`
	SizeBytes  int64   `json:"size_bytes"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"created_at"`
	FinishedAt *string `json:"finished_at,omitempty"`
}

// InsertBackup, yedek kaydı ekler.
func (s *Store) InsertBackup(ctx context.Context, b Backup) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO backups (site_id, kind, storage, location, encrypted, size_bytes, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		b.SiteID, b.Kind, b.Storage, b.Location, b.Encrypted, b.SizeBytes, b.Status)
	if err != nil {
		return 0, fmt.Errorf("backup insert: %w", err)
	}
	return res.LastInsertId()
}

// MarkBackupDone, yedeği başarılı işaretler.
func (s *Store) MarkBackupDone(ctx context.Context, id, sizeBytes int64) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE backups SET status = 'success',
		size_bytes = ?, finished_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`,
		sizeBytes, id); err != nil {
		return fmt.Errorf("backup done: %w", err)
	}
	return nil
}

// MarkBackupFailed, yedeği başarısız işaretler.
func (s *Store) MarkBackupFailed(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE backups SET status = 'failed',
		finished_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`, id); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	return nil
}

// ListBackupsBySite, sitenin yedeklerini döndürür (eskiden yeniye).
func (s *Store) ListBackupsBySite(ctx context.Context, siteID string) ([]Backup, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, site_id, kind, storage, location,
		encrypted, size_bytes, status, created_at, finished_at
		FROM backups WHERE site_id = ? ORDER BY id`, siteID)
	if err != nil {
		return nil, fmt.Errorf("backup list: %w", err)
	}
	defer rows.Close()

	out := []Backup{}
	for rows.Next() {
		var b Backup
		if err := rows.Scan(&b.ID, &b.SiteID, &b.Kind, &b.Storage, &b.Location,
			&b.Encrypted, &b.SizeBytes, &b.Status, &b.CreatedAt, &b.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// DeleteBackup, yedek kaydını siler.
func (s *Store) DeleteBackup(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM backups WHERE id = ?`, id); err != nil {
		return fmt.Errorf("backup delete: %w", err)
	}
	return nil
}
