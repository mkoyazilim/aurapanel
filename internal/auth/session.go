package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/store"
)

// Oturum süreleri (ARCHITECTURE §9.1).
const (
	sessionTTL       = 12 * time.Hour
	sessionCleanupAt = 100 // her N oturum açmada bir temizlik
)

// SessionStore, server-side session yönetimi (SQLite sessions tablosu).
type SessionStore struct {
	st        *store.Store
	creations int
}

// NewSessionStore, SessionStore oluşturur.
func NewSessionStore(st *store.Store) *SessionStore { return &SessionStore{st: st} }

// Create, yeni oturum kurar: (sessionID, csrfToken).
func (s *SessionStore) Create(ctx context.Context, userID int64, ip, userAgent string) (string, string, error) {
	id := randHex(32)
	csrf := randHex(32)
	if err := s.st.InsertSession(ctx, store.Session{
		ID: id, UserID: userID, IP: ip, UserAgent: userAgent,
		CSRFToken: csrf,
		ExpiresAt: time.Now().UTC().Add(sessionTTL).Format(time.RFC3339),
	}); err != nil {
		return "", "", err
	}
	// Tembel temizlik.
	s.creations++
	if s.creations%sessionCleanupAt == 0 {
		s.st.DeleteExpiredSessions(ctx, time.Now().UTC().Format(time.RFC3339))
	}
	return id, csrf, nil
}

// Validate, oturumu doğrular ve kullanıcıyı döndürür.
func (s *SessionStore) Validate(ctx context.Context, sessionID string) (*store.User, error) {
	if sessionID == "" {
		return nil, errors.New("oturum yok")
	}
	se, err := s.st.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if se == nil {
		return nil, errors.New("oturum geçersiz")
	}
	if se.ExpiresAt < time.Now().UTC().Format(time.RFC3339) {
		s.st.DeleteSession(ctx, sessionID)
		return nil, errors.New("oturum süresi doldu")
	}
	u, err := s.st.GetUserByID(ctx, se.UserID)
	if err != nil {
		return nil, err
	}
	if u == nil || u.Status != "active" {
		return nil, errors.New("kullanıcı etkin değil")
	}
	return u, nil
}

// Destroy, oturumu kapatır.
func (s *SessionStore) Destroy(ctx context.Context, sessionID string) error {
	return s.st.DeleteSession(ctx, sessionID)
}

// CSRFToken, oturumun CSRF token'ını döndürür.
func (s *SessionStore) CSRFToken(ctx context.Context, sessionID string) (string, error) {
	se, err := s.st.GetSession(ctx, sessionID)
	if err != nil || se == nil {
		return "", errors.New("oturum geçersiz")
	}
	return se.CSRFToken, nil
}

// randHex, güvenli rastgele hex döndürür.
func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// NewRequestID, istek kimliği üretir (16 bayt hex).
func NewRequestID() string { return randHex(16) }
