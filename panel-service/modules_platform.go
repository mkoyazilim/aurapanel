package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (s *service) handleDockerContainersGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.DockerContainers})
}

func (s *service) handleDockerContainerCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name          string   `json:"name"`
		Image         string   `json:"image"`
		Ports         []string `json:"ports"`
		RestartPolicy string   `json:"restart_policy"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid container payload.")
		return
	}
	if payload.Name == "" || payload.Image == "" {
		writeError(w, http.StatusBadRequest, "Container name and image are required.")
		return
	}
	container := DockerContainer{
		ID:      generateSecret(8),
		Name:    sanitizeName(payload.Name),
		Image:   payload.Image,
		Status:  "Up 10 seconds",
		Ports:   strings.Join(payload.Ports, ", "),
		Created: time.Now().UTC().Format("2006-01-02 15:04"),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.DockerContainers = append([]DockerContainer{container}, s.modules.DockerContainers...)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Container created.", Data: container})
}

func (s *service) handleDockerContainerAction(w http.ResponseWriter, r *http.Request, action string) {
	var payload struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid container action payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.modules.DockerContainers {
		if s.modules.DockerContainers[i].ID != payload.ID {
			continue
		}
		switch action {
		case "start":
			s.modules.DockerContainers[i].Status = "Up 5 seconds"
		case "stop":
			s.modules.DockerContainers[i].Status = "Exited (0) just now"
		case "restart":
			s.modules.DockerContainers[i].Status = "Up 1 second"
		case "remove":
			s.modules.DockerContainers = append(s.modules.DockerContainers[:i], s.modules.DockerContainers[i+1:]...)
		}
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Container action applied."})
		return
	}
	writeError(w, http.StatusNotFound, "Container not found.")
}

func (s *service) handleDockerImagesGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.DockerImages})
}

func (s *service) handleDockerImagePull(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Image string `json:"image"`
		Tag   string `json:"tag"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid image pull payload.")
		return
	}
	image := DockerImage{
		ID:         "sha256:" + generateSecret(8),
		Repository: firstNonEmpty(strings.TrimSpace(payload.Image), "custom"),
		Tag:        firstNonEmpty(strings.TrimSpace(payload.Tag), "latest"),
		Size:       "180 MB",
		Created:    time.Now().UTC().Format("2006-01-02"),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.DockerImages = append([]DockerImage{image}, s.modules.DockerImages...)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Image pulled.", Data: image})
}

func (s *service) handleDockerImageRemove(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid image remove payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.modules.DockerImages
	filtered := items[:0]
	deleted := false
	for _, item := range items {
		if item.ID == payload.ID {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.DockerImages = filtered
	if !deleted {
		writeError(w, http.StatusNotFound, "Image not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Image removed."})
}

func (s *service) handleDockerTemplatesGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.DockerTemplates})
}

func (s *service) handleDockerInstalledAppsGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.DockerInstalled})
}

func (s *service) handleDockerPackagesGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.DockerPackages})
}

func (s *service) handleDockerAppInstall(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		TemplateID string `json:"template_id"`
		AppName    string `json:"app_name"`
		PackageID  string `json:"package_id"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid docker app install payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var template DockerAppTemplate
	for _, item := range s.modules.DockerTemplates {
		if item.ID == payload.TemplateID {
			template = item
			break
		}
	}
	if template.ID == "" {
		writeError(w, http.StatusNotFound, "Template not found.")
		return
	}
	app := DockerInstalledApp{
		Name:    firstNonEmpty(payload.AppName, "app-"+template.ID),
		Image:   template.Image,
		Status:  "Up 5 seconds",
		Ports:   "8080:8080",
		Package: firstNonEmpty(payload.PackageID, "unlimited"),
	}
	s.modules.DockerInstalled = append([]DockerInstalledApp{app}, s.modules.DockerInstalled...)
	s.modules.DockerContainers = append([]DockerContainer{{
		ID:      generateSecret(8),
		Name:    sanitizeName(app.Name),
		Image:   app.Image,
		Status:  app.Status,
		Ports:   app.Ports,
		Created: time.Now().UTC().Format("2006-01-02 15:04"),
	}}, s.modules.DockerContainers...)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Docker app installed.", Data: app})
}

func (s *service) handleDockerAppRemove(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		AppName string `json:"app_name"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid docker app remove payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.modules.DockerInstalled
	filtered := items[:0]
	deleted := false
	for _, item := range items {
		if item.Name == payload.AppName {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.DockerInstalled = filtered
	if !deleted {
		writeError(w, http.StatusNotFound, "Installed app not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Docker app removed."})
}

func (s *service) handleMinIOBucketsList(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.MinIOBuckets})
}

