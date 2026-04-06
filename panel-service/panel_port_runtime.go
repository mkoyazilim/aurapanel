package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultPanelEdgeVhostConfigPath = "/usr/local/lsws/conf/vhosts/Example/vhconf.conf"
	panelEdgeExtprocBeginMarker     = "# AURAPANEL PANEL EDGE EXTPROC BEGIN"
	panelEdgeExtprocEndMarker       = "# AURAPANEL PANEL EDGE EXTPROC END"
)

type panelPortChangeResult struct {
	FirewallActions  []string
	Warnings         []string
	RestartScheduled bool
	RestartApplied   bool
	EdgeSynced       bool
}

type panelEdgeConfig struct {
	Enabled         bool
	Domain          string
	VhostConfigPath string
}

type panelEdgeConfigApplyResult struct {
	Warnings   []string
	EdgeSynced bool
}

type fileRollbackBackup struct {
	Path    string
	Exists  bool
	Perm    os.FileMode
	Content []byte
}

func applyPanelPortChange(port int, openFirewall bool) (panelPortChangeResult, error) {
	result := panelPortChangeResult{
		FirewallActions:  []string{},
		Warnings:         []string{},
		RestartScheduled: false,
		RestartApplied:   false,
		EdgeSynced:       false,
	}

	if openFirewall {
		if err := openFirewallPort(port); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Firewall update failed for tcp/%d: %v", port, err))
		} else {
			result.FirewallActions = append(result.FirewallActions, fmt.Sprintf("Allow tcp/%d on firewall", port))
		}
	}

	gatewayEnvPath := adminGatewayEnvPath()
	gatewayBackup, err := backupFileForRollback(gatewayEnvPath)
	if err != nil {
		return result, fmt.Errorf("gateway env backup failed: %w", err)
	}

	edgeEnabled := panelEdgeSingleDomainEnabled()
	edgePath := panelEdgeVhostConfigPath()
	edgeBackup := fileRollbackBackup{}
	if edgeEnabled {
		edgeBackup, err = backupFileForRollback(edgePath)
		if err != nil {
			return result, fmt.Errorf("panel edge config backup failed: %w", err)
		}
	}

	if err := writeEnvFileValues(gatewayEnvPath, map[string]string{
		"AURAPANEL_GATEWAY_ADDR": fmt.Sprintf(":%d", port),
	}); err != nil {
		return result, fmt.Errorf("gateway env update failed: %w", err)
	}

	rollback := func(reason error) error {
		_ = restoreFileFromRollback(gatewayBackup)
		if edgeEnabled {
			_ = restoreFileFromRollback(edgeBackup)
			_ = reloadOpenLiteSpeed()
		}
		return reason
	}

	if edgeEnabled {
		if err := withOLSConfigLock(func() error {
			if err := updatePanelEdgeGatewayUpstream(edgePath, port); err != nil {
				return fmt.Errorf("panel edge sync failed: %w", err)
			}
			if err := reloadOpenLiteSpeed(); err != nil {
				return fmt.Errorf("openlitespeed reload failed after panel edge sync: %w", err)
			}
			return nil
		}); err != nil {
			return result, rollback(err)
		}
		result.EdgeSynced = true
	}

	restarted, warning, err := restartGatewayForPanelPortChange(port)
	if err != nil {
		return result, rollback(err)
	}
	result.RestartApplied = restarted
	if warning != "" {
		result.RestartScheduled = true
		result.Warnings = append(result.Warnings, warning)
	}

	return result, nil
}

func panelEdgeSingleDomainEnabled() bool {
	value := strings.TrimSpace(os.Getenv("AURAPANEL_PANEL_EDGE_SINGLE_DOMAIN"))
	if value == "" {
		value = strings.TrimSpace(readEnvFileValue(adminServiceEnvPath(), "AURAPANEL_PANEL_EDGE_SINGLE_DOMAIN"))
	}
	return envBoolEnabled(value)
}

func panelEdgeDomain() string {
	value := strings.TrimSpace(os.Getenv("AURAPANEL_PANEL_EDGE_DOMAIN"))
	if value == "" {
		value = strings.TrimSpace(readEnvFileValue(adminServiceEnvPath(), "AURAPANEL_PANEL_EDGE_DOMAIN"))
	}
	return normalizeDomain(value)
}

func envBoolEnabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func panelEdgeVhostConfigPath() string {
	value := strings.TrimSpace(os.Getenv("AURAPANEL_PANEL_EDGE_VHOST_CONF"))
	if value == "" {
		value = strings.TrimSpace(readEnvFileValue(adminServiceEnvPath(), "AURAPANEL_PANEL_EDGE_VHOST_CONF"))
	}
	return firstNonEmpty(value, defaultPanelEdgeVhostConfigPath)
}

func loadPanelEdgeConfig() panelEdgeConfig {
	return panelEdgeConfig{
		Enabled:         panelEdgeSingleDomainEnabled(),
		Domain:          panelEdgeDomain(),
		VhostConfigPath: panelEdgeVhostConfigPath(),
	}
}

