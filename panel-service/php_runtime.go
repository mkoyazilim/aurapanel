package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type phpPackageManager string

const (
	phpPkgManagerAPT phpPackageManager = "apt"
	phpPkgManagerDNF phpPackageManager = "dnf"
)

type phpExtensionSpec struct {
	ID          string
	Name        string
	Description string
	APTSuffixes []string
	DNFSuffixes []string
	Baseline    bool
}

type PHPExtensionInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Package     string `json:"package"`
	Installed   bool   `json:"installed"`
	Available   bool   `json:"available"`
	Baseline    bool   `json:"baseline"`
}

func phpExtensionCatalogSpecs() []phpExtensionSpec {
	return []phpExtensionSpec{
		{
			ID:          "bcmath",
			Name:        "BCMath",
			Description: "High precision mathematics support.",
			APTSuffixes: []string{"bcmath"},
			DNFSuffixes: []string{"bcmath"},
			Baseline:    false,
		},
		{
			ID:          "curl",
			Name:        "cURL",
			Description: "HTTP client integrations and APIs.",
			APTSuffixes: []string{"curl"},
			DNFSuffixes: []string{"curl"},
			Baseline:    true,
		},
		{
			ID:          "gd",
			Name:        "GD",
			Description: "Image processing primitives.",
			APTSuffixes: []string{"gd"},
			DNFSuffixes: []string{"gd"},
			Baseline:    false,
		},
		{
			ID:          "imagick",
			Name:        "Imagick",
			Description: "ImageMagick adapter for advanced media workflows.",
			APTSuffixes: []string{"imagick", "pecl-imagick"},
			DNFSuffixes: []string{"pecl-imagick", "imagick"},
			Baseline:    false,
		},
		{
			ID:          "intl",
			Name:        "Intl",
			Description: "Locale and unicode aware internationalization.",
			APTSuffixes: []string{"intl"},
			DNFSuffixes: []string{"intl"},
			Baseline:    true,
		},
		{
			ID:          "ioncube",
			Name:        "ionCube Loader",
			Description: "Runtime loader for encoded commercial PHP packages.",
			APTSuffixes: []string{"ioncube"},
			DNFSuffixes: []string{"ioncube"},
			Baseline:    false,
		},
		{
			ID:          "mbstring",
			Name:        "MBString",
			Description: "Multibyte string handling.",
			APTSuffixes: []string{"mbstring"},
			DNFSuffixes: []string{"mbstring"},
			Baseline:    true,
		},
		{
			ID:          "mysql",
			Name:        "MySQL",
			Description: "MySQL/MariaDB database driver.",
			APTSuffixes: []string{"mysql", "mysqlnd"},
			DNFSuffixes: []string{"mysqlnd", "mysql"},
			Baseline:    true,
		},
		{
			ID:          "opcache",
			Name:        "OPcache",
			Description: "Opcode cache for reduced CPU usage.",
			APTSuffixes: []string{"opcache"},
			DNFSuffixes: []string{"opcache"},
			Baseline:    true,
		},
		{
			ID:          "pgsql",
			Name:        "PostgreSQL",
			Description: "PostgreSQL database driver.",
			APTSuffixes: []string{"pgsql"},
			DNFSuffixes: []string{"pgsql"},
			Baseline:    true,
		},
		{
			ID:          "redis",
			Name:        "Redis",
			Description: "Redis client extension for cache/session workloads.",
			APTSuffixes: []string{"redis", "pecl-redis"},
			DNFSuffixes: []string{"pecl-redis", "redis"},
			Baseline:    false,
		},
		{
			ID:          "soap",
			Name:        "SOAP",
			Description: "SOAP protocol adapter.",
			APTSuffixes: []string{"soap"},
			DNFSuffixes: []string{"soap"},
			Baseline:    false,
		},
		{
			ID:          "sodium",
			Name:        "Sodium",
			Description: "Modern cryptography primitives.",
			APTSuffixes: []string{"sodium"},
			DNFSuffixes: []string{"sodium"},
			Baseline:    false,
		},
		{
			ID:          "sqlite3",
			Name:        "SQLite3",
			Description: "Embedded SQLite database driver.",
			APTSuffixes: []string{"sqlite3"},
			DNFSuffixes: []string{"sqlite3"},
			Baseline:    true,
		},
		{
			ID:          "xml",
			Name:        "XML",
			Description: "XML parser and writer stack.",
			APTSuffixes: []string{"xml"},
			DNFSuffixes: []string{"xml"},
			Baseline:    true,
		},
		{
			ID:          "zip",
			Name:        "ZIP",
			Description: "ZIP archive handling support.",
			APTSuffixes: []string{"zip"},
			DNFSuffixes: []string{"zip"},
			Baseline:    true,
		},
	}
}

