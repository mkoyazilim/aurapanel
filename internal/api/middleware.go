package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/auth"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

type ctxKey string

const (
	ctxRequestID ctxKey = "request_id"
	ctxUser      ctxKey = "user"
	ctxSessionID ctxKey = "session_id"
	ctxIsPAT     ctxKey = "is_pat"
)

const sessionCookie = "aurapanel_session"

// requestIDMiddleware, istek kimliğini bağlama koyar.
func (s *Server) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = auth.NewRequestID()
		}
		w.Header().Set("X-Request-ID", rid)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxRequestID, rid)))
	})
}

// recoverMiddleware, panic'leri yakalar ve 500 döner (asla çökmemeli).
func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.deps.Log.Error("panic", "error", fmt.Sprint(rec), "path", r.URL.Path)
				writeErr(w, http.StatusInternalServerError, "iç hata")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// publicPaths, oturum gerektirmeyen uç noktalar.
var publicPaths = map[string]bool{
	"GET /healthz":           true,
	"POST /api/v1/auth/login": true,
}

// mustChangeExempt, zorunlu şifre değişiminden muaf uç noktalar.
var mustChangeExempt = map[string]bool{
	"POST /api/v1/auth/change-password": true,
	"POST /api/v1/auth/logout":          true,
	"GET /api/v1/auth/me":               true,
}

// authMiddleware: session/PAT doğrulama + CSRF + zorunlu şifre değişimi.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		if publicPaths[key] {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()
		user, sessionID, isPAT, err := s.authenticate(r)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, err.Error())
			return
		}
		ctx = context.WithValue(ctx, ctxUser, user)
		ctx = context.WithValue(ctx, ctxSessionID, sessionID)
		ctx = context.WithValue(ctx, ctxIsPAT, isPAT)

		// CSRF: oturum tabanlı isteklerde durum değiştiren yöntemler.
		if !isPAT && r.Method != http.MethodGet && r.Method != http.MethodHead {
			if !mustChangeExempt[key] {
				token, err := s.deps.Sessions.CSRFToken(ctx, sessionID)
				if err != nil || r.Header.Get("X-CSRF-Token") != token {
					writeErr(w, http.StatusForbidden, "CSRF doğrulaması başarısız")
					return
				}
			}
		}

		// Zorunlu şifre değişimi (ilk kurulum): diğer her şey engellenir.
		if user.MustChangePassword && !mustChangeExempt[key] {
			writeErr(w, http.StatusForbidden, "varsayılan şifrenizi değiştirmelisiniz")
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authenticate, session cookie veya Authorization PAT ile kimlik doğrular.
func (s *Server) authenticate(r *http.Request) (user *store.User, sessionID string, isPAT bool, err error) {
	// PAT öncelikli: Authorization: Bearer ap_...
	if ah := r.Header.Get("Authorization"); strings.HasPrefix(ah, "Bearer ap_") {
		hash := auth.HashPAT(strings.TrimPrefix(ah, "Bearer "))
		tok, err := s.deps.Store.GetPATTokenByHash(r.Context(), hash)
		if err != nil {
			return nil, "", false, fmt.Errorf("kimlik doğrulama hatası")
		}
		if tok == nil {
			return nil, "", false, fmt.Errorf("geçersiz token")
		}
		if tok.ExpiresAt.Valid && tok.ExpiresAt.String < time.Now().UTC().Format(time.RFC3339) {
			return nil, "", false, fmt.Errorf("token süresi doldu")
		}
		u, err := s.deps.Store.GetUserByID(r.Context(), tok.UserID)
		if err != nil || u == nil {
			return nil, "", false, fmt.Errorf("geçersiz token")
		}
		s.deps.Store.TouchPATToken(r.Context(), tok.ID)
		return u, "", true, nil
	}

	// Session cookie.
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, "", false, fmt.Errorf("oturum yok")
	}
	u, err := s.deps.Sessions.Validate(r.Context(), c.Value)
	if err != nil {
		return nil, "", false, fmt.Errorf("oturum geçersiz")
	}
	return u, c.Value, false, nil
}

// userFromCtx, bağlamdaki kullanıcıyı döndürür.
func userFromCtx(ctx context.Context) (*store.User, bool) {
	u, ok := ctx.Value(ctxUser).(*store.User)
	return u, ok && u != nil
}

// requireAdmin, ctx'teki kullanıcının admin olmasını zorunlu kılar.
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	u, ok := userFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "kimlik doğrulaması yok")
		return nil, false
	}
	role, err := s.deps.Store.GetRoleName(r.Context(), u.RoleID)
	if err != nil || role != "admin" {
		writeErr(w, http.StatusForbidden, "yönetici yetkisi gerekli")
		return nil, false
	}
	return u, true
}

// loginThrottle, per-IP + per-hesap giriş kısıtlaması (ARCHITECTURE §9.3).
type loginThrottle struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	max      int
	window   time.Duration
}

func newLoginThrottle(max int, window time.Duration) *loginThrottle {
	return &loginThrottle{attempts: map[string][]time.Time{}, max: max, window: window}
}

// Check, denemeye izin var mı? (IP ve hesap anahtarları ayrı sayılır)
func (t *loginThrottle) Check(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-t.window)
	recent := t.attempts[key][:0]
	for _, ts := range t.attempts[key] {
		if ts.After(cutoff) {
			recent = append(recent, ts)
		}
	}
	t.attempts[key] = recent
	return len(recent) < t.max
}

// Record, denemeyi kaydeder.
func (t *loginThrottle) Record(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.attempts[key] = append(t.attempts[key], time.Now())
}

// Reset, başarılı girişte sayacı sıfırlar.
func (t *loginThrottle) Reset(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.attempts, key)
}

// clientIP, istek IP'sini döndürür (reverse proxy desteği).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
