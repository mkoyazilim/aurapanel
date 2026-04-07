package main

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	watchdogEnvEnabled = "AURAPANEL_WATCHDOG_ENABLED"

	watchdogDefaultIntervalSeconds  = 20
	watchdogDefaultFailureThreshold = 3
	watchdogDefaultCooldownSeconds  = 90
	watchdogDefaultMaxLogEntries    = 400

	watchdogMinIntervalSeconds  = 10
	watchdogMaxIntervalSeconds  = 300
	watchdogMinFailureThreshold = 1
	watchdogMaxFailureThreshold = 12
	watchdogMinCooldownSeconds  = 10
	watchdogMaxCooldownSeconds  = 1800
	watchdogMinMaxLogEntries    = 100
	watchdogMaxMaxLogEntries    = 5000
)

type watchdogTarget struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

type watchdogServiceView struct {
	Name                string `json:"name"`
	Desc                string `json:"desc"`
	LastStatus          string `json:"last_status"`
	LastError           string `json:"last_error,omitempty"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	LastCheckAt         int64  `json:"last_check_at"`
	LastSuccessAt       int64  `json:"last_success_at,omitempty"`
	LastFailureAt       int64  `json:"last_failure_at,omitempty"`
	LastActionAt        int64  `json:"last_action_at,omitempty"`
}

type watchdogProbeResult struct {
	Status     string
	Message    string
	Actionable bool
}

func defaultWatchdogConfig() WatchdogConfig {
	return WatchdogConfig{
		Enabled:          watchdogEnabledDefaultFromEnv(),
		IntervalSeconds:  watchdogDefaultIntervalSeconds,
		FailureThreshold: watchdogDefaultFailureThreshold,
		CooldownSeconds:  watchdogDefaultCooldownSeconds,
		MaxLogEntries:    watchdogDefaultMaxLogEntries,
		Services:         append([]string{}, defaultWatchdogServices()...),
	}
}

func watchdogEnabledDefaultFromEnv() bool {
	raw := strings.TrimSpace(os.Getenv(watchdogEnvEnabled))
	if raw == "" {
		return true
	}
	return envBoolEnabled(raw)
}

func defaultWatchdogServices() []string {
	return []string{"api-gateway", "panel-service", "openlitespeed", "mariadb", "redis"}
}

func watchdogTargetsCatalog() []watchdogTarget {
	return []watchdogTarget{
		{Name: "api-gateway", Desc: "AuraPanel API gateway"},
		{Name: "panel-service", Desc: "AuraPanel panel service"},
		{Name: "openlitespeed", Desc: "OpenLiteSpeed web server"},
		{Name: "mariadb", Desc: "MariaDB database engine"},
		{Name: "postgresql", Desc: "PostgreSQL database engine"},
		{Name: "redis", Desc: "Redis in-memory data store"},
		{Name: "docker", Desc: "Docker container runtime"},
		{Name: "fail2ban", Desc: "Fail2Ban intrusion prevention"},
		{Name: "pdns", Desc: "PowerDNS authoritative daemon"},
		{Name: "postfix", Desc: "Postfix mail transport"},
		{Name: "dovecot", Desc: "Dovecot mail access"},
		{Name: "pure-ftpd", Desc: "FTP service"},
		{Name: "minio", Desc: "MinIO object storage"},
	}
}

func watchdogTargetDescriptions() map[string]string {
	items := watchdogTargetsCatalog()
	result := make(map[string]string, len(items))
	for _, item := range items {
		result[item.Name] = item.Desc
	}
	return result
}

func normalizeWatchdogServiceName(value string) string {
	name := strings.ToLower(strings.TrimSpace(value))
	if name == "" {
		return ""
	}
	for _, target := range watchdogTargetsCatalog() {
		if name == target.Name {
			return name
		}
	}
	return ""
}

func sanitizeWatchdogConfig(cfg WatchdogConfig) WatchdogConfig {
	if cfg.IntervalSeconds == 0 &&
		cfg.FailureThreshold == 0 &&
		cfg.CooldownSeconds == 0 &&
		cfg.MaxLogEntries == 0 &&
		len(cfg.Services) == 0 {
		defaults := defaultWatchdogConfig()
		cfg.IntervalSeconds = defaults.IntervalSeconds
		cfg.FailureThreshold = defaults.FailureThreshold
		cfg.CooldownSeconds = defaults.CooldownSeconds
		cfg.MaxLogEntries = defaults.MaxLogEntries
		cfg.Services = append([]string{}, defaults.Services...)
		cfg.Enabled = defaults.Enabled
	}

	cfg.IntervalSeconds = clampIntWithFallback(cfg.IntervalSeconds, watchdogDefaultIntervalSeconds, watchdogMinIntervalSeconds, watchdogMaxIntervalSeconds)
	cfg.FailureThreshold = clampIntWithFallback(cfg.FailureThreshold, watchdogDefaultFailureThreshold, watchdogMinFailureThreshold, watchdogMaxFailureThreshold)
	cfg.CooldownSeconds = clampIntWithFallback(cfg.CooldownSeconds, watchdogDefaultCooldownSeconds, watchdogMinCooldownSeconds, watchdogMaxCooldownSeconds)
	cfg.MaxLogEntries = clampIntWithFallback(cfg.MaxLogEntries, watchdogDefaultMaxLogEntries, watchdogMinMaxLogEntries, watchdogMaxMaxLogEntries)

	services := make([]string, 0, len(cfg.Services))
	seen := map[string]struct{}{}
	for _, item := range cfg.Services {
		name := normalizeWatchdogServiceName(item)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		services = append(services, name)
	}
	if len(services) == 0 {
		services = append([]string{}, defaultWatchdogServices()...)
	}
	cfg.Services = services
	return cfg
}

func clampIntWithFallback(value, fallback, minValue, maxValue int) int {
	if value == 0 {
		value = fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func watchdogConfigEqual(a, b WatchdogConfig) bool {
	if a.Enabled != b.Enabled ||
		a.IntervalSeconds != b.IntervalSeconds ||
		a.FailureThreshold != b.FailureThreshold ||
		a.CooldownSeconds != b.CooldownSeconds ||
		a.MaxLogEntries != b.MaxLogEntries ||
		len(a.Services) != len(b.Services) {
		return false
	}
	for i := range a.Services {
		if a.Services[i] != b.Services[i] {
			return false
		}
	}
	return true
}

func (s *service) startWatchdogWorker() {
	go func() {
		nextTick := 8 * time.Second
		timer := time.NewTimer(nextTick)
		defer timer.Stop()

		for now := range timer.C {
			nextTick = s.runWatchdogTick(now.UTC())
			if nextTick < time.Duration(watchdogMinIntervalSeconds)*time.Second {
				nextTick = time.Duration(watchdogMinIntervalSeconds) * time.Second
			}
			if nextTick > time.Duration(watchdogMaxIntervalSeconds)*time.Second {
				nextTick = time.Duration(watchdogMaxIntervalSeconds) * time.Second
			}
			timer.Reset(nextTick)
		}
	}()
}

func (s *service) runWatchdogTick(now time.Time) time.Duration {
	s.watchdogRunMu.Lock()
	defer s.watchdogRunMu.Unlock()

	s.mu.Lock()
	originalCfg := s.modules.WatchdogConfig
	cfg := sanitizeWatchdogConfig(s.modules.WatchdogConfig)
	if !watchdogConfigEqual(originalCfg, cfg) {
		s.modules.WatchdogConfig = cfg
	}
	if s.modules.WatchdogStatus == nil {
		s.modules.WatchdogStatus = map[string]WatchdogServiceState{}
	}
	if s.modules.WatchdogLogs == nil {
		s.modules.WatchdogLogs = []WatchdogLogEntry{}
	}
	gatewayPort := s.state.GatewayPort
	if gatewayPort <= 0 {
		gatewayPort = defaultGatewayPort
	}
	configChanged := !watchdogConfigEqual(originalCfg, cfg)
	services := append([]string{}, cfg.Services...)
	s.mu.Unlock()

	if configChanged {
		s.enqueueStatePersist()
	}
	if !cfg.Enabled {
		return time.Duration(cfg.IntervalSeconds) * time.Second
	}

	descMap := watchdogTargetDescriptions()
	persistNeeded := false

	for _, serviceName := range services {
		probe := watchdogProbeService(serviceName, gatewayPort)
		serviceLabel := firstNonEmpty(descMap[serviceName], serviceName)
		shouldRestart := false
		lastFailures := 0
		restartReason := ""

		s.mu.Lock()
		state := s.modules.WatchdogStatus[serviceName]
		state.Name = serviceName
		state.LastCheckAt = now.Unix()
		lastFailures = state.ConsecutiveFailures
		switch probe.Status {
		case "healthy":
			state.LastStatus = "healthy"
			state.LastSuccessAt = now.Unix()
			state.LastError = ""
			state.ConsecutiveFailures = 0
			if lastFailures > 0 {
				persistNeeded = s.appendWatchdogLogLocked("info", "check_ok", serviceName, fmt.Sprintf("%s recovered: %s", serviceLabel, probe.Message), now) || persistNeeded
			}
		case "unhealthy":
			state.LastStatus = "unhealthy"
			state.LastFailureAt = now.Unix()
			state.LastError = probe.Message
			state.ConsecutiveFailures++

			if state.ConsecutiveFailures == 1 {
				persistNeeded = s.appendWatchdogLogLocked("warn", "check_fail", serviceName, fmt.Sprintf("%s unhealthy: %s", serviceLabel, probe.Message), now) || persistNeeded
			}
			if state.ConsecutiveFailures == cfg.FailureThreshold {
				persistNeeded = s.appendWatchdogLogLocked("warn", "threshold", serviceName, fmt.Sprintf("%s reached failure threshold (%d).", serviceLabel, cfg.FailureThreshold), now) || persistNeeded
			}
			if probe.Actionable && state.ConsecutiveFailures >= cfg.FailureThreshold {
				if state.LastActionAt == 0 || (now.Unix()-state.LastActionAt) >= int64(cfg.CooldownSeconds) {
					shouldRestart = true
					state.LastActionAt = now.Unix()
					restartReason = probe.Message
				}
			}
		default:
			state.LastStatus = "unknown"
			state.ConsecutiveFailures = 0
			state.LastError = probe.Message
		}
		s.modules.WatchdogStatus[serviceName] = state
		s.mu.Unlock()

		if !shouldRestart {
			continue
		}

		scheduled, err := executeServiceActionFromPanel(serviceName, "restart")
		s.mu.Lock()
		if err != nil {
			persistNeeded = s.appendWatchdogLogLocked("error", "restart_fail", serviceName, fmt.Sprintf("%s restart failed: %v", serviceLabel, err), now) || persistNeeded
		} else if scheduled {
			persistNeeded = s.appendWatchdogLogLocked("info", "restart_scheduled", serviceName, fmt.Sprintf("%s restart scheduled after failures (%s).", serviceLabel, restartReason), now) || persistNeeded
		} else {
			persistNeeded = s.appendWatchdogLogLocked("info", "restart_ok", serviceName, fmt.Sprintf("%s restart applied after failures (%s).", serviceLabel, restartReason), now) || persistNeeded
		}
		s.mu.Unlock()
	}

	if persistNeeded {
		s.enqueueStatePersist()
	}
	return time.Duration(cfg.IntervalSeconds) * time.Second
}

func (s *service) appendWatchdogLogLocked(level, event, serviceName, message string, now time.Time) bool {
	entry := WatchdogLogEntry{
		ID:        fmt.Sprintf("%d-%s", now.UnixNano(), strings.ReplaceAll(serviceName, " ", "-")),
		Timestamp: now.UTC().Format(time.RFC3339),
		Service:   serviceName,
		Level:     strings.ToLower(strings.TrimSpace(level)),
		Event:     strings.ToLower(strings.TrimSpace(event)),
		Message:   strings.TrimSpace(message),
	}
	if entry.Level == "" {
		entry.Level = "info"
	}
	if entry.Event == "" {
		entry.Event = "log"
	}
	if entry.Message == "" {
		entry.Message = "Watchdog event."
	}

	s.modules.WatchdogLogs = append(s.modules.WatchdogLogs, entry)
	limit := sanitizeWatchdogConfig(s.modules.WatchdogConfig).MaxLogEntries
	if len(s.modules.WatchdogLogs) > limit {
		s.modules.WatchdogLogs = append([]WatchdogLogEntry{}, s.modules.WatchdogLogs[len(s.modules.WatchdogLogs)-limit:]...)
	}
	return true
}

func watchdogServiceUnits(name string) []string {
	switch normalizeWatchdogServiceName(name) {
	case "api-gateway":
		return []string{"aurapanel-api"}
	case "panel-service":
		return []string{"aurapanel-service"}
	case "openlitespeed":
		return []string{"lshttpd", "openlitespeed", "lsws"}
	case "mariadb":
		return []string{"mariadb"}
	case "postgresql":
		return []string{"postgresql"}
	case "redis":
		return []string{"redis-server", "redis"}
	case "docker":
		return []string{"docker"}
	case "fail2ban":
		return []string{"fail2ban"}
	case "pdns":
		return []string{"pdns"}
	case "postfix":
		return []string{"postfix"}
	case "dovecot":
		return []string{"dovecot"}
	case "pure-ftpd":
		return []string{"pure-ftpd"}
	case "minio":
		return []string{"minio"}
	default:
		return nil
	}
}

func watchdogProbeService(name string, gatewayPort int) watchdogProbeResult {
	normalized := normalizeWatchdogServiceName(name)
	if normalized == "" {
		return watchdogProbeResult{Status: "unknown", Message: "Unsupported service target.", Actionable: false}
	}

	units := watchdogServiceUnits(normalized)
	if len(units) == 0 {
		return watchdogProbeResult{Status: "unknown", Message: "No service unit mapping found.", Actionable: false}
	}

	activeState, loaded := detectSystemdStatus(units...)
	if !loaded {
		return watchdogProbeResult{Status: "unknown", Message: "Service unit not installed, check skipped.", Actionable: false}
	}
	if !strings.EqualFold(activeState, "active") {
		return watchdogProbeResult{Status: "unhealthy", Message: fmt.Sprintf("systemd state is %s", strings.TrimSpace(activeState)), Actionable: true}
	}

	switch normalized {
	case "api-gateway":
		if !watchdogHTTPHealthy(fmt.Sprintf("http://127.0.0.1:%d/api/health", gatewayPort), 4*time.Second) {
			return watchdogProbeResult{Status: "unhealthy", Message: "gateway health endpoint did not return 2xx", Actionable: true}
		}
	case "panel-service":
		if !watchdogHTTPHealthy("http://127.0.0.1:8081/api/v1/health", 4*time.Second) {
			return watchdogProbeResult{Status: "unhealthy", Message: "panel-service health endpoint did not return 2xx", Actionable: true}
		}
	}
	return watchdogProbeResult{Status: "healthy", Message: "service is healthy", Actionable: false}
}

func watchdogHTTPHealthy(url string, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func (s *service) watchdogResponseDataLocked() map[string]interface{} {
	cfg := sanitizeWatchdogConfig(s.modules.WatchdogConfig)
	descMap := watchdogTargetDescriptions()

	ordered := make([]watchdogServiceView, 0, len(cfg.Services))
	seen := map[string]struct{}{}
	for _, name := range cfg.Services {
		seen[name] = struct{}{}
		state := s.modules.WatchdogStatus[name]
		ordered = append(ordered, watchdogServiceView{
			Name:                name,
			Desc:                descMap[name],
			LastStatus:          firstNonEmpty(strings.TrimSpace(state.LastStatus), "unknown"),
			LastError:           state.LastError,
			ConsecutiveFailures: state.ConsecutiveFailures,
			LastCheckAt:         state.LastCheckAt,
			LastSuccessAt:       state.LastSuccessAt,
			LastFailureAt:       state.LastFailureAt,
			LastActionAt:        state.LastActionAt,
		})
	}

	extras := make([]string, 0)
	for name := range s.modules.WatchdogStatus {
		if _, ok := seen[name]; ok {
			continue
		}
		extras = append(extras, name)
	}
	sort.Strings(extras)
	for _, name := range extras {
		state := s.modules.WatchdogStatus[name]
		ordered = append(ordered, watchdogServiceView{
			Name:                name,
			Desc:                descMap[name],
			LastStatus:          firstNonEmpty(strings.TrimSpace(state.LastStatus), "unknown"),
			LastError:           state.LastError,
			ConsecutiveFailures: state.ConsecutiveFailures,
			LastCheckAt:         state.LastCheckAt,
			LastSuccessAt:       state.LastSuccessAt,
			LastFailureAt:       state.LastFailureAt,
			LastActionAt:        state.LastActionAt,
		})
	}

	logs := make([]WatchdogLogEntry, 0, len(s.modules.WatchdogLogs))
	for i := len(s.modules.WatchdogLogs) - 1; i >= 0; i-- {
		logs = append(logs, s.modules.WatchdogLogs[i])
	}

	unhealthy := 0
	for _, item := range ordered {
		if item.LastStatus == "unhealthy" {
			unhealthy++
		}
	}

	return map[string]interface{}{
		"enabled":            cfg.Enabled,
		"config":             cfg,
		"supported_services": watchdogTargetsCatalog(),
		"status":             ordered,
		"logs":               logs,
		"summary": map[string]interface{}{
			"service_count":   len(ordered),
			"unhealthy_count": unhealthy,
			"log_count":       len(logs),
		},
	}
}

func (s *service) handleWatchdogGet(w http.ResponseWriter) {
	s.mu.RLock()
	data := s.watchdogResponseDataLocked()
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: data})
}

func (s *service) handleWatchdogToggle(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Enabled *bool `json:"enabled"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid watchdog toggle payload.")
		return
	}
	if payload.Enabled == nil {
		writeError(w, http.StatusBadRequest, "enabled field is required.")
		return
	}

	now := time.Now().UTC()
	s.mu.Lock()
	current := sanitizeWatchdogConfig(s.modules.WatchdogConfig)
	next := current
	next.Enabled = *payload.Enabled
	next = sanitizeWatchdogConfig(next)
	changed := !watchdogConfigEqual(current, next)
	if changed {
		s.modules.WatchdogConfig = next
		mode := "disabled"
		if next.Enabled {
			mode = "enabled"
		}
		s.appendWatchdogLogLocked("info", "watchdog_toggle", "watchdog", fmt.Sprintf("Watchdog %s by panel action.", mode), now)
	}
	data := s.watchdogResponseDataLocked()
	s.mu.Unlock()

	if changed {
		s.enqueueStatePersist()
		go s.runWatchdogTick(time.Now().UTC())
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Watchdog mode updated.", Data: data})
}