func phpVersionPackageToken(version string) string {
	return strings.ReplaceAll(strings.TrimSpace(version), ".", "")
}

func normalizePHPVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	version = strings.TrimPrefix(strings.ToLower(version), "php")
	if strings.Contains(version, ".") {
		return version
	}
	if len(version) == 2 {
		return version[:1] + "." + version[1:]
	}
	return version
}

func detectPHPPackageManager() (phpPackageManager, error) {
	if fileExists("/usr/bin/dnf") {
		return phpPkgManagerDNF, nil
	}
	if fileExists("/usr/bin/apt-get") {
		return phpPkgManagerAPT, nil
	}
	return "", fmt.Errorf("supported package manager not found (apt-get/dnf)")
}

func installedPHPVersionsSet() map[string]struct{} {
	installed := map[string]struct{}{}
	for _, item := range discoverPHPVersions() {
		if item.Installed {
			installed[item.Version] = struct{}{}
		}
	}
	return installed
}

func firstInstalledPHPVersion() string {
	for _, item := range discoverPHPVersions() {
		if item.Installed {
			return item.Version
		}
	}
	return "8.3"
}

func packageSetFromLines(raw, prefix string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		for _, suffix := range []string{".x86_64", ".aarch64", ".arm64", ".noarch"} {
			name = strings.TrimSuffix(name, suffix)
		}
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		set[name] = struct{}{}
	}
	return set
}

func collectInstalledPHPPackages(manager phpPackageManager, token string) (map[string]struct{}, error) {
	prefix := "lsphp" + token + "-"
	switch manager {
	case phpPkgManagerAPT:
		raw, err := commandOutputTrimmed("dpkg-query", "-W", "-f=${Package}\n")
		if err != nil {
			return nil, err
		}
		return packageSetFromLines(raw, prefix), nil
	case phpPkgManagerDNF:
		raw, err := commandOutputTrimmed("rpm", "-qa", "--qf", "%{NAME}\n")
		if err != nil {
			return nil, err
		}
		return packageSetFromLines(raw, prefix), nil
	default:
		return nil, fmt.Errorf("unsupported package manager: %s", manager)
	}
}

func collectAvailablePHPPackages(manager phpPackageManager, token string) (map[string]struct{}, error) {
	prefix := "lsphp" + token + "-"
	switch manager {
	case phpPkgManagerAPT:
		raw, err := commandOutputTrimmed("apt-cache", "pkgnames", prefix)
		if err != nil {
			return nil, err
		}
		return packageSetFromLines(raw, prefix), nil
	case phpPkgManagerDNF:
		raw, err := commandOutputTrimmed("dnf", "list", "--available", "lsphp"+token+"*")
		if err != nil {
			msg := strings.ToLower(strings.TrimSpace(err.Error()))
			if strings.Contains(msg, "no matching packages") || strings.Contains(msg, "error: no matching") {
				return map[string]struct{}{}, nil
			}
			return nil, err
		}
		return packageSetFromLines(raw, prefix), nil
	default:
		return nil, fmt.Errorf("unsupported package manager: %s", manager)
	}
}

func extensionSuffixCandidates(spec phpExtensionSpec, manager phpPackageManager) []string {
	if manager == phpPkgManagerDNF {
		return spec.DNFSuffixes
	}
	return spec.APTSuffixes
}

