package store

import (
	"context"
	"fmt"
)

// CronJob, cron_jobs tablosu kaydı.
type CronJob struct {
	ID         int64   `json:"id"`
	SiteID     string  `json:"site_id"`
	Schedule   string  `json:"schedule"`
	Command    string  `json:"command"`
	Label      string  `json:"label"`
	Enabled    bool    `json:"enabled"`
	CreatedAt  string  `json:"created_at"`
	LastRunAt  *string `json:"last_run_at,omitempty"`
	LastStatus *string `json:"last_status,omitempty"`
}

// ListCronJobs, bir sitenin cron job'larını döndürür.
func (s *Store) ListCronJobs(ctx context.Context, siteID string) ([]CronJob, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, site_id, schedule, command, label, enabled, created_at, last_run_at, last_status
		 FROM cron_jobs WHERE site_id = ? ORDER BY id`, siteID)
	if err != nil {
		return nil, fmt.Errorf("cron list: %w", err)
	}
	defer rows.Close()
	out := make([]CronJob, 0)
	for rows.Next() {
		var j CronJob
		var enabled int
		if err := rows.Scan(&j.ID, &j.SiteID, &j.Schedule, &j.Command, &j.Label,
			&enabled, &j.CreatedAt, &j.LastRunAt, &j.LastStatus); err != nil {
			return nil, err
		}
		j.Enabled = enabled == 1
		out = append(out, j)
	}
	return out, rows.Err()
}

// InsertCronJob, yeni cron job ekler.
func (s *Store) InsertCronJob(ctx context.Context, j CronJob) (int64, error) {
	enabled := 0
	if j.Enabled {
		enabled = 1
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO cron_jobs (site_id, schedule, command, label, enabled) VALUES (?, ?, ?, ?, ?)`,
		j.SiteID, j.Schedule, j.Command, j.Label, enabled)
	if err != nil {
		return 0, fmt.Errorf("cron insert: %w", err)
	}
	return res.LastInsertId()
}

// UpdateCronJob, mevcut cron job'ı günceller.
func (s *Store) UpdateCronJob(ctx context.Context, id int64, schedule, command, label string, enabled bool) error {
	en := 0
	if enabled {
		en = 1
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE cron_jobs SET schedule=?, command=?, label=?, enabled=? WHERE id=?`,
		schedule, command, label, en, id)
	return err
}

// DeleteCronJob, cron job siler.
func (s *Store) DeleteCronJob(ctx context.Context, siteID string, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM cron_jobs WHERE id=? AND site_id=?`, id, siteID)
	return err
}

// MarkCronRun, son çalışma bilgisini günceller.
func (s *Store) MarkCronRun(ctx context.Context, id int64, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE cron_jobs SET last_run_at = strftime('%Y-%m-%dT%H:%M:%fZ','now'), last_status=? WHERE id=?`,
		status, id)
	return err
}

// GetCronJob, tek bir job döndürür (sahiplik doğrulaması için).
func (s *Store) GetCronJob(ctx context.Context, siteID string, id int64) (*CronJob, error) {
	j := &CronJob{}
	var enabled int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, site_id, schedule, command, label, enabled, created_at, last_run_at, last_status
		 FROM cron_jobs WHERE id=? AND site_id=?`, id, siteID).
		Scan(&j.ID, &j.SiteID, &j.Schedule, &j.Command, &j.Label,
			&enabled, &j.CreatedAt, &j.LastRunAt, &j.LastStatus)
	if err != nil {
		return nil, err
	}
	j.Enabled = enabled == 1
	return j, nil
}
