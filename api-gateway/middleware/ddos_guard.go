package middleware

import (
	"net"
	"net/http"
	"net/netip"
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
	ddosEnvTrustedCIDR = "AURAPANEL_TRUSTED_PROXY_CIDRS"
)

type ddosGuardConfig struct {
	Enabled          bool
	Profile          string
	GlobalRPS        float64
	GlobalBurst      float64
	AuthRPS          float64
	AuthBurst        float64
	TrustedProxyCIDR []netip.Prefix
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
		if shouldBypassDDoSGuard(r, cfg.TrustedProxyCIDR) {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := ddosClientIP(r, cfg.TrustedProxyCIDR)
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
		Enabled:          enabled,
		Profile:          profile,
		GlobalRPS:        float64(globalRPS),
		GlobalBurst:      float64(globalBurst),
		AuthRPS:          float64(authRPS),
		AuthBurst:        float64(authBurst),
		TrustedProxyCIDR: parseTrustedProxyCIDRs(os.Getenv(ddosEnvTrustedCIDR)),
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

func shouldBypassDDoSGuard(r *http.Request, trusted []netip.Prefix) bool {
	if r == nil || r.URL == nil {
		return true
	}
	path := strings.TrimSpace(r.URL.Path)
	if path == "/api/health" {
		return true
	}

	ip := ddosClientIP(r, trusted)
	if parsed := net.ParseIP(ip); parsed != nil && parsed.IsLoopback() {
		return true
	}
	return false
}

func ddosClientIP(r *http.Request, trusted []netip.Prefix) string {
	if r == nil {
		return ""
	}
	remoteIP := remoteAddrIP(r.RemoteAddr)
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" && trustedProxySource(remoteIP, trusted) {
		first := xff
		if idx := strings.Index(first, ","); idx >= 0 {
			first = first[:idx]
		}
		first = strings.TrimSpace(first)
		if host, _, err := net.SplitHostPort(first); err == nil {
			first = strings.TrimSpace(host)
		}
		if parsed := net.ParseIP(first); parsed != nil {
			return parsed.String()
		}
	}
	if remoteIP != "" {
		return remoteIP
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func remoteAddrIP(remoteAddr string) string {
	value := strings.TrimSpace(remoteAddr)
	if value == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(value)
	if err != nil {
		host = value
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if parsed := net.ParseIP(host); parsed != nil {
		return parsed.String()
	}
	return ""
}

func trustedProxySource(remoteIP string, trusted []netip.Prefix) bool {
	if remoteIP == "" {
		return false
	}
	addr, err := netip.ParseAddr(remoteIP)
	if err != nil {
		return false
	}
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func parseTrustedProxyCIDRs(raw string) []netip.Prefix {
	defaults := []string{"127.0.0.0/8", "::1/128"}
	values := defaults
	if strings.TrimSpace(raw) != "" {
		values = strings.Split(raw, ",")
	}

	parsed := make([]netip.Prefix, 0, len(values))
	for _, item := range values {
		candidate := strings.TrimSpace(item)
		if candidate == "" {
			continue
		}
		if strings.Contains(candidate, "/") {
			prefix, err := netip.ParsePrefix(candidate)
			if err != nil {
				continue
			}
			parsed = append(parsed, prefix.Masked())
			continue
		}
		addr, err := netip.ParseAddr(candidate)
		if err != nil {
			continue
		}
		bits := 32
		if addr.Is6() {
			bits = 128
		}
		parsed = append(parsed, netip.PrefixFrom(addr, bits).Masked())
	}
	if len(parsed) == 0 {
		loopback4, _ := netip.ParsePrefix("127.0.0.0/8")
		loopback6, _ := netip.ParsePrefix("::1/128")
		return []netip.Prefix{loopback4, loopback6}
	}
	return parsed
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