func extensionPackageCandidates(spec phpExtensionSpec, token string, manager phpPackageManager) []string {
	prefix := "lsphp" + token + "-"
	suffixes := extensionSuffixCandidates(spec, manager)
	candidates := make([]string, 0, len(suffixes))
	for _, suffix := range suffixes {
		suffix = strings.TrimSpace(suffix)
		if suffix == "" {
			continue
		}
		candidates = append(candidates, prefix+suffix)
	}
	return uniqueStrings(candidates)
}

func resolveExtensionPackage(spec phpExtensionSpec, token string, manager phpPackageManager, installed, available map[string]struct{}) (string, bool, bool) {
	for _, candidate := range extensionPackageCandidates(spec, token, manager) {
		if _, ok := installed[candidate]; ok {
			return candidate, true, true
		}
	}
	for _, candidate := range extensionPackageCandidates(spec, token, manager) {
		if _, ok := available[candidate]; ok {
			return candidate, true, false
		}
	}
	return "", false, false
}

func buildPHPExtensionCatalog(version string) (phpPackageManager, []PHPExtensionInfo, error) {
	normalizedVersion := normalizePHPVersion(version)
	if normalizedVersion == "" {
		normalizedVersion = firstInstalledPHPVersion()
	}
	token := phpVersionPackageToken(normalizedVersion)

	manager, err := detectPHPPackageManager()
	if err != nil {
		return "", nil, err
	}

	installedPackages, err := collectInstalledPHPPackages(manager, token)
	if err != nil {
		return "", nil, err
	}
	availablePackages, err := collectAvailablePHPPackages(manager, token)
	if err != nil {
		return "", nil, err
	}

	specs := phpExtensionCatalogSpecs()
	items := make([]PHPExtensionInfo, 0, len(specs))
	for _, spec := range specs {
		pkgName, available, installed := resolveExtensionPackage(spec, token, manager, installedPackages, availablePackages)
		items = append(items, PHPExtensionInfo{
			ID:          spec.ID,
			Name:        spec.Name,
			Description: spec.Description,
			Package:     pkgName,
			Installed:   installed,
			Available:   available,
			Baseline:    spec.Baseline,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Installed != items[j].Installed {
			return items[i].Installed
		}
		if items[i].Baseline != items[j].Baseline {
			return items[i].Baseline
		}
		return items[i].Name < items[j].Name
	})

	return manager, items, nil
}

func installPHPPackages(manager phpPackageManager, packages []string) error {
	packages = uniqueStrings(packages)
	if len(packages) == 0 {
		return fmt.Errorf("no package selected for install")
	}

	var cmd *exec.Cmd
	switch manager {
	case phpPkgManagerAPT:
		args := append([]string{"install", "-y"}, packages...)
		cmd = exec.Command("apt-get", args...)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	case phpPkgManagerDNF:
		args := append([]string{"install", "-y", "--skip-broken"}, packages...)
		cmd = exec.Command("dnf", args...)
	default:
		return fmt.Errorf("unsupported package manager: %s", manager)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("package install failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func removePHPPackage(manager phpPackageManager, pkg string) error {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return fmt.Errorf("package name is required")
	}

	var cmd *exec.Cmd
	switch manager {
	case phpPkgManagerAPT:
		cmd = exec.Command("apt-get", "remove", "-y", pkg)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	case phpPkgManagerDNF:
		cmd = exec.Command("dnf", "remove", "-y", pkg)
	default:
		return fmt.Errorf("unsupported package manager: %s", manager)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("package remove failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func resolveExtensionSpec(extensionID string) (phpExtensionSpec, bool) {
	key := strings.ToLower(strings.TrimSpace(extensionID))
	for _, spec := range phpExtensionCatalogSpecs() {
		if spec.ID == key {
			return spec, true
		}
	}
	return phpExtensionSpec{}, false
}

func (s *service) handlePHPExtensionsList(w http.ResponseWriter, r *http.Request) {
	version := normalizePHPVersion(r.URL.Query().Get("version"))
	if version == "" {
		version = firstInstalledPHPVersion()
	}
	if _, ok := installedPHPVersionsSet()[version]; !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("PHP %s is not installed.", version))
		return
	}

	manager, items, err := buildPHPExtensionCatalog(version)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	installedCount := 0
	for _, item := range items {
		if item.Installed {
			installedCount++
		}
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"version":         version,
			"package_manager": string(manager),
			"extensions":      items,
			"installed_count": installedCount,
			"total_count":     len(items),
		},
	})
}

