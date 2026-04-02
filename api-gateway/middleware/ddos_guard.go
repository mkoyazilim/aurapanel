package middleware

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ddosEnvEnabled     = "AURAPANEL_DDOS_ENABLED"
	ddosEnvProfile     = "AURAPANEL_DDOS_PROFILE"
	ddosEnvGlobalRPS   = "AURAPANEL_DDOS_GLOBAL_RPS"
	ddosEnvGlobalBurst = "AURAPANEL_DDOS_GLOBAL_BURST"
	ddosEnvAuthRPS     = "AURAPANEL_DDOS_AUTH_RPS"
	ddosEnvAuthBurst   = "AURAPANEL_DDOS_AUTH_BURST"
)

type ddosGuardConfig struct {
	Enabled     bool
	Profile     string
	GlobalRPS   float64
	GlobalBurst float64
	AuthRPS     float64
	AuthBurst   float64
}

type ddosBucket struct {
	Tokens     float64
	LastRefill time.Time
	LastSeen   time.Time
}

type ddosLimiterState struct {
	mu          sync.Mutex
	global      map[string]*ddosBucket
	auth        map[string]*ddosBucket
	lastCleanup time.Time
}

func DDoSGuardMiddleware(next http.Handler) http.Handler {
	cfg := loadDDoSGuardConfig()
	if !cfg.Enabled {
		return next
	}

	state := &ddosLimiterState{
		global:      map[string]*ddosBucket{},
		auth:        map[string]*ddosBucket{},
		lastCleanup: time.Now(),
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldBypassDDoSGuard(r) {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := ddosClientIP(r)
		if clientIP == "" {
			clientIP = "unknown"
		}

		now := time.Now()
		if !state.consumeToken(now, state.global, clientIP, cfg.GlobalRPS, cfg.GlobalBurst) {
			WriteError(w, r, http.StatusTooManyRequests, "SECURITY_DDOS_RATE_LIMIT", "Gateway DDoS protection limit exceeded.")
			return
		}

		if isDDoSAuthPath(r.URL.Path) && !state.consumeToken(now, state.auth, clientIP, cfg.AuthRPS, cfg.AuthBurst) {
			WriteError(w, r, http.StatusTooManyRequests, "SECURITY_DDOS_AUTH_LIMIT", "Authentication protection limit exceeded.")
			return
		}

		w.Header().Set("X-Aura-DDoS-Profile", cfg.Profile)
		next.ServeHTTP(w, r)
	})
}

func loadDDoSGuardConfig() ddosGuardConfig {
	profile := normalizeDDoSProfile(os.Getenv(ddosEnvProfile))
	enabled := parseEnvBool(os.Getenv(ddosEnvEnabled))
	if !enabled && profile != "off" {
		enabled = true
	}

	defaultGlobalRPS := 120
	defaultGlobalBurst := 240
	defaultAuthRPS := 20
	defaultAuthBurst := 40

	if profile == "strict" {
		defaultGlobalRPS = 70
		defaultGlobalBurst = 140
		defaultAuthRPS = 12
		defaultAuthBurst = 24
	}

	if !enabled {
		profile = "off"
	}
	if enabled && profile == "off" {
		profile = "standard"
	}

	globalRPS := parseEnvIntOr(os.Getenv(ddosEnvGlobalRPS), defaultGlobalRPS, 5, 5000)
	globalBurst := parseEnvIntOr(os.Getenv(ddosEnvGlobalBurst), defaultGlobalBurst, globalRPS, 10000)
	authRPS := parseEnvIntOr(os.Getenv(ddosEnvAuthRPS), defaultAuthRPS, 2, 1000)
	authBurst := parseEnvIntOr(os.Getenv(ddosEnvAuthBurst), defaultAuthBurst, authRPS, 5000)

	return ddosGuardConfig{
		Enabled:     enabled,
		Profile:     profile,
		GlobalRPS:   float64(globalRPS),
		GlobalBurst: float64(globalBurst),
		AuthRPS:     float64(authRPS),
		AuthBurst:   float64(authBurst),
	}
}

func (s *ddosLimiterState) consumeToken(now time.Time, buckets map[string]*ddosBucket, key string, rate, burst float64) bool {
	if rate <= 0 || burst <= 0 {
		return true
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(now)

	bucket, ok := buckets[key]
	if !ok {
		bucket = &ddosBucket{
			Tokens:     burst,
			LastRefill: now,
			LastSeen:   now,
		}
		buckets[key] = bucket
	}

	elapsed := now.Sub(bucket.LastRefill).Seconds()
	if elapsed > 0 {
		bucket.Tokens += elapsed * rate
		if bucket.Tokens > burst {
			bucket.Tokens = burst
		}
		bucket.LastRefill = now
	}
	bucket.LastSeen = now

	if bucket.Tokens < 1 {
		return false
	}
	bucket.Tokens--
	return true
}

func (s *ddosLimiterState) cleanupExpiredLocked(now time.Time) {
	if now.Sub(s.lastCleanup) < 90*time.Second {
		return
	}
	s.lastCleanup = now
	expiry := 15 * time.Minute
	for key, bucket := range s.global {
		if now.Sub(bucket.LastSeen) > expiry {
			delete(s.global, key)
		}
	}
	for key, bucket := range s.auth {
		if now.Sub(bucket.LastSeen) > expiry {
			delete(s.auth, key)
		}
	}
}

func shouldBypassDDoSGuard(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return true
	}
	path := strings.TrimSpace(r.URL.Path)
	if path == "/api/health" {
		return true
	}

	ip := ddosClientIP(r)
	if parsed := net.ParseIP(ip); parsed != nil && parsed.IsLoopback() {
		return true
	}
	return false
}

func ddosClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		first := xff
		if idx := strings.Index(first, ","); idx >= 0 {
			first = first[:idx]
		}
		first = strings.TrimSpace(first)
		if host, _, err := net.SplitHostPort(first); err == nil {
			first = strings.TrimSpace(host)
		}
		return first
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return strings.TrimSpace(host)
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func isDDoSAuthPath(path string) bool {
	path = strings.TrimSpace(path)
	return path == "/api/auth/login" || path == "/api/v1/auth/login"
}

func normalizeDDoSProfile(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return "strict"
	case "standard":
		return "standard"
	default:
		return "off"
	}
}

func parseEnvIntOr(raw string, fallback, minValue, maxValue int) int {
	value := fallback
	if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
		value = parsed
	}
	if value < minValue {
		value = minValue
	}
	if value > maxValue {
		value = maxValue
	}
	return value
}

func parseEnvBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
