// Package audit, tüm kritik işlemlerin append-only kaydını sağlar.
//
// Kayıt alanları: timestamp, user, IP, action, target, result, request_id
// (ARCHITECTURE §9.8). Dosya içeriği ASLA audit log'a yazılmaz.
package audit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/store"
)

// Event, yazılacak tek bir audit olayı.
type Event struct {
	User      string
	IP        string
	Action    string
	Target    string
	Result    string
	RequestID string
	Extra     map[string]any
}

// Service, audit kayıtlarını store üzerinden yazar ve okur.
type Service struct {
	st *store.Store
}

// New, Service oluşturur.
func New(st *store.Store) *Service { return &Service{st: st} }

// Write, olayı append-only audit_logs tablosuna yazar.
// RequestID verilmediyse otomatik üretilir; Result boşsa "success" kabul edilir.
func (s *Service) Write(ctx context.Context, e Event) error {
	if e.RequestID == "" {
		e.RequestID = newRequestID()
	}
	if e.Result == "" {
		e.Result = "success"
	}
	extra := "{}"
	if e.Extra != nil {
		b, err := json.Marshal(e.Extra)
		if err != nil {
			return err
		}
		extra = string(b)
	}
	_, err := s.st.InsertAuditLog(ctx, store.AuditLog{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		User:      e.User,
		IP:        e.IP,
		Action:    e.Action,
		Target:    e.Target,
		Result:    e.Result,
		RequestID: e.RequestID,
		Extra:     extra,
	})
	return err
}

// List, en yeni kayıtları döndürür.
func (s *Service) List(ctx context.Context, limit int) ([]store.AuditLog, error) {
	return s.st.ListAuditLogs(ctx, limit)
}

// newRequestID, crypto/rand ile 16 byte rastgele hex üretir.
func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Kripto kaynağı arızalandıysa bile boş ID dönme; zaman tabanlı yedek.
		return "req-" + hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b)
}