func (s *service) handlePHPExtensionInstall(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Version   string `json:"version"`
		Extension string `json:"extension"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid PHP extension install payload.")
		return
	}

	version := normalizePHPVersion(payload.Version)
	if version == "" {
		version = firstInstalledPHPVersion()
	}
	if _, ok := installedPHPVersionsSet()[version]; !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("PHP %s is not installed.", version))
		return
	}

	spec, ok := resolveExtensionSpec(payload.Extension)
	if !ok {
		writeError(w, http.StatusBadRequest, "Unknown extension.")
		return
	}

	manager, catalog, err := buildPHPExtensionCatalog(version)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var selected PHPExtensionInfo
	found := false
	for _, item := range catalog {
		if item.ID == spec.ID {
			selected = item
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusBadRequest, "Extension metadata could not be resolved.")
		return
	}
	if !selected.Available || strings.TrimSpace(selected.Package) == "" {
		writeError(w, http.StatusBadRequest, "Extension package is not available for this PHP version/distribution.")
		return
	}
	if selected.Installed {
		writeJSON(w, http.StatusOK, apiResponse{
			Status:  "success",
			Message: fmt.Sprintf("%s is already installed for PHP %s.", selected.Name, version),
			Data: map[string]interface{}{
				"version":   version,
				"extension": selected,
			},
		})
		return
	}

	if err := installPHPPackages(manager, []string{selected.Package}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = restartPHPRuntime()

	s.mu.Lock()
	s.appendActivityLocked("system", "php_extension_install", fmt.Sprintf("%s installed for PHP %s (%s).", selected.Name, version, selected.Package), "")
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: fmt.Sprintf("%s installed for PHP %s.", selected.Name, version),
		Data: map[string]interface{}{
			"version":   version,
			"extension": selected.ID,
			"package":   selected.Package,
		},
	})
}

func (s *service) handlePHPExtensionRemove(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Version   string `json:"version"`
		Extension string `json:"extension"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid PHP extension remove payload.")
		return
	}

	version := normalizePHPVersion(payload.Version)
	if version == "" {
		version = firstInstalledPHPVersion()
	}
	if _, ok := installedPHPVersionsSet()[version]; !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("PHP %s is not installed.", version))
		return
	}

	spec, ok := resolveExtensionSpec(payload.Extension)
	if !ok {
		writeError(w, http.StatusBadRequest, "Unknown extension.")
		return
	}

	manager, catalog, err := buildPHPExtensionCatalog(version)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var selected PHPExtensionInfo
	found := false
	for _, item := range catalog {
		if item.ID == spec.ID {
			selected = item
			found = true
			break
		}
	}
	if !found || strings.TrimSpace(selected.Package) == "" {
		writeError(w, http.StatusBadRequest, "Extension metadata could not be resolved.")
		return
	}
	if !selected.Installed {
		writeJSON(w, http.StatusOK, apiResponse{
			Status:  "success",
			Message: fmt.Sprintf("%s is not installed for PHP %s.", selected.Name, version),
			Data: map[string]interface{}{
				"version":   version,
				"extension": selected.ID,
			},
		})
		return
	}

	if err := removePHPPackage(manager, selected.Package); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = restartPHPRuntime()

	s.mu.Lock()
	s.appendActivityLocked("system", "php_extension_remove", fmt.Sprintf("%s removed for PHP %s (%s).", selected.Name, version, selected.Package), "")
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: fmt.Sprintf("%s removed for PHP %s.", selected.Name, version),
		Data: map[string]interface{}{
			"version":   version,
			"extension": selected.ID,
			"package":   selected.Package,
		},
	})
}

