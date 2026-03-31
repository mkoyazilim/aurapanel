package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPanelRepoPath     = "/opt/aurapanel"
	defaultPanelDeployRemote = "origin"
	defaultPanelDeployBranch = "main"
)

type panelUpdateResult struct {
	PreviousVersion string
	CurrentVersion  string
	TargetVersion   string
	Steps           []string
	Warnings        []string
}

func panelRepoPath() string {
	return firstNonEmpty(strings.TrimSpace(os.Getenv("AURAPANEL_REPO_PATH")), defaultPanelRepoPath)
}

func panelDeployRemote() string {
	return firstNonEmpty(strings.TrimSpace(os.Getenv("AURAPANEL_DEPLOY_REMOTE")), defaultPanelDeployRemote)
}

func panelDeployBranch() string {
	return firstNonEmpty(strings.TrimSpace(os.Getenv("AURAPANEL_DEPLOY_BRANCH")), defaultPanelDeployBranch)
}

func panelDeployRef() string {
	return fmt.Sprintf("%s/%s", panelDeployRemote(), panelDeployBranch())
}

func panelDeployScriptPath(repoPath string) string {
	return filepath.Join(repoPath, "scripts", "deploy-main.sh")
}

func resolveCurrentPanelVersion() string {
	return currentPanelVersionFromRepo(panelRepoPath())
}

func currentPanelVersionFromRepo(repoPath string) string {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		repoPath = defaultPanelRepoPath
	}
	if value := strings.TrimSpace(os.Getenv("AURAPANEL_CURRENT_VERSION")); value != "" {
		return value
	}
	if !fileExists(filepath.Join(repoPath, ".git")) {
		return currentPanelVersion
	}
	if out, err := commandOutputTrimmed("git", "-C", repoPath, "describe", "--tags", "--always", "--dirty"); err == nil && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out)
	}
	if out, err := commandOutputTrimmed("git", "-C", repoPath, "rev-parse", "--short", "HEAD"); err == nil && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out)
	}
	return currentPanelVersion
}

func shortCommitSHA(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > 8 {
		return trimmed[:8]
	}
	return trimmed
}

func parseGitAheadBehind(raw string) (int, int, error) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("invalid ahead/behind payload: %q", raw)
	}
	ahead, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid ahead value: %w", err)
	}
	behind, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid behind value: %w", err)
	}
	return ahead, behind, nil
}

func gitAheadBehind(repoPath, remoteRef string) (int, int, error) {
	out, err := commandOutputTrimmed("git", "-C", repoPath, "rev-list", "--left-right", "--count", "HEAD..."+remoteRef)
	if err != nil {
		return 0, 0, err
	}
	return parseGitAheadBehind(out)
}

func normalizeGitRemoteURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "git@github.com:") {
		value = "https://github.com/" + strings.TrimPrefix(value, "git@github.com:")
	}
	if strings.HasPrefix(value, "http://github.com/") {
		value = "https://github.com/" + strings.TrimPrefix(value, "http://github.com/")
	}
	value = strings.TrimSuffix(value, ".git")
	if strings.HasPrefix(value, "https://github.com/") {
		return value
	}
	return ""
}