func (s *service) handleMinIOBucketCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		BucketName string `json:"bucket_name"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid bucket payload.")
		return
	}
	name := sanitizeName(payload.BucketName)
	if name == "" {
		writeError(w, http.StatusBadRequest, "Bucket name is required.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.MinIOBuckets = append(s.modules.MinIOBuckets, name)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Bucket created."})
}

func (s *service) handleMinIOCredentialCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		User string `json:"user"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid MinIO credential payload.")
		return
	}
	user := firstNonEmpty(strings.TrimSpace(payload.User), "admin")
	creds := MinIOCredential{
		User:      user,
		AccessKey: strings.ToUpper(sanitizeName(user)) + "KEY",
		SecretKey: "minio-" + strings.ToLower(generateSecret(12)),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.MinIOCredentials[user] = creds
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: creds, Message: "Credentials generated."})
}

func (s *service) handleFederatedNodes(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.FederatedNodes})
}

func (s *service) handleFederatedMode(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.FederatedMode})
}

func (s *service) handleFederatedJoin(w http.ResponseWriter, r *http.Request) {
	var payload FederatedNode
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid federated join payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.FederatedNodes = append(s.modules.FederatedNodes, payload)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Node joined federation.", Data: payload})
}

func (s *service) handleRuntimeAppsList(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.RuntimeApps})
}

func (s *service) handleRuntimeNodeInstall(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Node.js dependencies installed."})
}

func (s *service) handleRuntimeNodeStart(w http.ResponseWriter, r *http.Request) {
	var payload RuntimeApp
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Node.js start payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	app := RuntimeApp{Runtime: "nodejs", Dir: payload.Dir, AppName: firstNonEmpty(payload.AppName, "node-app"), Status: "running"}
	s.modules.RuntimeApps = append([]RuntimeApp{app}, filterRuntimeApps(s.modules.RuntimeApps, app.AppName)...)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Node.js app started.", Data: app})
}

