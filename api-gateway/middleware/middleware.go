package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	defaultIssuer         = "aurapanel-gateway"
	defaultAudience       = "aurapanel-ui"
	defaultAuthCookieName = "aurapanel_session"
)

type contextKey string

const (
	authUserContextKey  contextKey = "auth_user"
	requestIDContextKey contextKey = "request_id"
)

type AuthUser struct {
	Email        string
	Role         string
	Name         string
	Username     string
	Permissions  []string
	RolePolicyID string
	RolePolicy   string
}

type gatewayClaims struct {
	Email        string   `json:"email"`
	Name         string   `json:"name"`
	Role         string   `json:"role"`
	Username     string   `json:"username,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
	RolePolicyID string   `json:"role_policy_id,omitempty"`
	RolePolicy   string   `json:"role_policy_name,omitempty"`
	jwt.RegisteredClaims
}

type ErrorResponse struct {
	Status    string `json:"status"`
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

var (
	originInit    sync.Once
	cachedOrigins map[string]struct{}
)

func devSimulationEnabled() bool {
	normalized := strings.ToLower(strings.TrimSpace(os.Getenv("AURAPANEL_DEV_SIMULATION")))
	return normalized == "1" || normalized == "true" || normalized == "yes" || normalized == "on"
}

func JwtSecret() string {
	return strings.TrimSpace(os.Getenv("AURAPANEL_JWT_SECRET"))
}

func JwtIssuer() string {
	issuer := strings.TrimSpace(os.Getenv("AURAPANEL_JWT_ISSUER"))
	if issuer == "" {
		return defaultIssuer
	}
	return issuer
}

func JwtAudience() string {
	aud := strings.TrimSpace(os.Getenv("AURAPANEL_JWT_AUDIENCE"))
	if aud == "" {
		return defaultAudience
	}
	return aud
}

func RequireSecurityConfig() error {
	if devSimulationEnabled() {
		log.Printf("[SECURITY] AURAPANEL_DEV_SIMULATION is enabled; strict startup checks are relaxed")
		return nil
	}

	secret := JwtSecret()
	if secret == "" {
		return errors.New("AURAPANEL_JWT_SECRET is required in production mode")
	}
	if len(secret) < 32 {
		return errors.New("AURAPANEL_JWT_SECRET must be at least 32 characters")
	}
	if strings.EqualFold(secret, "change-me-in-production") {
		return errors.New("AURAPANEL_JWT_SECRET must not use insecure default value")
	}
	proxyToken := strings.TrimSpace(os.Getenv("AURAPANEL_INTERNAL_PROXY_TOKEN"))
	if len(proxyToken) < 32 {
		return errors.New("AURAPANEL_INTERNAL_PROXY_TOKEN must be set and at least 32 characters")
	}

	return nil
}

// RequestIDMiddleware injects request id into context and response.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if requestID == "" {
			requestID = generateRequestID()
		}

		w.Header().Set("X-Request-Id", requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func generateRequestID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("rid-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(requestIDContextKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Logger logs each request.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[req_id=%s] [%s] %s - %v", GetRequestID(r.Context()), r.Method, r.URL.Path, time.Since(start))
	})
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Status:    "error",
		Error:     code,
		Message:   message,
		RequestID: GetRequestID(r.Context()),
	})
}

// AuthMiddleware validates JWT coming from headers.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := extractToken(r)
		if tokenString == "" {
			WriteError(w, r, http.StatusUnauthorized, "AUTH_MISSING_TOKEN", "Authorization token is required")
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &gatewayClaims{}, func(token *jwt.Token) (interface{}, error) {
			if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(JwtSecret()), nil
		},
			jwt.WithIssuer(JwtIssuer()),
			jwt.WithAudience(JwtAudience()),
			jwt.WithLeeway(30*time.Second),
		)
		if err != nil || !token.Valid {
			WriteError(w, r, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "Token validation failed")
			return
		}

		claims, ok := token.Claims.(*gatewayClaims)
		if !ok {
			WriteError(w, r, http.StatusUnauthorized, "AUTH_INVALID_CLAIMS", "Token claims are invalid")
			return
		}

		role := strings.ToLower(strings.TrimSpace(claims.Role))
		if role != "admin" && role != "reseller" && role != "user" {
			WriteError(w, r, http.StatusUnauthorized, "AUTH_INVALID_ROLE", "Token role is invalid")
			return
		}
		username := sanitizeIdentity(strings.TrimSpace(claims.Username))
		if username == "" {
			local := strings.Split(strings.ToLower(strings.TrimSpace(claims.Email)), "@")
			if len(local) > 0 {
				username = sanitizeIdentity(local[0])
			}
		}

		ctx := context.WithValue(r.Context(), authUserContextKey, AuthUser{
			Email:        claims.Email,
			Role:         role,
			Name:         claims.Name,
			Username:     username,
			Permissions:  append([]string(nil), claims.Permissions...),
			RolePolicyID: strings.TrimSpace(claims.RolePolicyID),
			RolePolicy:   strings.TrimSpace(claims.RolePolicy),
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func sanitizeIdentity(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	builder := strings.Builder{}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func extractToken(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}

	if cookie, err := r.Cookie(authCookieName()); err == nil {
		if value := strings.TrimSpace(cookie.Value); value != "" {
			return value
		}
	}

	if isWebsocketUpgrade(r) {
		return strings.TrimSpace(r.URL.Query().Get("token"))
	}

	return ""
}

func authCookieName() string {
	value := strings.TrimSpace(os.Getenv("AURAPANEL_AUTH_COOKIE_NAME"))
	if value == "" {
		return defaultAuthCookieName
	}
	return value
}

func isWebsocketUpgrade(r *http.Request) bool {
	if strings.HasSuffix(strings.TrimSpace(r.URL.Path), "/terminal/ws") {
		return true
	}
	upgrade := strings.ToLower(strings.TrimSpace(r.Header.Get("Upgrade")))
	connection := strings.ToLower(strings.TrimSpace(r.Header.Get("Connection")))
	return upgrade == "websocket" || strings.Contains(connection, "upgrade")
}

func GetAuthUser(ctx context.Context) (AuthUser, bool) {
	v := ctx.Value(authUserContextKey)
	if v == nil {
		return AuthUser{}, false
	}

	user, ok := v.(AuthUser)
	return user, ok
}

func allowedOrigins() map[string]struct{} {
	originInit.Do(func() {
		cachedOrigins = make(map[string]struct{})

		raw := strings.TrimSpace(os.Getenv("AURAPANEL_ALLOWED_ORIGINS"))
		if raw == "" && devSimulationEnabled() {
			raw = "http://127.0.0.1:5173,http://localhost:5173"
		}

		for _, item := range strings.Split(raw, ",") {
			origin := strings.TrimSpace(item)
			if origin == "" {
				continue
			}
			cachedOrigins[origin] = struct{}{}
		}
	})

	return cachedOrigins
}

func isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	_, ok := allowedOrigins()[origin]
	return ok
}

// CorsMiddleware injects CORS headers with allowlist logic.
func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			if !isOriginAllowed(origin) {
				if r.Method == http.MethodOptions {
					WriteError(w, r, http.StatusForbidden, "CORS_ORIGIN_DENIED", "Origin is not allowed")
					return
				}
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-Id")
				w.Header().Set("Access-Control-Expose-Headers", "X-Request-Id")
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
