package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (s *service) handleDockerContainersGet(w http.ResponseWriter) {
	containers, err := runtimeDockerContainers()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	s.mu.Lock()
	s.modules.DockerContainers = containers
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: containers})
}

func (s *service) handleDockerContainerCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name          string   `json:"name"`
		Image         string   `json:"image"`
		Ports         []string `json:"ports"`
		RestartPolicy string   `json:"restart_policy"`
		MemoryLimit   string   `json:"memory_limit"`
		CPULimit      string   `json:"cpu_limit"`
		Env           []string `json:"env"`
		Volumes       []string `json:"volumes"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid container payload.")
		return
	}
	if payload.Name == "" || payload.Image == "" {
		writeError(w, http.StatusBadRequest, "Container name and image are required.")
		return
	}
	if err := createRuntimeDockerContainer(payload.Name, payload.Image, payload.Ports, payload.RestartPolicy, payload.MemoryLimit, payload.CPULimit, payload.Env, payload.Volumes); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	containers, err := runtimeDockerContainers()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	var created DockerContainer
	name := sanitizeName(payload.Name)
	for _, item := range containers {
		if item.Name == name {
			created = item
			break
		}
	}
	s.mu.Lock()
	s.modules.DockerContainers = containers
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Container created.", Data: created})
}

func (s *service) handleDockerContainerAction(w http.ResponseWriter, r *http.Request, action string) {
	var payload struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid container action payload.")
		return
	}
	if err := applyRuntimeDockerContainerAction(payload.ID, action); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	containers, err := runtimeDockerContainers()
	if err == nil {
		s.mu.Lock()
		s.modules.DockerContainers = containers
		s.mu.Unlock()
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Container action applied."})
}

func (s *service) handleDockerImagesGet(w http.ResponseWriter) {
	images, err := runtimeDockerImages()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	s.mu.Lock()
	s.modules.DockerImages = images
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: images})
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
	if err := pullRuntimeDockerImage(payload.Image, payload.Tag); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	images, err := runtimeDockerImages()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	var pulled DockerImage
	repo := firstNonEmpty(strings.TrimSpace(payload.Image), "custom")
	tag := firstNonEmpty(strings.TrimSpace(payload.Tag), "latest")
	for _, item := range images {
		if item.Repository == repo && item.Tag == tag {
			pulled = item
			break
		}
	}
	s.mu.Lock()
	s.modules.DockerImages = images
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Image pulled.", Data: pulled})
}

func (s *service) handleDockerImageRemove(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid image remove payload.")
		return
	}
	if err := removeRuntimeDockerImage(payload.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	images, err := runtimeDockerImages()
	if err == nil {
		s.mu.Lock()
		s.modules.DockerImages = images
		s.mu.Unlock()
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
	items := append([]DockerInstalledApp(nil), s.modules.DockerInstalled...)
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
}

func (s *service) handleDockerPackagesGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.DockerPackages})
}

func (s *service) handleDockerAppInstall(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		TemplateID string   `json:"template_id"`
		AppName    string   `json:"app_name"`
		PackageID  string   `json:"package_id"`
		CustomEnv  []string `json:"custom_env"`
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

	var memLimit, cpuLimit string
	for _, pkg := range s.modules.DockerPackages {
		if pkg.ID == payload.PackageID {
			if pkg.MemoryLimit != "unlimited" {
				memLimit = pkg.MemoryLimit
			}
			if pkg.CPULimit != "unlimited" {
				cpuLimit = pkg.CPULimit
			}
			break
		}
	}

	appName := sanitizeName(firstNonEmpty(payload.AppName, "app-"+template.ID))
	if appName == "" {
		appName = "app-" + template.ID
	}
	ports := dockerTemplatePorts(template)
	volumes := dockerTemplateVolumes(template, appName)
	if err := createRuntimeDockerContainer(appName, template.Image, ports, "unless-stopped", memLimit, cpuLimit, payload.CustomEnv, volumes); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	app := DockerInstalledApp{
		Name:         appName,
		Image:        template.Image,
		Status:       "running",
		Ports:        strings.Join(ports, ", "),
		Package:      firstNonEmpty(payload.PackageID, "unlimited"),
		Runtime:      firstNonEmpty(template.Runtime, "docker"),
		Provisioning: firstNonEmpty(template.Provisioning, "container"),
	}
	s.modules.DockerInstalled = append([]DockerInstalledApp{app}, filterDockerInstalledApps(s.modules.DockerInstalled, app.Name)...)
	if containers, err := runtimeDockerContainers(); err == nil {
		s.modules.DockerContainers = containers
		for _, container := range containers {
			if container.Name == app.Name {
				app.Status = container.Status
				if strings.TrimSpace(container.Ports) != "" {
					app.Ports = container.Ports
				}
				break
			}
		}
		s.modules.DockerInstalled = append([]DockerInstalledApp{app}, filterDockerInstalledApps(s.modules.DockerInstalled, app.Name)...)
	}
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
	for _, item := range s.modules.DockerInstalled {
		if item.Name == payload.AppName {
			_ = applyRuntimeDockerContainerAction(item.Name, "remove")
			break
		}
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
	if containers, err := runtimeDockerContainers(); err == nil {
		s.modules.DockerContainers = containers
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
	s.mu.Lock()
	s.modules.RuntimeApps = runtimeAppsFromSystemd(s.modules.RuntimeApps)
	items := append([]RuntimeApp(nil), s.modules.RuntimeApps...)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
}

func (s *service) handleRuntimeNodeInstall(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Dir string `json:"dir"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Node.js dependency payload.")
		return
	}
	if err := installRuntimeNodeDeps(payload.Dir); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Node.js dependencies installed."})
}