func boolToEnvValue(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func applyPanelEdgeConfigChange(config panelEdgeConfig, gatewayPort int) (panelEdgeConfigApplyResult, error) {
	result := panelEdgeConfigApplyResult{
		Warnings:   []string{},
		EdgeSynced: false,
	}
	config.Domain = normalizeDomain(config.Domain)
	config.VhostConfigPath = firstNonEmpty(strings.TrimSpace(config.VhostConfigPath), defaultPanelEdgeVhostConfigPath)

	serviceEnvPath := adminServiceEnvPath()
	serviceEnvBackup, err := backupFileForRollback(serviceEnvPath)
	if err != nil {
		return result, fmt.Errorf("service env backup failed: %w", err)
	}

	edgeBackup := fileRollbackBackup{}
	if config.Enabled {
		edgeBackup, err = backupFileForRollback(config.VhostConfigPath)
		if err != nil {
			return result, fmt.Errorf("panel edge config backup failed: %w", err)
		}
	}

	if err := writeEnvFileValues(serviceEnvPath, map[string]string{
		"AURAPANEL_PANEL_EDGE_SINGLE_DOMAIN": boolToEnvValue(config.Enabled),
		"AURAPANEL_PANEL_EDGE_DOMAIN":        config.Domain,
		"AURAPANEL_PANEL_EDGE_VHOST_CONF":    config.VhostConfigPath,
	}); err != nil {
		return result, fmt.Errorf("service env update failed: %w", err)
	}

	rollback := func(reason error) error {
		_ = restoreFileFromRollback(serviceEnvBackup)
		if config.Enabled {
			_ = restoreFileFromRollback(edgeBackup)
			_ = reloadOpenLiteSpeed()
		}
		return reason
	}

	if config.Enabled {
		if err := withOLSConfigLock(func() error {
			if err := updatePanelEdgeGatewayUpstream(config.VhostConfigPath, gatewayPort); err != nil {
				return fmt.Errorf("panel edge sync failed: %w", err)
			}
			if err := reloadOpenLiteSpeed(); err != nil {
				return fmt.Errorf("openlitespeed reload failed after panel edge sync: %w", err)
			}
			return nil
		}); err != nil {
			return result, rollback(err)
		}
		result.EdgeSynced = true
	}

	_ = os.Setenv("AURAPANEL_PANEL_EDGE_SINGLE_DOMAIN", boolToEnvValue(config.Enabled))
	_ = os.Setenv("AURAPANEL_PANEL_EDGE_DOMAIN", config.Domain)
	_ = os.Setenv("AURAPANEL_PANEL_EDGE_VHOST_CONF", config.VhostConfigPath)

	return result, nil
}

func updatePanelEdgeGatewayUpstream(path string, port int) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("panel edge config not found: %s", path)
		}
		return err
	}
	rendered, err := replacePanelEdgeExtProcessorBlock(string(raw), fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	return writeOLSFileAtomically(path, []byte(rendered), 0o640)
}

func replacePanelEdgeExtProcessorBlock(content, upstreamAddr string) (string, error) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	out := make([]string, 0, len(lines)+10)
	beginFound := false
	endFound := false
	inManagedBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == panelEdgeExtprocBeginMarker {
			if inManagedBlock {
				return "", fmt.Errorf("panel edge config contains invalid nested markers")
			}
			beginFound = true
			inManagedBlock = true
			out = append(out, renderPanelEdgeExtProcessorLines(upstreamAddr)...)
			continue
		}
		if inManagedBlock {
			if trimmed == panelEdgeExtprocEndMarker {
				endFound = true
				inManagedBlock = false
			}
			continue
		}
		out = append(out, line)
	}

	if inManagedBlock || !beginFound || !endFound {
		return "", fmt.Errorf("panel edge managed extprocessor block not found")
	}

	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n", nil
}

func renderPanelEdgeExtProcessorLines(upstreamAddr string) []string {
	return []string{
		panelEdgeExtprocBeginMarker,
		"extprocessor aurapanel_gateway {",
		"  type                    proxy",
		"  address                 " + strings.TrimSpace(upstreamAddr),
		"  maxConns                1000",
		"  initTimeout             60",
		"  retryTimeout            0",
		"  respBuffer              0",
		"}",
		panelEdgeExtprocEndMarker,
	}
}

func backupFileForRollback(path string) (fileRollbackBackup, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fileRollbackBackup{Path: path, Exists: false, Perm: 0o600}, nil
		}
		return fileRollbackBackup{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fileRollbackBackup{}, err
	}
	return fileRollbackBackup{
		Path:    path,
		Exists:  true,
		Perm:    info.Mode().Perm(),
		Content: raw,
	}, nil
}

func restoreFileFromRollback(backup fileRollbackBackup) error {
	if !backup.Exists {
		if err := os.Remove(backup.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(backup.Path), 0o755); err != nil {
		return err
	}
	perm := backup.Perm
	if perm == 0 {
		perm = 0o600
	}
	return os.WriteFile(backup.Path, backup.Content, perm)
}

func restartGatewayForPanelPortChange(port int) (bool, string, error) {
	if runtime.GOOS != "linux" {
		return false, "Gateway restart is not automatic on non-linux hosts; restart the gateway manually.", nil
	}
	if !systemctlUnitExists("aurapanel-api.service") {
		return false, "Gateway unit not found; restart aurapanel-api manually.", nil
	}
	if err := executeServiceAction("api-gateway", "restart"); err != nil {
		return false, "", fmt.Errorf("gateway restart failed: %w", err)
	}
	if err := waitForGatewayHealthOnPort(port, 25*time.Second); err != nil {
		return false, "", fmt.Errorf("gateway did not become healthy on port %d: %w", port, err)
	}
	return true, "", nil
}

func waitForGatewayHealthOnPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/api/health", port)
	client := &http.Client{Timeout: 2 * time.Second}

	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("status code %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timeout")
	}
	return lastErr
}