func (s *service) handleWatchdogConfigSet(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Enabled          *bool     `json:"enabled"`
		IntervalSeconds  *int      `json:"interval_seconds"`
		FailureThreshold *int      `json:"failure_threshold"`
		CooldownSeconds  *int      `json:"cooldown_seconds"`
		MaxLogEntries    *int      `json:"max_log_entries"`
		Services         *[]string `json:"services"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid watchdog config payload.")
		return
	}

	now := time.Now().UTC()
	s.mu.Lock()
	current := sanitizeWatchdogConfig(s.modules.WatchdogConfig)
	next := current
	if payload.Enabled != nil {
		next.Enabled = *payload.Enabled
	}
	if payload.IntervalSeconds != nil {
		next.IntervalSeconds = *payload.IntervalSeconds
	}
	if payload.FailureThreshold != nil {
		next.FailureThreshold = *payload.FailureThreshold
	}
	if payload.CooldownSeconds != nil {
		next.CooldownSeconds = *payload.CooldownSeconds
	}
	if payload.MaxLogEntries != nil {
		next.MaxLogEntries = *payload.MaxLogEntries
	}
	if payload.Services != nil {
		next.Services = append([]string{}, (*payload.Services)...)
	}
	next = sanitizeWatchdogConfig(next)
	changed := !watchdogConfigEqual(current, next)
	if changed {
		s.modules.WatchdogConfig = next
		s.appendWatchdogLogLocked("info", "watchdog_config", "watchdog", "Watchdog configuration updated.", now)
	}
	data := s.watchdogResponseDataLocked()
	s.mu.Unlock()

	if changed {
		s.enqueueStatePersist()
		go s.runWatchdogTick(time.Now().UTC())
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Watchdog configuration saved.", Data: data})
}

func (s *service) handleWatchdogLogsClear(w http.ResponseWriter) {
	now := time.Now().UTC()
	s.mu.Lock()
	s.modules.WatchdogLogs = []WatchdogLogEntry{}
	s.appendWatchdogLogLocked("info", "logs_cleared", "watchdog", "Watchdog logs cleared.", now)
	data := s.watchdogResponseDataLocked()
	s.mu.Unlock()

	s.enqueueStatePersist()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Watchdog logs cleared.", Data: data})
}
