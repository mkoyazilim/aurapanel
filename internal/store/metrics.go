package store

import (
	"context"
	"fmt"
)

// Metric, tek bir metrik ölçümü.
type Metric struct {
	ID         int64   `json:"id"`
	SiteID     string  `json:"site_id"`
	Ts         string  `json:"ts"`
	CPUPct     float64 `json:"cpu_pct"`
	MemMB      float64 `json:"mem_mb"`
	DiskMB     float64 `json:"disk_mb"`
	DiskInodes int64   `json:"disk_inodes"`
	PIDs       int64   `json:"pids"`
}

// InsertMetric, yeni metrik satırı ekler.
func (s *Store) InsertMetric(ctx context.Context, m Metric) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO metrics (site_id, cpu_pct, mem_mb, disk_mb, disk_inodes, pids)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		m.SiteID, m.CPUPct, m.MemMB, m.DiskMB, m.DiskInodes, m.PIDs)
	if err != nil {
		return fmt.Errorf("metric insert: %w", err)
	}
	return nil
}

// ListMetrics, son N saatin metriklerini döndürür.
func (s *Store) ListMetrics(ctx context.Context, siteID string, hours int) ([]Metric, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, site_id, ts, cpu_pct, mem_mb, disk_mb, disk_inodes, pids
		 FROM metrics
		 WHERE site_id = ? AND ts >= strftime('%Y-%m-%dT%H:%M:%fZ', 'now', ? || ' hours')
		 ORDER BY ts`, siteID, fmt.Sprintf("-%d", hours))
	if err != nil {
		return nil, fmt.Errorf("metrics list: %w", err)
	}
	defer rows.Close()
	var out []Metric
	for rows.Next() {
		var m Metric
		if err := rows.Scan(&m.ID, &m.SiteID, &m.Ts, &m.CPUPct, &m.MemMB, &m.DiskMB, &m.DiskInodes, &m.PIDs); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// PruneMetrics, 25 saatten eski metrik satırlarını siler.
func (s *Store) PruneMetrics(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM metrics WHERE ts < strftime('%Y-%m-%dT%H:%M:%fZ', 'now', '-25 hours')`)
	return err
}

// LatestMetric, bir sitenin en son metriğini döndürür.
func (s *Store) LatestMetric(ctx context.Context, siteID string) (*Metric, error) {
	m := &Metric{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, site_id, ts, cpu_pct, mem_mb, disk_mb, disk_inodes, pids
		 FROM metrics WHERE site_id=? ORDER BY ts DESC LIMIT 1`, siteID).
		Scan(&m.ID, &m.SiteID, &m.Ts, &m.CPUPct, &m.MemMB, &m.DiskMB, &m.DiskInodes, &m.PIDs)
	if err != nil {
		return nil, err
	}
	return m, nil
}
