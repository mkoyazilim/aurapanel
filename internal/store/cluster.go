package store

import (
	"context"
	"fmt"
	"time"
)

// ClusterEvent, cluster_events tablosundaki tek kayıt.
type ClusterEvent struct {
	ID        int64  `json:"id"`
	ServerID  string `json:"server_id"`
	EventType string `json:"event_type"`
	Detail    string `json:"detail"`
	CreatedAt string `json:"created_at"`
}

// InsertClusterEvent, yeni olay kaydı ekler.
func (s *Store) InsertClusterEvent(ctx context.Context, e ClusterEvent) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cluster_events (server_id, event_type, detail) VALUES (?, ?, ?)`,
		e.ServerID, e.EventType, e.Detail)
	return err
}

// ListClusterEvents, olayları döndürür. serverID "" ise tüm sunucular; limit <= 0 ise 50.
func (s *Store) ListClusterEvents(ctx context.Context, serverID string, limit int) ([]ClusterEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	var (
		rows interface {
			Next() bool
			Scan(...any) error
			Close() error
		}
		err error
	)
	if serverID == "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, server_id, event_type, detail, created_at FROM cluster_events ORDER BY id DESC LIMIT ?`,
			limit)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, server_id, event_type, detail, created_at FROM cluster_events WHERE server_id = ? ORDER BY id DESC LIMIT ?`,
			serverID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("cluster events: %w", err)
	}
	defer rows.Close()

	var out []ClusterEvent
	for rows.Next() {
		var e ClusterEvent
		if err := rows.Scan(&e.ID, &e.ServerID, &e.EventType, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// RotateServerAPIKey, sunucunun API anahtarını atomik olarak değiştirir.
func (s *Store) RotateServerAPIKey(ctx context.Context, serverID, newKey string) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	res, err := s.db.ExecContext(ctx,
		`UPDATE servers SET api_key = ?, updated_at = ? WHERE id = ?`,
		newKey, now, serverID)
	if err != nil {
		return fmt.Errorf("rotate key: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("server not found: %s", serverID)
	}
	return nil
}