func (s *service) handleRuntimeNodeStop(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		AppName string `json:"app_name"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Node.js stop payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.modules.RuntimeApps {
		if s.modules.RuntimeApps[i].AppName == payload.AppName {
			s.modules.RuntimeApps[i].Status = "stopped"
		}
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Node.js app stopped."})
}

func (s *service) handleRuntimePythonVenv(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Python virtualenv created."})
}

func (s *service) handleRuntimePythonInstall(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Python requirements installed."})
}

func (s *service) handleRuntimePythonStart(w http.ResponseWriter, r *http.Request) {
	var payload RuntimeApp
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Python start payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	app := RuntimeApp{Runtime: "python", Dir: payload.Dir, AppName: firstNonEmpty(payload.AppName, "python-app"), Status: "running"}
	s.modules.RuntimeApps = append([]RuntimeApp{app}, filterRuntimeApps(s.modules.RuntimeApps, app.AppName)...)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Python app started.", Data: app})
}

func filterRuntimeApps(items []RuntimeApp, appName string) []RuntimeApp {
	filtered := items[:0]
	for _, item := range items {
		if item.AppName != appName {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (s *service) findWordPressSiteIndexLocked(domain string) int {
	for i := range s.modules.WordPressSites {
		if s.modules.WordPressSites[i].Domain == domain {
			return i
		}
	}
	return -1
}

func (s *service) refreshWordPressSiteStatsLocked(domain string) {
	index := s.findWordPressSiteIndexLocked(domain)
	if index == -1 {
		return
	}
	plugins := s.modules.WordPressPlugins[domain]
	themes := s.modules.WordPressThemes[domain]
	activePlugins := 0
	for _, plugin := range plugins {
		if plugin.Status == "active" {
			activePlugins++
		}
	}
	activeTheme := "-"
	for _, theme := range themes {
		if theme.Status == "active" {
			activeTheme = firstNonEmpty(theme.Title, theme.Name)
			break
		}
	}
	s.modules.WordPressSites[index].ActivePlugins = activePlugins
	s.modules.WordPressSites[index].TotalPlugins = len(plugins)
	s.modules.WordPressSites[index].ActiveTheme = activeTheme
}

func (s *service) handleCMSInstall(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		AppType    string `json:"app_type"`
		Domain     string `json:"domain"`
		DBName     string `json:"db_name"`
		DBUser     string `json:"db_user"`
		AdminEmail string `json:"admin_email"`
		AdminUser  string `json:"admin_user"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid CMS install payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	if domain == "" {
		writeError(w, http.StatusBadRequest, "Domain is required.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.findWebsiteLocked(domain) == nil {
		s.state.Websites = append(s.state.Websites, Website{
			Domain:        domain,
			Owner:         "aura",
			User:          "aura",
			PHP:           "8.3",
			PHPVersion:    "8.3",
			Package:       "default",
			Email:         firstNonEmpty(payload.AdminEmail, "admin@"+domain),
			Status:        "active",
			SSL:           true,
			DiskUsage:     "256 MB",
			Quota:         quotaForPackage(s.state.Packages, "default"),
			MailDomain:    true,
			ApacheBackend: false,
			CreatedAt:     time.Now().UTC().Unix(),
		})
		s.ensureDefaultSiteArtifactsLocked(domain)
	}
	if payload.AppType == "wordpress" {
		wp := buildWordPressSite(domain, "aura", firstNonEmpty(payload.AdminEmail, "admin@"+domain), "8.3")
		wp.DBName = firstNonEmpty(payload.DBName, wp.DBName)
		wp.DBUser = firstNonEmpty(payload.DBUser, wp.DBUser)
		if index := s.findWordPressSiteIndexLocked(domain); index >= 0 {
			s.modules.WordPressSites[index] = wp
		} else {
			s.modules.WordPressSites = append([]WordPressSite{wp}, s.modules.WordPressSites...)
		}
		if _, ok := s.modules.WordPressPlugins[domain]; !ok {
			s.modules.WordPressPlugins[domain] = []WordPressPlugin{
				{Name: "akismet", Title: "Akismet", Version: "5.0", Status: "active", Update: "up-to-date"},
				{Name: "performance-lab", Title: "Performance Lab", Version: "4.2", Status: "inactive", Update: "up-to-date"},
			}
		}
		if _, ok := s.modules.WordPressThemes[domain]; !ok {
			s.modules.WordPressThemes[domain] = []WordPressTheme{
				{Name: "twentytwentysix", Title: "Twenty Twenty-Six", Version: "1.0", Status: "active", Update: "up-to-date"},
			}
		}
		s.refreshWordPressSiteStatsLocked(domain)
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("%s installed on %s.", firstNonEmpty(payload.AppType, "Application"), domain)})
}

func (s *service) handleWordPressSites(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.WordPressSites})
}

func (s *service) handleWordPressScan(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "WordPress scan completed.", Data: s.modules.WordPressSites})
}

func (s *service) handleWordPressPluginsGet(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.WordPressPlugins[domain]})
}

func (s *service) handleWordPressPluginsUpdate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string   `json:"domain"`
		Names  []string `json:"names"`
		All    bool     `json:"all"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid WordPress plugin update payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.modules.WordPressPlugins[domain] {
		if payload.All || containsString(payload.Names, s.modules.WordPressPlugins[domain][i].Name) {
			s.modules.WordPressPlugins[domain][i].Update = "up-to-date"
			if s.modules.WordPressPlugins[domain][i].Status == "" {
				s.modules.WordPressPlugins[domain][i].Status = "active"
			}
		}
	}
	s.refreshWordPressSiteStatsLocked(domain)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Plugins updated."})
}

func (s *service) handleWordPressPluginsDelete(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string   `json:"domain"`
		Names  []string `json:"names"`
		All    bool     `json:"all"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid WordPress plugin delete payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.modules.WordPressPlugins[domain]
	filtered := items[:0]
	for _, item := range items {
		if payload.All || containsString(payload.Names, item.Name) {
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.WordPressPlugins[domain] = filtered
	s.refreshWordPressSiteStatsLocked(domain)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Plugins deleted."})
}

func (s *service) handleWordPressThemesGet(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.WordPressThemes[domain]})
}

