package store

import (
	"context"
)

// AuditLog, append-only audit kaydı (ARCHITECTURE §9.8).
// Extra alanı JSON metindir; dosya içeriği ASLA buraya yazılmaz.
type AuditLog struct {
	ID        int64  `json:"id"`
	Timestamp string `json:"timestamp"`
	User      string `json:"user"`
	IP        string `json:"ip"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Result    string `json:"result"`
	RequestID string `json:"request_id"`
	Extra     string `json:"extra"`
}

// InsertAuditLog, kaydı ekler ve ID'sini döndürür.
func (s *Store) InsertAuditLog(ctx context.Context, e AuditLog) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO audit_logs
		(timestamp, user, ip, action, target, result, request_id, extra)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Timestamp, e.User, e.IP, e.Action, e.Target, e.Result, e.RequestID, e.Extra)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListAuditLogs, en yeni kayıtları (ID azalan) limit kadar döndürür.
func (s *Store) ListAuditLogs(ctx context.Context, limit int) ([]AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, timestamp, user, ip, action, target, result, request_id, extra
		FROM audit_logs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AuditLog, 0, limit)
	for rows.Next() {
		var e AuditLog
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.User, &e.IP, &e.Action, &e.Target, &e.Result, &e.RequestID, &e.Extra); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
