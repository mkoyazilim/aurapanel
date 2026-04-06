package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	webStackModeOLSOnly   = "ols-only"
	webStackModeNginxEdge = "nginx-edge"
)

type webStackApplyResult struct {
	PreviousMode    string   `json:"previous_mode"`
	CurrentMode     string   `json:"current_mode"`
	Applied         bool     `json:"applied"`
	NginxActive     bool     `json:"nginx_active"`
	OpenLiteSpeedUp bool     `json:"openlitespeed_active"`
	ScriptPath      string   `json:"script_path,omitempty"`
	ExecutionOutput string   `json:"execution_output,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
}

func normalizeWebStackMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case webStackModeNginxEdge, "nginx_edge", "nginxedge":
		return webStackModeNginxEdge
	case webStackModeOLSOnly, "ols_only", "olsonly", "":
		return webStackModeOLSOnly
	default:
		return ""
	}
}

func currentWebStackMode() string {
	value := strings.TrimSpace(os.Getenv("AURAPANEL_WEB_STACK_MODE"))
	if value == "" {
		value = strings.TrimSpace(readEnvFileValue(adminServiceEnvPath(), "AURAPANEL_WEB_STACK_MODE"))
	}
	normalized := normalizeWebStackMode(value)
	if normalized == "" {
		return webStackModeOLSOnly
	}
	return normalized
}

func webStackScriptPath() (string, error) {
	candidates := []string{}
	if explicit := strings.TrimSpace(os.Getenv("AURAPANEL_WEB_STACK_SCRIPT")); explicit != "" {
		candidates = append(candidates, explicit)
	}
	candidates = append(candidates,
		filepath.Join(panelRepoPath(), "installer", "web-stack-mode.sh"),
		filepath.Join(filepath.Dir(panelRepoPath()), "installer", "web-stack-mode.sh"),
		filepath.Join("..", "installer", "web-stack-mode.sh"),
	)
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("web stack script not found")
}

func detectWebStackStatus() map[string]interface{} {
	mode := currentWebStackMode()
	return map[string]interface{}{
		"mode":                 mode,
		"supported_modes":      []string{webStackModeOLSOnly, webStackModeNginxEdge},
		"nginx_active":         serviceActive("nginx"),
		"openlitespeed_active": serviceActive("lshttpd", "openlitespeed", "lsws"),
	}
}

func applyWebStackMode(targetMode string) (webStackApplyResult, error) {
	targetMode = normalizeWebStackMode(targetMode)
	if targetMode == "" {
		return webStackApplyResult{}, fmt.Errorf("invalid web stack mode")
	}

	result := webStackApplyResult{
		PreviousMode: currentWebStackMode(),
		CurrentMode:  currentWebStackMode(),
		Warnings:     []string{},
	}
	if targetMode == result.PreviousMode {
		result.NginxActive = serviceActive("nginx")
		result.OpenLiteSpeedUp = serviceActive("lshttpd", "openlitespeed", "lsws")
		return result, nil
	}

	scriptPath, err := webStackScriptPath()
	if err != nil {
		return result, err
	}
	result.ScriptPath = scriptPath

	output, err := runCommandCombinedOutputWithTimeout(
		20*time.Minute,
		"bash",
		scriptPath,
		"--mode", targetMode,
		"--apply",
		"--auto-install-nginx",
	)
	result.ExecutionOutput = strings.TrimSpace(string(output))
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return result, fmt.Errorf("web stack apply failed: %s", message)
	}

	_ = os.Setenv("AURAPANEL_WEB_STACK_MODE", targetMode)
	if targetMode == webStackModeNginxEdge {
		_ = os.Setenv("AURAPANEL_NGINX_EDGE_ENABLED", "1")
		_ = os.Setenv("AURAPANEL_OLS_BACKEND_ADDR", "127.0.0.1:8088")
	} else {
		_ = os.Setenv("AURAPANEL_NGINX_EDGE_ENABLED", "0")
		_ = os.Setenv("AURAPANEL_OLS_BACKEND_ADDR", "*:80")
	}

	result.CurrentMode = currentWebStackMode()
	result.Applied = true
	result.NginxActive = serviceActive("nginx")
	result.OpenLiteSpeedUp = serviceActive("lshttpd", "openlitespeed", "lsws")
	if result.CurrentMode != targetMode {
		result.Warnings = append(result.Warnings, "Requested mode differs from runtime env mode after apply.")
	}
	return result, nil
}

func (s *service) handleWebStackGet(w http.ResponseWriter) {
	data := detectWebStackStatus()
	if scriptPath, err := webStackScriptPath(); err == nil {
		data["script_path"] = scriptPath
	} else {
		data["script_path"] = ""
		data["warning"] = err.Error()
	}
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data:   data,
	})
}

func (s *service) handleWebStackSet(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Mode string `json:"mode"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid web stack payload.")
		return
	}
	targetMode := normalizeWebStackMode(payload.Mode)
	if targetMode == "" {
		writeError(w, http.StatusBadRequest, "Web stack mode must be 'ols-only' or 'nginx-edge'.")
		return
	}

	result, err := applyWebStackMode(targetMode)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: "Web stack mode updated.",
		Data:    result,
	})
}
