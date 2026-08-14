package store

import (
	"context"
	"database/sql"
	"fmt"
)

// DriftEvent, configuration drift kaydı (ARCHITECTURE §6).
type DriftEvent struct {
	ID         int64          `json:"id"`
	SiteID     string         `json:"site_id"`
	Resource   string         `json:"resource"`
	Expected   string         `json:"expected"`
	Actual     string         `json:"actual"`
	Severity   string         `json:"severity"`
	Status     string         `json:"status"`
	DetectedAt string         `json:"detected_at"`
	ResolvedAt sql.NullString `json:"resolved_at,omitempty"`
}

// InsertDriftEvent, yeni drift kaydı ekler.
func (s *Store) InsertDriftEvent(ctx context.Context, e DriftEvent) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO drift_events
		(site_id, resource, expected, actual, severity, status)
		VALUES (?, ?, ?, ?, ?, 'open')`,
		e.SiteID, e.Resource, e.Expected, e.Actual, e.Severity)
	if err != nil {
		return 0, fmt.Errorf("drift insert: %w", err)
	}
	return res.LastInsertId()
}

// ListOpenDriftEvents, çözülmemiş drift kayıtlarını döndürür.
func (s *Store) ListOpenDriftEvents(ctx context.Context) ([]DriftEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, site_id, resource, expected, actual, severity, status, detected_at, resolved_at
		FROM drift_events WHERE status = 'open' ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("drift list: %w", err)
	}
	defer rows.Close()

	out := []DriftEvent{}
	for rows.Next() {
		var e DriftEvent
		if err := rows.Scan(&e.ID, &e.SiteID, &e.Resource, &e.Expected, &e.Actual,
			&e.Severity, &e.Status, &e.DetectedAt, &e.ResolvedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ResolveDriftEvents, bir sitenin açık drift kayıtlarını çözülmüş işaretler
// (Repair sonrası).
func (s *Store) ResolveDriftEvents(ctx context.Context, siteID string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE drift_events
		SET status = 'resolved', resolved_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
		WHERE site_id = ? AND status = 'open'`, siteID); err != nil {
		return fmt.Errorf("drift resolve: %w", err)
	}
	return nil
}