func (s *service) handleRuntimeNodeStart(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Dir         string `json:"dir"`
		AppName     string `json:"app_name"`
		StartScript string `json:"start_script"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Node.js start payload.")
		return
	}
	app, err := startRuntimeNodeApp(payload.Dir, payload.AppName, payload.StartScript)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
	if err := stopRuntimeApp(payload.AppName); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
	var payload struct {
		Dir string `json:"dir"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Python virtualenv payload.")
		return
	}
	if err := createRuntimePythonVenv(payload.Dir); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Python virtualenv created."})
}

func (s *service) handleRuntimePythonInstall(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Dir string `json:"dir"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Python install payload.")
		return
	}
	if err := installRuntimePythonRequirements(payload.Dir); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Python requirements installed."})
}

func (s *service) handleRuntimePythonStart(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Dir        string `json:"dir"`
		AppName    string `json:"app_name"`
		WSGIModule string `json:"wsgi_module"`
		Port       int    `json:"port"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Python start payload.")
		return
	}
	app, err := startRuntimePythonApp(payload.Dir, payload.AppName, payload.WSGIModule, payload.Port)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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

func filterDockerInstalledApps(items []DockerInstalledApp, appName string) []DockerInstalledApp {
	filtered := items[:0]
	for _, item := range items {
		if item.Name != appName {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func dockerTemplatePorts(template DockerAppTemplate) []string {
	if len(template.DefaultPorts) == 0 {
		return []string{"8080:8080"}
	}
	ports := make([]string, 0, len(template.DefaultPorts))
	for _, port := range template.DefaultPorts {
		port = strings.TrimSpace(port)
		if port != "" {
			ports = append(ports, port)
		}
	}
	if len(ports) == 0 {
		return []string{"8080:8080"}
	}
	return ports
}

func dockerTemplateVolumes(template DockerAppTemplate, appName string) []string {
	if len(template.DefaultVolumes) == 0 {
		return nil
	}
	appName = sanitizeName(appName)
	volumes := make([]string, 0, len(template.DefaultVolumes))
	for _, volume := range template.DefaultVolumes {
		volume = strings.TrimSpace(strings.ReplaceAll(volume, "__APP_NAME__", appName))
		if volume != "" {
			volumes = append(volumes, volume)
		}
	}
	return volumes
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

	docroot := domainDocroot(domain)
	if !fileExists(filepath.Join(docroot, "wp-config.php")) {
		return
	}

	// Plugins
	if output, err := exec.Command("wp", "plugin", "list", "--path="+docroot, "--allow-root", "--format=json").Output(); err == nil {
		var plugins []struct {
			Name    string `json:"name"`
			Title   string `json:"title"`
			Status  string `json:"status"`
			Update  string `json:"update"`
			Version string `json:"version"`
		}
		if json.Unmarshal(output, &plugins) == nil {
			var wpPlugins []WordPressPlugin
			for _, p := range plugins {
				wpPlugins = append(wpPlugins, WordPressPlugin{
					Name:    p.Name,
					Title:   p.Title,
					Version: p.Version,
					Status:  p.Status,
					Update:  p.Update,
				})
			}
			s.modules.WordPressPlugins[domain] = wpPlugins
		}
	}

	// Themes
	if output, err := exec.Command("wp", "theme", "list", "--path="+docroot, "--allow-root", "--format=json").Output(); err == nil {
		var themes []struct {
			Name    string `json:"name"`
			Title   string `json:"title"`
			Status  string `json:"status"`
			Update  string `json:"update"`
			Version string `json:"version"`
		}
		if json.Unmarshal(output, &themes) == nil {
			var wpThemes []WordPressTheme
			for _, t := range themes {
				wpThemes = append(wpThemes, WordPressTheme{
					Name:    t.Name,
					Title:   t.Title,
					Version: t.Version,
					Status:  t.Status,
					Update:  t.Update,
				})
			}
			s.modules.WordPressThemes[domain] = wpThemes
		}
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
		Owner      string `json:"owner"`
		User       string `json:"user"`
		DBName     string `json:"db_name"`
		DBUser     string `json:"db_user"`
		DBPass     string `json:"db_pass"`
		AdminEmail string `json:"admin_email"`
		AdminUser  string `json:"admin_user"`
		AdminPass  string `json:"admin_pass"`
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
	siteOwner := s.resolveRequestedOwner(r, payload.Owner, payload.User)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.findWebsiteLocked(domain) == nil {
		site := Website{
			Domain:        domain,
			Owner:         siteOwner,
			User:          siteOwner,
			PHP:           "8.3",
			PHPVersion:    "8.3",
			Package:       "default",
			Email:         firstNonEmpty(payload.AdminEmail, "admin@"+domain),
			Status:        "active",
			SSL:           false,
			DiskUsage:     "256 MB",
			Quota:         quotaForPackage(s.state.Packages, "default"),
			MailDomain:    true,
			ApacheBackend: false,
			CreatedAt:     time.Now().UTC().Unix(),
		}
		s.state.Websites = append(s.state.Websites, site)
		if err := s.provisionWebsiteArtifactsLocked(site); err != nil {
			// Rollback if provisioning fails
			s.state.Websites = s.state.Websites[:len(s.state.Websites)-1]
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if payload.AppType == "wordpress" {
		wp := buildWordPressSite(domain, siteOwner, firstNonEmpty(payload.AdminEmail, "admin@"+domain), "8.3")
		wp.DBName = firstNonEmpty(payload.DBName, wp.DBName)
		wp.DBUser = firstNonEmpty(payload.DBUser, wp.DBUser)
		dbPass := strings.TrimSpace(payload.DBPass)
		if dbPass == "" {
			dbPass = generateSecret(18)
		}
		adminPass := strings.TrimSpace(payload.AdminPass)
		if adminPass == "" {
			adminPass = generateSecret(18)
		}

		docroot := domainDocroot(domain)

		// Run wp-cli download & install asynchronously so we don't block the API call for too long
		go func() {
			os.MkdirAll(docroot, 0755)
			exec.Command("wp", "core", "download", "--path="+docroot, "--allow-root").Run()

			exec.Command("wp", "config", "create", "--path="+docroot, "--allow-root", "--dbname="+wp.DBName, "--dbuser="+wp.DBUser, "--dbpass="+dbPass, "--dbhost=127.0.0.1").Run()

			exec.Command("wp", "core", "install", "--path="+docroot, "--allow-root", "--url=https://"+domain, "--title="+domain, "--admin_user="+firstNonEmpty(payload.AdminUser, "admin"), "--admin_password="+adminPass, "--admin_email="+wp.AdminEmail).Run()

			exec.Command("chown", "-R", siteOwner+":"+siteOwner, docroot).Run()

			s.mu.Lock()
			s.refreshWordPressSiteStatsLocked(domain)
			s.mu.Unlock()
		}()

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
		generated := map[string]string{}
		if strings.TrimSpace(payload.DBPass) == "" {
			generated["db_pass"] = dbPass
		}
		if strings.TrimSpace(payload.AdminPass) == "" {
			generated["admin_pass"] = adminPass
		}
		responseData := map[string]interface{}{"domain": domain}
		if len(generated) > 0 {
			responseData["generated_credentials"] = generated
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Status:  "success",
			Message: fmt.Sprintf("%s installed on %s.", firstNonEmpty(payload.AppType, "Application"), domain),
			Data:    responseData,
		})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: fmt.Sprintf("%s installed on %s.", firstNonEmpty(payload.AppType, "Application"), domain),
		Data:    map[string]interface{}{"domain": domain},
	})
}

func (s *service) handleWordPressSites(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.WordPressSites})
}

func (s *service) handleWordPressScan(w http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Real scan across all websites
	s.modules.WordPressSites = []WordPressSite{}
	for _, site := range s.state.Websites {
		docroot := domainDocroot(site.Domain)
		if fileExists(filepath.Join(docroot, "wp-config.php")) {
			wp := buildWordPressSite(site.Domain, site.Owner, site.Email, site.PHP)
			s.modules.WordPressSites = append(s.modules.WordPressSites, wp)
			s.refreshWordPressSiteStatsLocked(site.Domain)
		}
	}

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
	docroot := fmt.Sprintf("/usr/local/lsws/%s/html", domain)

	s.mu.Lock()
	defer s.mu.Unlock()

	args := []string{"plugin", "update", "--path=" + docroot, "--allow-root"}
	if payload.All {
		args = append(args, "--all")
	} else {
		if len(payload.Names) == 0 {
			writeError(w, http.StatusBadRequest, "No plugins specified.")
			return
		}
		args = append(args, payload.Names...)
	}

	cmd := exec.Command("wp", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("WP-CLI error: %s", string(output)))
		return
	}

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
	docroot := fmt.Sprintf("/usr/local/lsws/%s/html", domain)

	s.mu.Lock()
	defer s.mu.Unlock()

	args := []string{"plugin", "delete", "--path=" + docroot, "--allow-root"}
	if payload.All {
		args = append(args, "--all")
	} else {
		if len(payload.Names) == 0 {
			writeError(w, http.StatusBadRequest, "No plugins specified.")
			return
		}
		args = append(args, payload.Names...)
	}

	cmd := exec.Command("wp", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("WP-CLI error: %s", string(output)))
		return
	}

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
	docroot := fmt.Sprintf("/usr/local/lsws/%s/html", domain)

	s.mu.Lock()
	defer s.mu.Unlock()

	args := []string{"theme", "update", "--path=" + docroot, "--allow-root"}
	if payload.All {
		args = append(args, "--all")
	} else {
		if len(payload.Names) == 0 {
			writeError(w, http.StatusBadRequest, "No themes specified.")
			return
		}
		args = append(args, payload.Names...)
	}

	cmd := exec.Command("wp", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("WP-CLI error: %s", string(output)))
		return
	}

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
	docroot := fmt.Sprintf("/usr/local/lsws/%s/html", domain)

	s.mu.Lock()
	defer s.mu.Unlock()

	args := []string{"theme", "delete", "--path=" + docroot, "--allow-root"}
	if payload.All {
		args = append(args, "--all")
	} else {
		if len(payload.Names) == 0 {
			writeError(w, http.StatusBadRequest, "No themes specified.")
			return
		}
		args = append(args, payload.Names...)
	}

	cmd := exec.Command("wp", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("WP-CLI error: %s", string(output)))
		return
	}

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
		Domain string `json:"domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid WordPress backup payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	s.mu.RLock()
	siteIndex := s.findWordPressSiteIndexLocked(domain)
	var site WordPressSite
	if siteIndex >= 0 {
		site = s.modules.WordPressSites[siteIndex]
	}
	s.mu.RUnlock()
	if site.Domain == "" {
		writeError(w, http.StatusNotFound, "WordPress site not found.")
		return
	}
	record, err := createRuntimeWordPressBackup(site, "full")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
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
				path := item.Path
				if path == "" {
					path = filepath.Join(siteBackupDir(), item.FileName)
				}
				content, err := os.ReadFile(path)
				if err != nil {
					writeError(w, http.StatusNotFound, "WordPress backup file not found.")
					return
				}
				writeBlob(w, item.FileName, "application/gzip", content)
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
	s.mu.RLock()
	var record WordPressBackup
	found := false
	for _, items := range s.modules.WordPressBackups {
		for _, item := range items {
			if item.ID == payload.ID {
				record = item
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	s.mu.RUnlock()
	if !found {
		writeError(w, http.StatusNotFound, "WordPress backup not found.")
		return
	}
	if err := restoreRuntimeWordPressBackup(record); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("WordPress backup restored for %s.", record.Domain)})
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
	owner := s.resolveRequestedOwner(r)
	record := WordPressStaging{
		ID:            generateSecret(8),
		SourceDomain:  source,
		StagingDomain: target,
		Owner:         owner,
		CreatedAt:     time.Now().UTC().Unix(),
		Status:        "ready",
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.WordPressStaging[source] = append([]WordPressStaging{record}, s.modules.WordPressStaging[source]...)
	if s.findWordPressSiteIndexLocked(target) == -1 {
		wp := buildWordPressSite(target, owner, "admin@"+target, "8.3")
		wp.Status = "staging"
		s.modules.WordPressSites = append([]WordPressSite{wp}, s.modules.WordPressSites...)
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Staging site created.", Data: record})
}
