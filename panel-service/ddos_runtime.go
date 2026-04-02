package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
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

type ddosConfig struct {
	Enabled     bool
	Profile     string
	GlobalRPS   int
	GlobalBurst int
	AuthRPS     int
	AuthBurst   int
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

func ddosProfileDefaults(profile string) (int, int, int, int) {
	if profile == "strict" {
		return 70, 140, 12, 24
	}
	return 120, 240, 20, 40
}

func parseDDoSInt(raw string, fallback, minValue, maxValue int) int {
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

func ddosEnvValue(key string) string {
	return firstNonEmpty(
		strings.TrimSpace(os.Getenv(key)),
		strings.TrimSpace(readEnvFileValue(adminGatewayEnvPath(), key)),
		strings.TrimSpace(readEnvFileValue(adminServiceEnvPath(), key)),
	)
}

func loadDDoSRuntimeConfig() ddosConfig {
	profile := normalizeDDoSProfile(ddosEnvValue(ddosEnvProfile))
	enabled := envBoolEnabled(ddosEnvValue(ddosEnvEnabled))
	if !enabled && profile != "off" {
		enabled = true
	}
	if enabled && profile == "off" {
		profile = "standard"
	}
	if !enabled {
		profile = "off"
	}

	defaultGlobalRPS, defaultGlobalBurst, defaultAuthRPS, defaultAuthBurst := ddosProfileDefaults(profile)

	cfg := ddosConfig{
		Enabled:     enabled,
		Profile:     profile,
		GlobalRPS:   parseDDoSInt(ddosEnvValue(ddosEnvGlobalRPS), defaultGlobalRPS, 5, 5000),
		GlobalBurst: parseDDoSInt(ddosEnvValue(ddosEnvGlobalBurst), defaultGlobalBurst, 5, 10000),
		AuthRPS:     parseDDoSInt(ddosEnvValue(ddosEnvAuthRPS), defaultAuthRPS, 2, 1000),
		AuthBurst:   parseDDoSInt(ddosEnvValue(ddosEnvAuthBurst), defaultAuthBurst, 2, 5000),
	}

	if cfg.GlobalBurst < cfg.GlobalRPS {
		cfg.GlobalBurst = cfg.GlobalRPS
	}
	if cfg.AuthBurst < cfg.AuthRPS {
		cfg.AuthBurst = cfg.AuthRPS
	}
	return cfg
}

func sanitizeDDoSConfig(cfg ddosConfig) (ddosConfig, []string) {
	warnings := []string{}
	cfg.Profile = normalizeDDoSProfile(cfg.Profile)
	if cfg.Enabled && cfg.Profile == "off" {
		cfg.Profile = "standard"
	}
	if !cfg.Enabled {
		cfg.Profile = "off"
	}
	defaultGlobalRPS, defaultGlobalBurst, defaultAuthRPS, defaultAuthBurst := ddosProfileDefaults(cfg.Profile)

	cfg.GlobalRPS = parseDDoSInt(strconv.Itoa(cfg.GlobalRPS), defaultGlobalRPS, 5, 5000)
	cfg.GlobalBurst = parseDDoSInt(strconv.Itoa(cfg.GlobalBurst), defaultGlobalBurst, cfg.GlobalRPS, 10000)
	cfg.AuthRPS = parseDDoSInt(strconv.Itoa(cfg.AuthRPS), defaultAuthRPS, 2, 1000)
	cfg.AuthBurst = parseDDoSInt(strconv.Itoa(cfg.AuthBurst), defaultAuthBurst, cfg.AuthRPS, 5000)

	if cfg.AuthRPS > cfg.GlobalRPS {
		warnings = append(warnings, "Authentication rate limit is higher than global limit; global guard will trigger first.")
	}
	if cfg.Profile == "strict" && cfg.GlobalRPS > 150 {
		warnings = append(warnings, "Strict profile is configured with high global RPS; this reduces strictness.")
	}
	return cfg, warnings
}

func ddosConfigEqual(a, b ddosConfig) bool {
	return a.Enabled == b.Enabled &&
		a.Profile == b.Profile &&
		a.GlobalRPS == b.GlobalRPS &&
		a.GlobalBurst == b.GlobalBurst &&
		a.AuthRPS == b.AuthRPS &&
		a.AuthBurst == b.AuthBurst
}

func persistDDoSConfig(cfg ddosConfig) error {
	enabledValue := "0"
	if cfg.Enabled {
		enabledValue = "1"
	}
	updates := map[string]string{
		ddosEnvEnabled:     enabledValue,
		ddosEnvProfile:     cfg.Profile,
		ddosEnvGlobalRPS:   strconv.Itoa(cfg.GlobalRPS),
		ddosEnvGlobalBurst: strconv.Itoa(cfg.GlobalBurst),
		ddosEnvAuthRPS:     strconv.Itoa(cfg.AuthRPS),
		ddosEnvAuthBurst:   strconv.Itoa(cfg.AuthBurst),
	}

	for _, path := range []string{adminGatewayEnvPath(), adminServiceEnvPath()} {
		if err := writeEnvFileValues(path, updates); err != nil {
			return err
		}
	}
	for key, value := range updates {
		_ = os.Setenv(key, value)
	}
	return nil
}

func (s *service) ddosCompatibilityNotes(cfg ddosConfig) ([]string, []string) {
	notes := []string{
		"DDoS profile only controls API gateway request rates and does not rewrite firewall or Fail2Ban rules.",
	}
	recommendations := []string{}

	snapshot := collectSecuritySnapshot()
	fail2banActive := serviceActive("fail2ban")

	if snapshot.FirewallActive {
		notes = append(notes, "Firewall layer is active.")
	} else {
		recommendations = append(recommendations, "Activate firewall baseline (ufw/firewalld/nftables) for volumetric pre-filtering.")
	}
	if fail2banActive {
		notes = append(notes, "Fail2Ban is active for brute-force response.")
	} else {
		recommendations = append(recommendations, "Enable Fail2Ban to complement gateway rate controls against brute-force attempts.")
	}
	if snapshot.MLWAFActive {
		notes = append(notes, "ModSecurity/OWASP CRS appears active.")
	} else {
		recommendations = append(recommendations, "Enable WAF profile to improve L7 signature-based protection.")
	}
	if cfg.Enabled && cfg.Profile == "strict" {
		recommendations = append(recommendations, "Strict profile may throttle high-traffic APIs; monitor 429 rate after activation.")
	}
	if !cfg.Enabled {
		recommendations = append(recommendations, "DDoS guard is disabled; only external layers (firewall/WAF/upstream) are protecting API burst traffic.")
	}
	return notes, recommendations
}

func (s *service) handleSecurityDDoSGet(w http.ResponseWriter) {
	cfg := loadDDoSRuntimeConfig()
	notes, recommendations := s.ddosCompatibilityNotes(cfg)

	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"enabled":         cfg.Enabled,
			"profile":         cfg.Profile,
			"global_rps":      cfg.GlobalRPS,
			"global_burst":    cfg.GlobalBurst,
			"auth_rps":        cfg.AuthRPS,
			"auth_burst":      cfg.AuthBurst,
			"compatibility":   notes,
			"recommendations": recommendations,
		},
	})
}