func (s *service) handleWordPressThemesUpdate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string   `json:"domain"`
		Names  []string `json:"names"`
		All    bool     `json:"all"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid WordPress theme update payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.modules.WordPressThemes[domain] {
		if payload.All || containsString(payload.Names, s.modules.WordPressThemes[domain][i].Name) {
			s.modules.WordPressThemes[domain][i].Update = "up-to-date"
		}
	}
	s.refreshWordPressSiteStatsLocked(domain)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Themes updated."})
}

func (s *service) handleWordPressThemesDelete(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain string   `json:"domain"`
		Names  []string `json:"names"`
		All    bool     `json:"all"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid WordPress theme delete payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.modules.WordPressThemes[domain]
	filtered := items[:0]
	for _, item := range items {
		if payload.All || containsString(payload.Names, item.Name) {
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.WordPressThemes[domain] = filtered
	s.refreshWordPressSiteStatsLocked(domain)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Themes deleted."})
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func (s *service) handleWordPressBackupsGet(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.WordPressBackups[domain]})
}

func (s *service) handleWordPressBackupCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain     string `json:"domain"`
		BackupType string `json:"backup_type"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid WordPress backup payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	record := WordPressBackup{
		ID:         generateSecret(8),
		Domain:     domain,
		FileName:   fmt.Sprintf("%s-%s-%s.tar.gz", domain, firstNonEmpty(payload.BackupType, "full"), time.Now().UTC().Format("20060102-150405")),
		BackupType: firstNonEmpty(payload.BackupType, "full"),
		SizeBytes:  157286400,
		CreatedAt:  time.Now().UTC().Unix(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.WordPressBackups[domain] = append([]WordPressBackup{record}, s.modules.WordPressBackups[domain]...)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "WordPress backup created.", Data: record})
}

func (s *service) handleWordPressBackupDownload(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, items := range s.modules.WordPressBackups {
		for _, item := range items {
			if item.ID == id {
				writeBlob(w, item.FileName, "application/gzip", []byte("-- simulated wordpress backup --\n"))
				return
			}
		}
	}
	writeError(w, http.StatusNotFound, "WordPress backup not found.")
}

func (s *service) handleWordPressBackupRestore(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid WordPress restore payload.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("WordPress backup restore queued for %s.", payload.ID)})
}

func (s *service) handleWordPressStagingGet(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.WordPressStaging[domain]})
}

func (s *service) handleWordPressStagingCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		SourceDomain  string `json:"source_domain"`
		StagingDomain string `json:"staging_domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid WordPress staging payload.")
		return
	}
	source := normalizeDomain(payload.SourceDomain)
	target := normalizeDomain(payload.StagingDomain)
	if source == "" || target == "" {
		writeError(w, http.StatusBadRequest, "Source and staging domain are required.")
		return
	}
	record := WordPressStaging{
		ID:            generateSecret(8),
		SourceDomain:  source,
		StagingDomain: target,
		Owner:         "aura",
		CreatedAt:     time.Now().UTC().Unix(),
		Status:        "ready",
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.WordPressStaging[source] = append([]WordPressStaging{record}, s.modules.WordPressStaging[source]...)
	if s.findWordPressSiteIndexLocked(target) == -1 {
		wp := buildWordPressSite(target, "aura", "admin@"+target, "8.3")
		wp.Status = "staging"
		s.modules.WordPressSites = append([]WordPressSite{wp}, s.modules.WordPressSites...)
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Staging site created.", Data: record})
}