func fetchGitDeployUpdateStatus() UpdateStatus {
	repoPath := panelRepoPath()
	remote := panelDeployRemote()
	branch := panelDeployBranch()
	remoteRef := panelDeployRef()
	status := UpdateStatus{
		CurrentVersion: resolveCurrentPanelVersion(),
		Source:         fmt.Sprintf("Git %s/%s", remote, branch),
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	if !fileExists(filepath.Join(repoPath, ".git")) {
		status.Error = fmt.Sprintf("repository path is not a git checkout: %s", repoPath)
		return status
	}

	if _, err := commandOutputTrimmed("git", "-C", repoPath, "fetch", remote, branch); err != nil {
		status.Error = fmt.Sprintf("git fetch failed for %s: %v", remoteRef, err)
		return status
	}

	localHead, err := commandOutputTrimmed("git", "-C", repoPath, "rev-parse", "HEAD")
	if err != nil {
		status.Error = fmt.Sprintf("local commit check failed: %v", err)
		return status
	}
	remoteHead, err := commandOutputTrimmed("git", "-C", repoPath, "rev-parse", remoteRef)
	if err != nil {
		status.Error = fmt.Sprintf("remote commit check failed: %v", err)
		return status
	}
	ahead, behind, err := gitAheadBehind(repoPath, remoteRef)
	if err != nil {
		status.Error = fmt.Sprintf("git divergence check failed: %v", err)
		return status
	}

	status.UpdateAvailable = behind > 0
	status.LatestTag = shortCommitSHA(remoteHead)
	status.LatestVersion = fmt.Sprintf("%s/%s @ %s", remote, branch, status.LatestTag)
	status.ReleaseName = status.LatestVersion
	status.ReleaseNotes = fmt.Sprintf("sync state: ahead=%d, behind=%d", ahead, behind)
	if !status.UpdateAvailable {
		status.ReleaseNotes = fmt.Sprintf("sync state: up-to-date (%s)", shortCommitSHA(localHead))
	}

	if remoteURL, urlErr := commandOutputTrimmed("git", "-C", repoPath, "remote", "get-url", remote); urlErr == nil {
		normalized := normalizeGitRemoteURL(remoteURL)
		if normalized != "" {
			status.ReleaseURL = normalized + "/tree/" + branch
		}
	}

	return status
}

func applyPanelUpdateFromDeployScript(scheduleRestart bool) (panelUpdateResult, error) {
	result := panelUpdateResult{
		PreviousVersion: resolveCurrentPanelVersion(),
		TargetVersion:   panelDeployRef(),
		Steps:           []string{},
		Warnings:        []string{},
	}

	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("panel update can only be applied on linux hosts")
	}

	repoPath := panelRepoPath()
	if !fileExists(filepath.Join(repoPath, ".git")) {
		return result, fmt.Errorf("repository path is not a git checkout: %s", repoPath)
	}

	scriptPath := panelDeployScriptPath(repoPath)
	if !fileExists(scriptPath) {
		return result, fmt.Errorf("deploy script not found: %s", scriptPath)
	}

	if scheduleRestart {
		if err := runPanelUpdateStep(&result, "Run deploy pipeline (git pull + build)", "env", "AURAPANEL_DEPLOY_SKIP_RESTART=1", "bash", scriptPath); err != nil {
			return result, err
		}
		if err := runPanelUpdateStep(&result, "Schedule API gateway restart (10s)", "systemd-run", "--unit", "aurapanel-api-delayed-restart", "--on-active=10", "systemctl", "restart", "aurapanel-api"); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("API gateway restart could not be scheduled automatically: %v", err))
			result.Warnings = append(result.Warnings, "Restart aurapanel-api manually to apply gateway changes.")
		}
		if err := runPanelUpdateStep(&result, "Schedule panel-service restart (13s)", "systemd-run", "--unit", "aurapanel-service-delayed-restart", "--on-active=13", "systemctl", "restart", "aurapanel-service"); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Panel-service restart could not be scheduled automatically: %v", err))
			result.Warnings = append(result.Warnings, "Restart aurapanel-service manually to apply backend changes.")
		}
		result.Warnings = append(result.Warnings, "Service restarts are scheduled in the background; brief reconnect may occur.")
	} else {
		if err := runPanelUpdateStep(&result, "Run deploy pipeline (git pull + build + restart)", "bash", scriptPath); err != nil {
			return result, err
		}
	}

	result.CurrentVersion = resolveCurrentPanelVersion()
	if strings.TrimSpace(result.CurrentVersion) == strings.TrimSpace(result.PreviousVersion) {
		result.Warnings = append(result.Warnings, "No version change detected after deploy. The server may already be up to date.")
	}
	return result, nil
}

