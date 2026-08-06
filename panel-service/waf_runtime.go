package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

type wafInspectionInput struct {
	Action    string `json:"action"`
	Enabled   *bool  `json:"enabled"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Query     string `json:"query"`
	Body      string `json:"body"`
	UserAgent string `json:"user_agent"`
	IP        string `json:"ip"`
}

type wafInspectionResult struct {
	Allowed bool   `json:"allowed"`
	Score   int    `json:"score"`
	Reason  string `json:"reason"`
}

func modSecurityBlockToken(content string) (string, error) {
	candidates := []string{
		"module mod_security {",
		"module mod_security{",
	}
	for _, token := range candidates {
		if strings.Contains(content, token) {
			return token, nil
		}
	}
	return "", fmt.Errorf("ModSecurity block not found in OpenLiteSpeed config")
}

func parseBoolDirectiveValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "on", "true", "yes", "enabled":
		return true
	default:
		return false
	}
}

func readOLSDirectiveValue(block, key string) (string, bool) {
	for _, line := range strings.Split(block, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == key {
			return fields[1], true
		}
	}
	return "", false
}

func modSecurityEnabledFromContent(content string) (bool, error) {
	token, err := modSecurityBlockToken(content)
	if err != nil {
		return false, err
	}
	block, err := extractOLSConfigBlock(content, token)
	if err != nil {
		return false, err
	}
	modValue, modFound := readOLSDirectiveValue(block, "modsecurity")
	lsValue, lsFound := readOLSDirectiveValue(block, "ls_enabled")
	return modFound && lsFound && parseBoolDirectiveValue(modValue) && parseBoolDirectiveValue(lsValue), nil
}

func readModSecurityState() (bool, error) {
	if !fileExists(olsHTTPDConfigPath) {
		return false, fmt.Errorf("OpenLiteSpeed config not found")
	}
	raw, err := os.ReadFile(olsHTTPDConfigPath)
	if err != nil {
		return false, err
	}
	return modSecurityEnabledFromContent(string(raw))
}

func setModSecurityState(enabled bool) error {
	if !fileExists(olsHTTPDConfigPath) {
		return fmt.Errorf("OpenLiteSpeed config not found")
	}
	previous, err := os.ReadFile(olsHTTPDConfigPath)
	if err != nil {
		return err
	}
	content := string(previous)
	token, err := modSecurityBlockToken(content)
	if err != nil {
		return err
	}
	modsecurityValue := "off"
	lsEnabledValue := "0"
	if enabled {
		modsecurityValue = "on"
		lsEnabledValue = "1"
	}
	updated, err := replaceOLSBlockDirectives(content, token, map[string]string{
		"modsecurity": modsecurityValue,
		"ls_enabled":  lsEnabledValue,
	})
	if err != nil {
		return err
	}
	if err := writeOLSFileAtomically(olsHTTPDConfigPath, []byte(updated), 0o640); err != nil {
		return err
	}
	if err := ensureOLSHTTPDConfigOwnership(); err != nil {
		return err
	}
	if err := reloadOpenLiteSpeed(); err != nil {
		_ = writeOLSFileAtomically(olsHTTPDConfigPath, previous, 0o640)
		_ = ensureOLSHTTPDConfigOwnership()
		_ = reloadOpenLiteSpeed()
		return err
	}
	return nil
}

func detectModSecurityActive() bool {
	if !fileExists("/usr/local/lsws/modules/mod_security.so") || !fileExists(olsHTTPDConfigPath) {
		return false
	}
	enabled, err := readModSecurityState()
	if err != nil {
		return false
	}
	return enabled
}

func inspectWAFPayload(input wafInspectionInput) wafInspectionResult {
	normalized := strings.ToLower(strings.Join([]string{
		strings.TrimSpace(input.Path),
		strings.TrimSpace(input.Query),
		strings.TrimSpace(input.Body),
		strings.TrimSpace(input.UserAgent),
	}, "\n"))

	signatures := []struct {
		Name     string
		Patterns []string
		Score    int
	}{
		{Name: "XSS payload", Patterns: []string{"<script", "javascript:", "onerror=", "onload=", "%3cscript"}, Score: 95},
		{Name: "SQL injection payload", Patterns: []string{" union select ", "' or 1=1", "\" or 1=1", "sleep(", "benchmark(", "information_schema"}, Score: 98},
		{Name: "LFI payload", Patterns: []string{"../", "..%2f", "/etc/passwd", "boot.ini"}, Score: 90},
		{Name: "RCE payload", Patterns: []string{";wget ", ";curl ", "$(", "`id`", "|bash", "cmd.exe"}, Score: 99},
		{Name: "Scanner signature", Patterns: []string{"sqlmap", "nikto", "acunetix", "nmap"}, Score: 85},
	}

	for _, signature := range signatures {
		for _, pattern := range signature.Patterns {
			if strings.Contains(normalized, pattern) {
				reason := fmt.Sprintf("%s tespit edildi (%s).", signature.Name, pattern)
				if detectModSecurityActive() {
					reason = "ModSecurity/OWASP CRS aktif: " + reason
				} else {
					reason = "ModSecurity pasif ama istek riskli: " + reason
				}
				return wafInspectionResult{
					Allowed: false,
					Score:   signature.Score,
					Reason:  reason,
				}
			}
		}
	}

	reason := "Istek, AuraPanel WAF heuristiklerine gore temiz."
	if detectModSecurityActive() {
		reason = "ModSecurity/OWASP CRS aktif ve istek temel imzalara takilmadi."
	}
	return wafInspectionResult{
		Allowed: true,
		Score:   5,
		Reason:  reason,
	}
}

func (s *service) handleSecurityWAF(w http.ResponseWriter, r *http.Request) {
	var payload wafInspectionInput
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid WAF payload.")
		return
	}

	action := strings.ToLower(strings.TrimSpace(payload.Action))
	if payload.Enabled != nil {
		if *payload.Enabled {
			action = "enable"
		} else {
			action = "disable"
		}
	}

	switch action {
	case "", "inspect", "analyze", "test":
		result := inspectWAFPayload(payload)
		writeJSON(w, http.StatusOK, apiResponse{
			Status:  "success",
			Allowed: result.Allowed,
			Score:   result.Score,
			Reason:  result.Reason,
			Data:    result,
		})
		return
	case "status":
		enabled, err := readModSecurityState()
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Unable to read WAF state: %v", err))
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Status:  "success",
			Message: "WAF status loaded successfully.",
			Data: map[string]interface{}{
				"enabled": enabled,
			},
		})
		return
	case "enable", "disable":
		if !fileExists("/usr/local/lsws/modules/mod_security.so") {
			writeError(w, http.StatusBadRequest, "ModSecurity module is not installed on this server.")
			return
		}
		targetEnabled := action == "enable"
		if err := setModSecurityState(targetEnabled); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update WAF state: %v", err))
			return
		}
		currentState, err := readModSecurityState()
		if err != nil {
			currentState = targetEnabled
		}
		message := "WAF disabled successfully."
		if currentState {
			message = "WAF enabled successfully."
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Status:  "success",
			Message: message,
			Data: map[string]interface{}{
				"enabled": currentState,
			},
		})
		return
	default:
		writeError(w, http.StatusBadRequest, "Invalid WAF action. Supported actions: inspect, status, enable, disable.")
		return
	}
}