func (s *service) handleSecurityDDoSSet(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Enabled     *bool  `json:"enabled"`
		Profile     string `json:"profile"`
		GlobalRPS   int    `json:"global_rps"`
		GlobalBurst int    `json:"global_burst"`
		AuthRPS     int    `json:"auth_rps"`
		AuthBurst   int    `json:"auth_burst"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid DDoS profile payload.")
		return
	}
	if payload.Enabled == nil {
		writeError(w, http.StatusBadRequest, "enabled field is required.")
		return
	}

	current := loadDDoSRuntimeConfig()
	next := current
	next.Enabled = *payload.Enabled
	if strings.TrimSpace(payload.Profile) != "" {
		next.Profile = normalizeDDoSProfile(payload.Profile)
	}
	if next.Enabled && next.Profile == "off" {
		next.Profile = "standard"
	}
	if !next.Enabled {
		next.Profile = "off"
	}

	if payload.GlobalRPS > 0 {
		next.GlobalRPS = payload.GlobalRPS
	}
	if payload.GlobalBurst > 0 {
		next.GlobalBurst = payload.GlobalBurst
	}
	if payload.AuthRPS > 0 {
		next.AuthRPS = payload.AuthRPS
	}
	if payload.AuthBurst > 0 {
		next.AuthBurst = payload.AuthBurst
	}

	next, warnings := sanitizeDDoSConfig(next)
	if ddosConfigEqual(current, next) {
		notes, recommendations := s.ddosCompatibilityNotes(next)
		writeJSON(w, http.StatusOK, apiResponse{
			Status:  "success",
			Message: "DDoS profile unchanged.",
			Data: map[string]interface{}{
				"enabled":         next.Enabled,
				"profile":         next.Profile,
				"global_rps":      next.GlobalRPS,
				"global_burst":    next.GlobalBurst,
				"auth_rps":        next.AuthRPS,
				"auth_burst":      next.AuthBurst,
				"restart_applied": false,
				"warnings":        warnings,
				"compatibility":   notes,
				"recommendations": recommendations,
			},
		})
		return
	}

	if err := persistDDoSConfig(next); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("DDoS profile could not be persisted: %v", err))
		return
	}

	restartApplied := false
	restartWarning := ""
	port := 8090
	s.mu.RLock()
	if s.state.GatewayPort > 0 {
		port = s.state.GatewayPort
	}
	s.mu.RUnlock()

	if runtime.GOOS != "linux" {
		restartWarning = "Gateway restart is not automatic on non-linux hosts; restart aurapanel-api manually."
	} else if !systemctlUnitExists("aurapanel-api.service") {
		restartWarning = "Gateway unit not found; restart aurapanel-api manually."
	} else {
		if err := executeServiceAction("api-gateway", "restart"); err != nil {
			restartWarning = fmt.Sprintf("Gateway restart failed: %v", err)
		} else if err := waitForGatewayHealthOnPort(port, 25*time.Second); err != nil {
			restartWarning = fmt.Sprintf("Gateway health check failed after restart: %v", err)
		} else {
			restartApplied = true
		}
	}

	if restartWarning != "" {
		warnings = append(warnings, restartWarning)
	}

	notes, recommendations := s.ddosCompatibilityNotes(next)
	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: "DDoS profile updated.",
		Data: map[string]interface{}{
			"enabled":         next.Enabled,
			"profile":         next.Profile,
			"global_rps":      next.GlobalRPS,
			"global_burst":    next.GlobalBurst,
			"auth_rps":        next.AuthRPS,
			"auth_burst":      next.AuthBurst,
			"restart_applied": restartApplied,
			"warnings":        warnings,
			"compatibility":   notes,
			"recommendations": recommendations,
		},
	})
}