func discoverPHPVersions() []PHPVersionInfo {
	supportedVersions := []string{"8.4", "8.3", "8.2", "8.1", "8.0", "7.4"}
	items := map[string]PHPVersionInfo{}

	// Initialize all supported versions as not installed
	for _, v := range supportedVersions {
		items[v] = PHPVersionInfo{
			Version:   v,
			Installed: false,
			EOL:       strings.HasPrefix(v, "7.") || v == "8.0",
		}
	}

	patterns := []string{"/usr/local/lsws/lsphp*/bin/lsphp"}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			versionToken := strings.TrimPrefix(filepath.Base(filepath.Dir(filepath.Dir(match))), "lsphp")
			if len(versionToken) < 2 {
				continue
			}
			version := versionToken[:1] + "." + versionToken[1:]

			// If we found it on disk, mark as installed
			if info, exists := items[version]; exists {
				info.Installed = true
				items[version] = info
			} else {
				// It's a version we found but wasn't in our supported list
				items[version] = PHPVersionInfo{
					Version:   version,
					Installed: true,
					EOL:       strings.HasPrefix(version, "7.") || version == "8.0",
				}
			}
		}
	}

	versions := make([]PHPVersionInfo, 0, len(items))
	for _, item := range items {
		versions = append(versions, item)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i].Version > versions[j].Version })
	return versions
}

func detectPHPIniPath(version string) string {
	token := phpVersionPackageToken(version)
	candidates := []string{
		fmt.Sprintf("/usr/local/lsws/lsphp%s/etc/php/%s/litespeed/php.ini", token, version),
		fmt.Sprintf("/usr/local/lsws/lsphp%s/etc/php/%s/php.ini", token, version),
		fmt.Sprintf("/usr/local/lsws/lsphp%s/etc/php.ini", token),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}

func installPHPVersion(version string) error {
	version = normalizePHPVersion(version)
	if version == "" {
		return fmt.Errorf("php version is required")
	}
	token := phpVersionPackageToken(version)

	manager, err := detectPHPPackageManager()
	if err != nil {
		return err
	}

	available, err := collectAvailablePHPPackages(manager, token)
	if err != nil {
		return err
	}
	installed, err := collectInstalledPHPPackages(manager, token)
	if err != nil {
		return err
	}

	packages := []string{
		"lsphp" + token,
		"lsphp" + token + "-common",
	}
	for _, spec := range phpExtensionCatalogSpecs() {
		if !spec.Baseline {
			continue
		}
		pkgName, _, _ := resolveExtensionPackage(spec, token, manager, installed, available)
		if pkgName != "" {
			packages = append(packages, pkgName)
		}
	}
	packages = uniqueStrings(packages)

	if err := installPHPPackages(manager, packages); err != nil {
		return fmt.Errorf("php install failed: %w", err)
	}
	return nil
}

func removePHPVersion(version string) error {
	version = normalizePHPVersion(version)
	if version == "" {
		return fmt.Errorf("php version is required")
	}
	token := phpVersionPackageToken(version)

	manager, err := detectPHPPackageManager()
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	if manager == phpPkgManagerDNF {
		cmd = exec.Command("dnf", "remove", "-y", "lsphp"+token)
	} else {
		cmd = exec.Command("apt-get", "remove", "-y", "lsphp"+token)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("php remove failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func restartPHPRuntime() error {
	if fileExists("/usr/local/lsws/bin/lswsctrl") {
		cmd := exec.Command("/usr/local/lsws/bin/lswsctrl", "reload")
		if output, err := cmd.CombinedOutput(); err == nil {
			_ = output
			return nil
		}
	}
	cmd := exec.Command("systemctl", "reload", "lsws")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("php runtime reload failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}