func applyPanelUpdateToRelease(target string) (panelUpdateResult, error) {
	result := panelUpdateResult{
		PreviousVersion: resolveCurrentPanelVersion(),
		TargetVersion:   strings.TrimSpace(target),
		Steps:           []string{},
		Warnings:        []string{},
	}

	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("panel update can only be applied on linux hosts")
	}
	if result.TargetVersion == "" {
		return result, fmt.Errorf("target release version is required")
	}

	repoPath := panelRepoPath()
	if !fileExists(filepath.Join(repoPath, ".git")) {
		return result, fmt.Errorf("repository path is not a git checkout: %s", repoPath)
	}

	if dirty, err := commandOutputTrimmed("git", "-C", repoPath, "status", "--porcelain"); err == nil && strings.TrimSpace(dirty) != "" {
		return result, fmt.Errorf("repository is dirty; commit or stash local changes before panel update")
	}

	if err := runPanelUpdateStep(&result, "Fetch latest tags", "git", "-C", repoPath, "fetch", "--tags", "origin"); err != nil {
		return result, err
	}
	if err := runPanelUpdateStep(&result, "Checkout target release", "git", "-C", repoPath, "checkout", result.TargetVersion); err != nil {
		return result, err
	}

	frontendPath := filepath.Join(repoPath, "frontend")
	if fileExists(filepath.Join(frontendPath, "package-lock.json")) {
		if err := runPanelUpdateStep(&result, "Install frontend dependencies (npm ci)", "npm", "--prefix", frontendPath, "ci"); err != nil {
			if err2 := runPanelUpdateStep(&result, "Fallback dependency install (npm install)", "npm", "--prefix", frontendPath, "install"); err2 != nil {
				return result, err2
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf("npm ci failed and fallback npm install was used: %v", err))
		}
	} else {
		if err := runPanelUpdateStep(&result, "Install frontend dependencies (npm install)", "npm", "--prefix", frontendPath, "install"); err != nil {
			return result, err
		}
	}
	if err := runPanelUpdateStep(&result, "Build frontend", "npm", "--prefix", frontendPath, "run", "build"); err != nil {
		return result, err
	}

	panelServicePath := filepath.Join(repoPath, "panel-service")
	apiGatewayPath := filepath.Join(repoPath, "api-gateway")
	if err := runPanelUpdateStep(&result, "Build panel-service binary", "go", "-C", panelServicePath, "build", "-o", filepath.Join(panelServicePath, "panel-service")); err != nil {
		return result, err
	}
	if err := runPanelUpdateStep(&result, "Build api-gateway binary", "go", "-C", apiGatewayPath, "build", "-o", filepath.Join(apiGatewayPath, "apigw")); err != nil {
		return result, err
	}
	if err := runPanelUpdateStep(&result, "Restart API gateway", "systemctl", "restart", "aurapanel-api"); err != nil {
		return result, err
	}
	if err := runPanelUpdateStep(&result, "Schedule panel-service restart", "systemd-run", "--unit", "aurapanel-service-delayed-restart", "--on-active=3", "systemctl", "restart", "aurapanel-service"); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Panel-service restart could not be scheduled automatically: %v", err))
		result.Warnings = append(result.Warnings, "Restart aurapanel-service manually to apply backend code updates.")
	} else {
		result.Warnings = append(result.Warnings, "Panel-service restart is scheduled to run in a few seconds.")
	}
	result.CurrentVersion = resolveCurrentPanelVersion()
	return result, nil
}

func runPanelUpdateStep(result *panelUpdateResult, title string, command string, args ...string) error {
	result.Steps = append(result.Steps, title)
	output, err := commandOutputTrimmed(command, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", title, err)
	}
	if output != "" {
		result.Steps = append(result.Steps, output)
	}
	return nil
}
