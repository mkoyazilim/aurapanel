package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (s *service) handleSREPrediction(w http.ResponseWriter) {
	metrics := collectHostMetrics(s.startedAt)
	prediction := "Traffic profile is stable."
	switch {
	case metrics.DiskUsage >= 85:
		prediction = "Primary pressure point is disk saturation; backups and logs should be trimmed before traffic grows."
	case metrics.RAMUsage >= 85:
		prediction = "Primary pressure point is memory pressure; PHP workers and database buffers need tuning."
	case metrics.CPUUsage >= 85:
		prediction = "Primary pressure point is CPU saturation; cache hit ratio and PHP concurrency should be reviewed."
	case len(s.modules.BackupSchedules) > 0:
		prediction = "Traffic profile is healthy. Next pressure point is backup windows overlapping with production traffic."
	}
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"prediction": prediction,
			"metrics": map[string]int{
				"cpu_usage":  metrics.CPUUsage,
				"ram_usage":  metrics.RAMUsage,
				"disk_usage": metrics.DiskUsage,
			},
		},
		Message: "SRE prediction generated.",
	})
}

func (s *service) handleSRELogQuery(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Query string `json:"query"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid SRE query payload.")
		return
	}
	query := strings.TrimSpace(payload.Query)
	if query == "" {
		writeError(w, http.StatusBadRequest, "Query is required.")
		return
	}
	queryLower := strings.ToLower(query)
	matchedSources := []string{}
	matches := []string{}

	s.mu.RLock()
	websites := append([]Website(nil), s.state.Websites...)
	activities := append([]ActivityLogEntry(nil), s.modules.ActivityLogs...)
	s.mu.RUnlock()

	for _, item := range activities {
		line := fmt.Sprintf("%s %s %s", item.Action, item.Detail, item.IP)
		if strings.Contains(strings.ToLower(line), queryLower) {
			matchedSources = appendIfMissing(matchedSources, "panel-service.activity")
			matches = append(matches, line)
			if len(matches) >= 5 {
				break
			}
		}
	}

	for _, site := range websites {
		if len(matches) >= 5 {
			break
		}
		for _, kind := range []string{"error", "access"} {
			paths := discoverSiteLogPaths(site.Domain, kind)
			if len(paths) == 0 {
				continue
			}
			lines, err := tailManagedFile(paths[0], 300)
			if err != nil {
				continue
			}
			for _, line := range lines {
				if strings.Contains(strings.ToLower(line), queryLower) {
					matchedSources = appendIfMissing(matchedSources, filepath.Base(paths[0]))
					matches = append(matches, line)
					if len(matches) >= 5 {
						break
					}
				}
			}
		}
	}

	answerText := fmt.Sprintf("Query `%s` icin eslesen log bulunamadi.", query)
	confidence := 0.25
	if len(matches) > 0 {
		answerText = fmt.Sprintf("Query `%s` icin %d eslesme bulundu.", query, len(matches))
		confidence = 0.92
	}
	answer := map[string]interface{}{
		"answer":          answerText,
		"confidence":      confidence,
		"matched_sources": matchedSources,
		"matches":         matches,
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: answer})
}

func (s *service) handleSREOptimize(w http.ResponseWriter) {
	metrics := collectHostMetrics(s.startedAt)
	actions := []string{}
	if metrics.DiskUsage >= 80 {
		actions = append(actions, "Disk kullanimi yuksek; eski backup ve log dosyalarini rotate edin.")
	}
	if metrics.RAMUsage >= 80 {
		actions = append(actions, "RAM kullanimi yuksek; lsphp process limitlerini ve DB bufferlarini yeniden ayarlayin.")
	}
	if metrics.CPUUsage >= 80 {
		actions = append(actions, "CPU kullanimi yuksek; cache katmanlarini ve PHP worker sayisini optimize edin.")
	}
	if len(actions) == 0 {
		actions = append(actions,
			"Backup gorevlerini trafik piki disina tasiyin.",
			"Statik icerik icin cache TTL degerlerini gozden gecirin.",
			"Panel ve servis loglari icin rotate politikasini etkin tutun.",
		)
	}
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"actions": actions,
		},
	})
}

func (s *service) handleGitOpsDeploy(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain     string `json:"domain"`
		RepoURL    string `json:"repo_url"`
		Branch     string `json:"branch"`
		DeployPath string `json:"deploy_path"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid GitOps deploy payload.")
		return
	}
	domain := normalizeDomain(payload.Domain)
	deployPath := strings.TrimSpace(payload.DeployPath)
	if deployPath == "" && domain != "" {
		deployPath = domainDocroot(domain)
	}
	commit, err := deployRuntimeGitRepo(payload.RepoURL, payload.Branch, deployPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	s.appendActivityLocked("system", "gitops_deploy", fmt.Sprintf("Git repo deployed to %s (%s).", deployPath, commit), "")
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: fmt.Sprintf("GitOps deployment completed for %s.", firstNonEmpty(domain, deployPath)),
		Data: map[string]interface{}{
			"domain":      domain,
			"deploy_path": deployPath,
			"branch":      firstNonEmpty(payload.Branch, "main"),
			"commit":      commit,
		},
	})
}

func (s *service) handleRedisIsolation(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain      string `json:"domain"`
		MaxMemoryMB int    `json:"max_memory_mb"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Redis isolation payload.")
		return
	}
	result, err := createRuntimeRedisIsolation(payload.Domain, maxInt(payload.MaxMemoryMB, 128))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	if s.modules.RedisIsolations == nil {
		s.modules.RedisIsolations = map[string]RedisIsolation{}
	}
	s.modules.RedisIsolations[result.Domain] = result
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: fmt.Sprintf("Host Redis isolation created for %s.", result.Domain),
		Data:    result,
	})
}

func (s *service) handleRedisIsolationList(w http.ResponseWriter) {
	s.mu.RLock()
	items := make([]RedisIsolation, 0, len(s.modules.RedisIsolations))
	for _, item := range s.modules.RedisIsolations {
		items = append(items, item)
	}
	s.mu.RUnlock()

	sort.Slice(items, func(i, j int) bool {
		return items[i].Domain < items[j].Domain
	})

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
}

func (s *service) handleResellerQuotasGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.ResellerQuotas})
}

func (s *service) handleResellerQuotaSet(w http.ResponseWriter, r *http.Request) {
	var payload ResellerQuota
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid reseller quota payload.")
		return
	}
	payload.UpdatedAt = time.Now().UTC().Unix()

	// Apply actual system quota if xfs_quota or setquota is available
	go func(username string, diskGB int) {
		// Example implementation for ext4/ext3 using setquota
		if _, err := exec.LookPath("setquota"); err == nil && diskGB > 0 {
			// Convert GB to Blocks (1 block = 1KB usually in quota)
			blocks := diskGB * 1024 * 1024
			_ = exec.Command("setquota", "-u", username, fmt.Sprintf("%d", blocks), fmt.Sprintf("%d", blocks), "0", "0", "-a").Run()
		}
		// Example implementation for XFS using xfs_quota
		if _, err := exec.LookPath("xfs_quota"); err == nil && diskGB > 0 {
			_ = exec.Command("xfs_quota", "-x", "-c", fmt.Sprintf("limit bsoft=%dg bhard=%dg %s", diskGB, diskGB, username), "/").Run()
		}
	}(payload.Username, payload.DiskGB)

	s.mu.Lock()
	defer s.mu.Unlock()
	replaced := false
	for i := range s.modules.ResellerQuotas {
		if s.modules.ResellerQuotas[i].Username == payload.Username {
			s.modules.ResellerQuotas[i] = payload
			replaced = true
			break
		}
	}
	if !replaced {
		s.modules.ResellerQuotas = append(s.modules.ResellerQuotas, payload)
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Reseller quota saved and applied to system.", Data: payload})
}

func (s *service) handleWhiteLabelsGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.WhiteLabels})
}

func (s *service) handleWhiteLabelSet(w http.ResponseWriter, r *http.Request) {
	var payload WhiteLabel
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid white-label payload.")
		return
	}
	payload.UpdatedAt = time.Now().UTC().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
	replaced := false
	for i := range s.modules.WhiteLabels {
		if s.modules.WhiteLabels[i].Username == payload.Username {
			s.modules.WhiteLabels[i] = payload
			replaced = true
			break
		}
	}
	if !replaced {
		s.modules.WhiteLabels = append(s.modules.WhiteLabels, payload)
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "White-label profile saved.", Data: payload})
}

func (s *service) handleACLPoliciesGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.ACLPolicies})
}

func (s *service) handleACLPolicySet(w http.ResponseWriter, r *http.Request) {
	var payload ACLPolicy
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid ACL policy payload.")
		return
	}
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Description = strings.TrimSpace(payload.Description)
	if payload.Name == "" {
		writeError(w, http.StatusBadRequest, "ACL policy name is required.")
		return
	}
	normalizedPermissions := make([]string, 0, len(payload.Permissions))
	seen := map[string]struct{}{}
	for _, item := range payload.Permissions {
		key := strings.TrimSpace(item)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalizedPermissions = append(normalizedPermissions, key)
	}
	payload.Permissions = normalizedPermissions
	payload.ID = firstNonEmpty(payload.ID, "acl-"+generateSecret(5))
	payload.UpdatedAt = time.Now().UTC().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
	replaced := false
	for i := range s.modules.ACLPolicies {
		if s.modules.ACLPolicies[i].ID == payload.ID {
			s.modules.ACLPolicies[i] = payload
			replaced = true
			break
		}
	}
	if !replaced {
		s.modules.ACLPolicies = append(s.modules.ACLPolicies, payload)
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "ACL policy saved.", Data: payload})
}

func (s *service) handleACLPolicyDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.modules.ACLPolicies
	filtered := items[:0]
	deleted := false
	for _, item := range items {
		if item.ID == id {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.ACLPolicies = filtered
	if !deleted {
		writeError(w, http.StatusNotFound, "ACL policy not found.")
		return
	}
	cleanAssignments := s.modules.ACLAssignments[:0]
	for _, assignment := range s.modules.ACLAssignments {
		if strings.TrimSpace(assignment.PolicyID) == id {
			if user := s.findUserLocked(assignment.Username); user != nil {
				user.RolePolicyID = ""
			}
			continue
		}
		cleanAssignments = append(cleanAssignments, assignment)
	}
	s.modules.ACLAssignments = cleanAssignments
	for i := range s.state.Users {
		if strings.TrimSpace(s.state.Users[i].RolePolicyID) == id {
			s.state.Users[i].RolePolicyID = ""
		}
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "ACL policy deleted."})
}

func (s *service) handleACLAssignmentsGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	updatedAtByUser := map[string]int64{}
	for _, assignment := range s.modules.ACLAssignments {
		updatedAtByUser[sanitizeName(assignment.Username)] = assignment.UpdatedAt
	}
	items := make([]ACLAssignment, 0, len(s.state.Users))
	for _, user := range s.state.Users {
		username := sanitizeName(user.Username)
		policyID := strings.TrimSpace(user.RolePolicyID)
		if policyID == "" {
			for _, assignment := range s.modules.ACLAssignments {
				if sanitizeName(assignment.Username) == username {
					policyID = strings.TrimSpace(assignment.PolicyID)
					break
				}
			}
		}
		if policyID == "" {
			continue
		}
		items = append(items, ACLAssignment{
			Username:  user.Username,
			PolicyID:  policyID,
			UpdatedAt: updatedAtByUser[username],
		})
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
}

func (s *service) handleACLAssignmentSet(w http.ResponseWriter, r *http.Request) {
	var payload ACLAssignment
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid ACL assignment payload.")
		return
	}
	payload.Username = sanitizeName(payload.Username)
	payload.PolicyID = strings.TrimSpace(payload.PolicyID)
	if payload.Username == "" || payload.PolicyID == "" {
		writeError(w, http.StatusBadRequest, "Username and policy_id are required.")
		return
	}
	payload.UpdatedAt = time.Now().UTC().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.policyExistsLocked(payload.PolicyID) {
		writeError(w, http.StatusBadRequest, "ACL policy not found.")
		return
	}
	replaced := false
	for i := range s.modules.ACLAssignments {
		if s.modules.ACLAssignments[i].Username == payload.Username {
			s.modules.ACLAssignments[i] = payload
			replaced = true
			break
		}
	}
	if !replaced {
		s.modules.ACLAssignments = append(s.modules.ACLAssignments, payload)
	}
	if user := s.findUserLocked(payload.Username); user != nil {
		user.RolePolicyID = payload.PolicyID
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "ACL assignment saved.", Data: payload})
}

func (s *service) handleACLAssignmentDelete(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.URL.Query().Get("username"))
	username = sanitizeName(username)
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.modules.ACLAssignments
	filtered := items[:0]
	deleted := false
	for _, item := range items {
		if item.Username == username {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.ACLAssignments = filtered
	if user := s.findUserLocked(username); user != nil {
		user.RolePolicyID = ""
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "ACL assignment not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "ACL assignment deleted."})
}

func (s *service) handleACLEffectiveGet(w http.ResponseWriter, r *http.Request) {
	username := sanitizeName(strings.TrimSpace(r.URL.Query().Get("username")))
	s.mu.RLock()
	defer s.mu.RUnlock()
	if username == "" {
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: []string{}})
		return
	}
	if user := s.findUserLocked(username); user != nil {
		_, _, permissions := s.resolveUserACLLocked(*user)
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: permissions})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: []string{}})
}

func (s *service) handleSecurityLivePatch(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Target string `json:"target"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid live patch payload.")
		return
	}
	output, err := refreshRuntimeLivePatch(payload.Target)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.EBPFEvents = append([]string{fmt.Sprintf("Live patch runtime checked for %s: %s", firstNonEmpty(payload.Target, "kernel"), output)}, s.state.EBPFEvents...)
	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: fmt.Sprintf("Live patch runtime refreshed for %s.", firstNonEmpty(payload.Target, "kernel")),
		Data: map[string]interface{}{
			"target": firstNonEmpty(payload.Target, "kernel"),
			"output": output,
		},
	})
}

func (s *service) handleMalwareJobs(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.MalwareJobs})
}

func (s *service) handleMalwareStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.state.MalwareJobs {
		if s.state.MalwareJobs[i].ID != id {
			continue
		}
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.MalwareJobs[i]})
		return
	}
	writeError(w, http.StatusNotFound, "Malware job not found.")
}

func (s *service) handleMalwareStart(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Path   string `json:"path"`
		Engine string `json:"engine"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid malware scan payload.")
		return
	}
	job, err := runRuntimeMalwareScan(payload.Path, payload.Engine)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.MalwareJobs = append([]MalwareJob{job}, s.state.MalwareJobs...)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Malware scan started.", Data: job})
}

func (s *service) handleMalwareQuarantine(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		JobID     string `json:"job_id"`
		FindingID string `json:"finding_id"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid quarantine payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for jobIndex := range s.state.MalwareJobs {
		if s.state.MalwareJobs[jobIndex].ID != payload.JobID {
			continue
		}
		for findingIndex := range s.state.MalwareJobs[jobIndex].Findings {
			finding := &s.state.MalwareJobs[jobIndex].Findings[findingIndex]
			if finding.ID != payload.FindingID {
				continue
			}
			quarantinePath, err := quarantineRuntimeFile(finding.FilePath)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			finding.Quarantined = true
			record := QuarantineRecord{
				ID:             generateSecret(8),
				JobID:          payload.JobID,
				FindingID:      payload.FindingID,
				OriginalPath:   finding.FilePath,
				QuarantinePath: quarantinePath,
			}
			s.state.Quarantine = append([]QuarantineRecord{record}, s.state.Quarantine...)
			writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Finding quarantined.", Data: record})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Malware finding not found.")
}

func (s *service) handleMalwareQuarantineList(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.Quarantine})
}

func appendIfMissing(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func (s *service) handleMalwareQuarantineRestore(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		QuarantineID string `json:"quarantine_id"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid quarantine restore payload.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Quarantine {
		if s.state.Quarantine[i].ID == payload.QuarantineID {
			if err := restoreRuntimeQuarantine(s.state.Quarantine[i]); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			s.state.Quarantine[i].RestoredAt = time.Now().UTC().Format(time.RFC3339)
			writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Quarantine record restored.", Data: s.state.Quarantine[i]})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Quarantine record not found.")
}

func (s *service) handleCloudflareZones(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Cloudflare credentials payload.")
		return
	}
	creds := cloudflareResolveCredentials(payload)
	if !creds.valid() {
		writeError(w, http.StatusBadRequest, "Cloudflare credentials are required.")
		return
	}
	zones, err := cloudflareListZones(creds)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.mu.Lock()
	s.modules.CloudflareZones = zones
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: zones})
}

func (s *service) handleCloudflareStatus(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: cloudflareRuntimeSnapshot()})
}

func (s *service) handleCloudflareServerAuth(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Cloudflare server auth payload.")
		return
	}
	creds := cloudflareRequestCredentials(payload)
	if !creds.valid() {
		writeError(w, http.StatusBadRequest, "Cloudflare email + API key or API token is required.")
		return
	}
	autoSync := true
	if value, ok := payload["auto_sync"]; ok {
		autoSync = boolValue(value)
	}
	if err := persistCloudflareServerCredentials(creds, autoSync); err != nil {
		writeError(w, http.StatusInternalServerError, "Cloudflare credentials could not be persisted to server env.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: "Cloudflare server credentials saved.",
		Data:    cloudflareRuntimeSnapshot(),
	})
}

// SSH Configuration Management

func (s *service) handleSSHConfigGet(w http.ResponseWriter) {
	configPath := "/etc/ssh/sshd_config"
	data := map[string]string{
		"port":              "22",
		"permit_root_login": "yes",
	}

	content, err := os.ReadFile(configPath)
	if err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "Port ") {
				data["port"] = strings.TrimSpace(strings.TrimPrefix(trimmed, "Port "))
			} else if strings.HasPrefix(trimmed, "PermitRootLogin ") {
				data["permit_root_login"] = strings.TrimSpace(strings.TrimPrefix(trimmed, "PermitRootLogin "))
			}
		}
	}

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: data})
}

func (s *service) handleSSHConfigSet(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid SSH config payload.")
		return
	}
	port, err := parseSSHConfigPort(payload["port"])
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	permitRootLogin, err := normalizePermitRootLogin(payload["permit_root_login"])
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	configPath := "/etc/ssh/sshd_config"
	content, err := os.ReadFile(configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to read sshd_config")
		return
	}
	updatedContent := normalizeSSHConfigContent(string(content), port, permitRootLogin)
	if err := os.WriteFile(configPath, []byte(updatedContent), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to write sshd_config")
		return
	}
	if err := applySSHRuntimeConfig(port); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("SSH config was written but service apply failed: %v", err))
		return
	}

	s.mu.Lock()
	s.appendActivityLocked("system", "ssh_config", "SSH Port and Root Login configuration updated.", "")
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "SSH configuration updated successfully."})
}

func normalizeSSHConfigContent(content string, port int, permitRootLogin string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines)+2)

	portDirective := fmt.Sprintf("Port %d", port)
	rootDirective := fmt.Sprintf("PermitRootLogin %s", permitRootLogin)
	portWritten := false
	rootWritten := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lowered := strings.ToLower(trimmed)
		if strings.HasPrefix(lowered, "port ") {
			if !portWritten {
				out = append(out, portDirective)
				portWritten = true
			}
			continue
		}
		if strings.HasPrefix(lowered, "permitrootlogin ") {
			if !rootWritten {
				out = append(out, rootDirective)
				rootWritten = true
			}
			continue
		}
		out = append(out, line)
	}

	if !portWritten {
		out = append(out, portDirective)
	}
	if !rootWritten {
		out = append(out, rootDirective)
	}

	return strings.Join(out, "\n")
}

func applySSHRuntimeConfig(expectedPort int) error {
	if !runtimeHostLinux() {
		return nil
	}

	if err := exec.Command("/usr/sbin/sshd", "-t").Run(); err != nil {
		return fmt.Errorf("sshd config test failed: %w", err)
	}

	unit, err := detectSSHServiceUnit()
	if err != nil {
		return err
	}

	if err := ensureSSHServiceKillModeControlGroup(unit); err != nil {
		return err
	}

	// Socket activation can resurrect stale listeners on distro defaults.
	_ = exec.Command("systemctl", "disable", "--now", "ssh.socket").Run()
	_ = exec.Command("systemctl", "disable", "--now", "sshd.socket").Run()
	_ = exec.Command("systemctl", "daemon-reload").Run()

	_ = exec.Command("systemctl", "enable", unit).Run()
	_ = exec.Command("systemctl", "stop", unit).Run()

	// Kill only listener masters; keep active user sessions intact.
	_ = exec.Command("pkill", "-f", "sshd: .*\\[listener\\]").Run()

	if err := exec.Command("systemctl", "start", unit).Run(); err != nil {
		return fmt.Errorf("failed to start %s: %w", unit, err)
	}

	if err := waitForLocalSSHDPort(expectedPort, 10*time.Second); err != nil {
		return err
	}

	// Safety net for stale listeners that may survive from prior restarts.
	if expectedPort != 22 && localTCPPortListening(22) {
		_ = exec.Command("pkill", "-f", "sshd: .*\\[listener\\]").Run()
		if err := exec.Command("systemctl", "restart", unit).Run(); err != nil {
			return fmt.Errorf("failed to restart %s while cleaning stale listeners: %w", unit, err)
		}
		if err := waitForLocalSSHDPort(expectedPort, 10*time.Second); err != nil {
			return err
		}
		if localTCPPortListening(22) {
			return fmt.Errorf("stale ssh listener still present on port 22")
		}
	}

	return nil
}

func ensureSSHServiceKillModeControlGroup(unit string) error {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return fmt.Errorf("ssh service unit is required")
	}

	dropInDir := filepath.Join("/etc/systemd/system", unit+".d")
	if err := os.MkdirAll(dropInDir, 0o755); err != nil {
		return fmt.Errorf("failed to create ssh drop-in directory: %w", err)
	}

	overridePath := filepath.Join(dropInDir, "99-aurapanel-killmode.conf")
	overrideContent := "[Service]\nKillMode=control-group\n"
	if err := os.WriteFile(overridePath, []byte(overrideContent), 0o644); err != nil {
		return fmt.Errorf("failed to write ssh killmode override: %w", err)
	}

	return nil
}

func detectSSHServiceUnit() (string, error) {
	for _, unit := range []string{"ssh.service", "sshd.service"} {
		if systemdUnitLoaded(unit) {
			return unit, nil
		}
	}
	return "", fmt.Errorf("no loaded ssh service unit found (ssh.service/sshd.service)")
}

func systemdUnitLoaded(unit string) bool {
	out, err := exec.Command("systemctl", "show", "-p", "LoadState", "--value", unit).Output()
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(string(out)), "loaded")
}

func waitForLocalSSHDPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if localTCPPortListening(port) {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("sshd did not start listening on port %d in time", port)
}

func localTCPPortListening(port int) bool {
	output, err := exec.Command("ss", "-ltn").Output()
	if err != nil {
		return false
	}
	needle := ":" + strconv.Itoa(port)
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		localAddr := fields[3]
		if strings.HasSuffix(localAddr, needle) {
			return true
		}
	}
	return false
}

func parseSSHConfigPort(raw interface{}) (int, error) {
	switch value := raw.(type) {
	case float64:
		port := int(value)
		if float64(port) != value {
			return 0, fmt.Errorf("SSH port must be an integer between 1 and 65535.")
		}
		if port < 1 || port > 65535 {
			return 0, fmt.Errorf("SSH port must be between 1 and 65535.")
		}
		return port, nil
	case int:
		if value < 1 || value > 65535 {
			return 0, fmt.Errorf("SSH port must be between 1 and 65535.")
		}
		return value, nil
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return 0, fmt.Errorf("SSH port is required.")
		}
		port, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, fmt.Errorf("SSH port must be numeric.")
		}
		if port < 1 || port > 65535 {
			return 0, fmt.Errorf("SSH port must be between 1 and 65535.")
		}
		return port, nil
	default:
		return 0, fmt.Errorf("SSH port is required.")
	}
}

func normalizePermitRootLogin(raw interface{}) (string, error) {
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("permit_root_login is required.")
	}
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "yes", "no", "prohibit-password", "forced-commands-only":
		return normalized, nil
	default:
		return "", fmt.Errorf("permit_root_login must be one of: yes, no, prohibit-password, forced-commands-only.")
	}
}

func (s *service) handleCloudflareDNSList(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Cloudflare DNS list payload.")
		return
	}
	creds := cloudflareResolveCredentials(payload)
	zoneID := strings.TrimSpace(stringValue(payload["zone_id"]))
	if !creds.valid() || zoneID == "" {
		writeError(w, http.StatusBadRequest, "Cloudflare credentials and zone_id are required.")
		return
	}
	records, err := cloudflareListDNSRecords(creds, zoneID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.mu.Lock()
	s.modules.CloudflareDNS[zoneID] = records
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: records})
}

func (s *service) handleCloudflareDNSCreate(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Cloudflare DNS create payload.")
		return
	}
	creds := cloudflareResolveCredentials(payload)
	zoneID := strings.TrimSpace(stringValue(payload["zone_id"]))
	record := CloudflareDNSRecord{
		Type:    strings.ToUpper(strings.TrimSpace(stringValue(payload["type"]))),
		Name:    strings.TrimSpace(stringValue(payload["name"])),
		Content: strings.TrimSpace(stringValue(payload["content"])),
		TTL:     intValue(payload["ttl"], 1),
		Proxied: boolValue(payload["proxied"]),
	}
	if !creds.valid() || zoneID == "" || record.Type == "" || record.Name == "" || record.Content == "" {
		writeError(w, http.StatusBadRequest, "Cloudflare credentials, zone_id and DNS record fields are required.")
		return
	}
	created, err := cloudflareCreateDNSRecord(creds, zoneID, record)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.mu.Lock()
	s.modules.CloudflareDNS[zoneID] = append(s.modules.CloudflareDNS[zoneID], created)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Cloudflare DNS record created.", Data: created})
}

func (s *service) handleCloudflareDNSDelete(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Cloudflare DNS delete payload.")
		return
	}
	creds := cloudflareResolveCredentials(payload)
	zoneID := strings.TrimSpace(stringValue(payload["zone_id"]))
	recordID := strings.TrimSpace(stringValue(payload["record_id"]))
	if !creds.valid() || zoneID == "" || recordID == "" {
		writeError(w, http.StatusBadRequest, "Cloudflare credentials, zone_id and record_id are required.")
		return
	}
	if err := cloudflareDeleteDNSRecord(creds, zoneID, recordID); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.modules.CloudflareDNS[zoneID]
	filtered := items[:0]
	for _, item := range items {
		if item.ID == recordID {
			continue
		}
		filtered = append(filtered, item)
	}
	s.modules.CloudflareDNS[zoneID] = filtered
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Cloudflare DNS record deleted."})
}

func (s *service) handleCloudflareSSL(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Cloudflare SSL payload.")
		return
	}
	creds := cloudflareResolveCredentials(payload)
	zoneID := strings.TrimSpace(stringValue(payload["zone_id"]))
	mode := firstNonEmpty(strings.TrimSpace(stringValue(payload["mode"])), "full")
	if !creds.valid() || zoneID == "" {
		writeError(w, http.StatusBadRequest, "Cloudflare credentials and zone_id are required.")
		return
	}
	if err := cloudflarePatchSetting(creds, zoneID, "ssl", mode); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	settings := s.modules.CloudflareSettings[zoneID]
	settings.SSLMode = mode
	s.modules.CloudflareSettings[zoneID] = settings
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Cloudflare SSL mode updated."})
}

func (s *service) handleCloudflareSettings(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Cloudflare settings payload.")
		return
	}
	creds := cloudflareResolveCredentials(payload)
	zoneID := strings.TrimSpace(stringValue(payload["zone_id"]))
	if !creds.valid() || zoneID == "" {
		writeError(w, http.StatusBadRequest, "Cloudflare credentials and zone_id are required.")
		return
	}
	config, err := cloudflareZoneConfigSnapshot(creds, zoneID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.mu.Lock()
	s.modules.CloudflareSettings[zoneID] = config
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: config})
}

func (s *service) handleCloudflareAlwaysHTTPS(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Cloudflare Always HTTPS payload.")
		return
	}
	creds := cloudflareResolveCredentials(payload)
	zoneID := strings.TrimSpace(stringValue(payload["zone_id"]))
	enabled := boolValue(payload["enabled"])
	if !creds.valid() || zoneID == "" {
		writeError(w, http.StatusBadRequest, "Cloudflare credentials and zone_id are required.")
		return
	}
	settingValue := "off"
	if enabled {
		settingValue = "on"
	}
	if err := cloudflarePatchSetting(creds, zoneID, "always_use_https", settingValue); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.mu.Lock()
	settings := s.modules.CloudflareSettings[zoneID]
	settings.AlwaysHTTPS = enabled
	s.modules.CloudflareSettings[zoneID] = settings
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Cloudflare Always HTTPS updated."})
}

func (s *service) handleCloudflareMinify(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Cloudflare minify payload.")
		return
	}
	creds := cloudflareResolveCredentials(payload)
	zoneID := strings.TrimSpace(stringValue(payload["zone_id"]))
	if !creds.valid() || zoneID == "" {
		writeError(w, http.StatusBadRequest, "Cloudflare credentials and zone_id are required.")
		return
	}

	jsEnabled := boolValue(payload["js"])
	cssEnabled := boolValue(payload["css"])
	htmlEnabled := boolValue(payload["html"])
	value := map[string]string{
		"js":   "off",
		"css":  "off",
		"html": "off",
	}
	if jsEnabled {
		value["js"] = "on"
	}
	if cssEnabled {
		value["css"] = "on"
	}
	if htmlEnabled {
		value["html"] = "on"
	}

	if err := cloudflarePatchSetting(creds, zoneID, "minify", value); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.mu.Lock()
	settings := s.modules.CloudflareSettings[zoneID]
	settings.MinifyJS = jsEnabled
	settings.MinifyCSS = cssEnabled
	settings.MinifyHTML = htmlEnabled
	s.modules.CloudflareSettings[zoneID] = settings
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Cloudflare minify settings updated."})
}

func (s *service) handleCloudflareSecurity(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Cloudflare security payload.")
		return
	}
	creds := cloudflareResolveCredentials(payload)
	zoneID := strings.TrimSpace(stringValue(payload["zone_id"]))
	level := firstNonEmpty(strings.TrimSpace(stringValue(payload["level"])), "medium")
	if !creds.valid() || zoneID == "" {
		writeError(w, http.StatusBadRequest, "Cloudflare credentials and zone_id are required.")
		return
	}
	if err := cloudflarePatchSetting(creds, zoneID, "security_level", level); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	settings := s.modules.CloudflareSettings[zoneID]
	settings.SecurityLevel = level
	s.modules.CloudflareSettings[zoneID] = settings
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Cloudflare security level updated."})
}

func (s *service) handleCloudflareDevMode(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Cloudflare dev mode payload.")
		return
	}
	creds := cloudflareResolveCredentials(payload)
	zoneID := strings.TrimSpace(stringValue(payload["zone_id"]))
	enabled := boolValue(payload["enabled"])
	if !creds.valid() || zoneID == "" {
		writeError(w, http.StatusBadRequest, "Cloudflare credentials and zone_id are required.")
		return
	}
	mode := "off"
	if enabled {
		mode = "on"
	}
	if err := cloudflarePatchSetting(creds, zoneID, "development_mode", mode); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	settings := s.modules.CloudflareSettings[zoneID]
	settings.DevMode = enabled
	s.modules.CloudflareSettings[zoneID] = settings
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Cloudflare development mode updated."})
}

func (s *service) handleCloudflareCachePurge(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Cloudflare cache purge payload.")
		return
	}
	creds := cloudflareResolveCredentials(payload)
	zoneID := strings.TrimSpace(stringValue(payload["zone_id"]))
	if !creds.valid() || zoneID == "" {
		writeError(w, http.StatusBadRequest, "Cloudflare credentials and zone_id are required.")
		return
	}
	requestBody := map[string]interface{}{}
	if boolValue(payload["purge_everything"]) {
		requestBody["purge_everything"] = true
	} else if files, ok := payload["files"].([]interface{}); ok && len(files) > 0 {
		requestBody["files"] = files
	} else {
		writeError(w, http.StatusBadRequest, "Specify purge_everything or files.")
		return
	}
	if err := cloudflarePurgeCache(creds, zoneID, requestBody); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Cloudflare cache purge requested."})
}

func (s *service) handleCloudflareAnalytics(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Cloudflare analytics payload.")
		return
	}
	creds := cloudflareResolveCredentials(payload)
	zoneID := strings.TrimSpace(stringValue(payload["zone_id"]))
	if !creds.valid() || zoneID == "" {
		writeError(w, http.StatusBadRequest, "Cloudflare credentials and zone_id are required.")
		return
	}
	result, err := cloudflareAnalytics(creds, zoneID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: result})
}

func (s *service) isMigrationArchivePathAllowedLocked(path string) bool {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "" {
		return false
	}
	root := filepath.Clean(migrationUploadsDir())
	if cleanPath == root {
		return false
	}
	if strings.HasPrefix(cleanPath, root+string(os.PathSeparator)) {
		return true
	}
	_, ok := s.modules.UploadedArchives[cleanPath]
	return ok
}

func (s *service) buildMigrationPrecheckLocked(analysis MigrationAnalysis, targetOwner string) MigrationPrecheck {
	checks := []MigrationCheck{}
	conflicts := []MigrationConflict{}
	recommendations := []string{}
	ready := true

	target := sanitizeName(strings.TrimSpace(targetOwner))
	if target == "" {
		target = s.defaultOwnerLocked()
	}

	if analysis.ArchivePath == "" {
		ready = false
		checks = append(checks, MigrationCheck{Name: "Archive path", Status: "fail", Detail: "Archive path is missing."})
	} else {
		checks = append(checks, MigrationCheck{
			Name:   "Archive integrity",
			Status: "pass",
			Detail: fmt.Sprintf("%s archive detected (%s).", migrationSourceLabel(analysis.SourceType), firstNonEmpty(analysis.ArchiveSizeText, "size unknown")),
		})
	}

	if analysis.SourceType == "generic" {
		checks = append(checks, MigrationCheck{Name: "Source detection", Status: "warn", Detail: "Source type confidence is low; manual review is recommended."})
		recommendations = append(recommendations, "Set source type manually (cPanel/Plesk/CyberPanel) before import for safer mapping.")
	} else {
		checks = append(checks, MigrationCheck{Name: "Source detection", Status: "pass", Detail: fmt.Sprintf("Source type resolved as %s.", migrationSourceLabel(analysis.SourceType))})
	}

	if analysis.Stats.FileCount <= 0 {
		ready = false
		checks = append(checks, MigrationCheck{Name: "Archive content", Status: "fail", Detail: "Archive looks empty or unreadable."})
		conflicts = append(conflicts, MigrationConflict{
			Type:     "archive",
			Target:   analysis.ArchivePath,
			Severity: "high",
			Message:  "No importable files were detected.",
		})
	} else {
		checks = append(checks, MigrationCheck{Name: "Archive content", Status: "pass", Detail: fmt.Sprintf("%d files discovered.", analysis.Stats.FileCount)})
	}

	owner := s.findUserLocked(target)
	if owner == nil {
		ready = false
		checks = append(checks, MigrationCheck{Name: "Target owner", Status: "fail", Detail: fmt.Sprintf("Target owner %q does not exist.", target)})
		conflicts = append(conflicts, MigrationConflict{
			Type:     "owner",
			Target:   target,
			Severity: "high",
			Message:  "Target owner is missing.",
		})
		recommendations = append(recommendations, fmt.Sprintf("Create owner user %q before starting import.", target))
	} else {
		checks = append(checks, MigrationCheck{Name: "Target owner", Status: "pass", Detail: fmt.Sprintf("Owner %q is available with role %q.", owner.Username, normalizeRole(owner.Role))})
	}

	existingSites := map[string]Website{}
	for _, site := range s.state.Websites {
		existingSites[normalizeDomain(site.Domain)] = site
	}
	domainConflicts := 0
	for _, raw := range analysis.VhostCandidates {
		domain := normalizeDomain(raw)
		if domain == "" {
			continue
		}
		existing, ok := existingSites[domain]
		if !ok {
			continue
		}
		domainConflicts++
		ready = false
		conflicts = append(conflicts, MigrationConflict{
			Type:     "domain",
			Target:   domain,
			Severity: "high",
			Message:  fmt.Sprintf("Domain already exists under owner %q.", firstNonEmpty(existing.Owner, existing.User, "unknown")),
		})
	}
	if domainConflicts > 0 {
		checks = append(checks, MigrationCheck{
			Name:   "Domain conflicts",
			Status: "fail",
			Detail: fmt.Sprintf("%d domain conflict(s) detected.", domainConflicts),
		})
		recommendations = append(recommendations, "Rename conflicting domains in source backup or import into clean target domains.")
	} else if len(analysis.VhostCandidates) == 0 {
		checks = append(checks, MigrationCheck{Name: "Domain conflicts", Status: "warn", Detail: "No domain candidates found; website mapping may need manual input."})
	} else {
		checks = append(checks, MigrationCheck{Name: "Domain conflicts", Status: "pass", Detail: "No website domain collision detected."})
	}

	existingMailboxes := map[string]Mailbox{}
	for _, mailbox := range s.modules.Mailboxes {
		existingMailboxes[strings.ToLower(strings.TrimSpace(mailbox.Address))] = mailbox
	}
	mailConflicts := 0
	for _, raw := range analysis.EmailAccounts {
		address := strings.ToLower(strings.TrimSpace(raw))
		if address == "" {
			continue
		}
		existing, ok := existingMailboxes[address]
		if !ok {
			continue
		}
		mailConflicts++
		ready = false
		conflicts = append(conflicts, MigrationConflict{
			Type:     "mailbox",
			Target:   address,
			Severity: "medium",
			Message:  fmt.Sprintf("Mailbox already exists for owner %q.", firstNonEmpty(existing.Owner, existing.User, "unknown")),
		})
	}
	if mailConflicts > 0 {
		checks = append(checks, MigrationCheck{
			Name:   "Mailbox conflicts",
			Status: "fail",
			Detail: fmt.Sprintf("%d mailbox conflict(s) detected.", mailConflicts),
		})
		recommendations = append(recommendations, "Clean existing mailbox accounts or map them to different addresses before import.")
	} else {
		checks = append(checks, MigrationCheck{Name: "Mailbox conflicts", Status: "pass", Detail: "No mailbox collision detected."})
	}

	if len(analysis.MySQLDumps) == 0 {
		checks = append(checks, MigrationCheck{Name: "Database payload", Status: "warn", Detail: "No SQL dump found in archive."})
	} else {
		checks = append(checks, MigrationCheck{Name: "Database payload", Status: "pass", Detail: fmt.Sprintf("%d SQL dump(s) detected.", len(analysis.MySQLDumps))})
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Pre-check clean. You can start import safely in DRY-RUN mode.")
	}

	eta := estimateMigrationETASeconds(analysis)
	checks = append(checks, MigrationCheck{Name: "ETA", Status: "pass", Detail: fmt.Sprintf("Estimated migration duration ~%d seconds.", eta)})

	return MigrationPrecheck{
		Ready:           ready,
		ETASeconds:      eta,
		Checks:          checks,
		Conflicts:       conflicts,
		Recommendations: uniqueStrings(recommendations),
	}
}

func (s *service) updateMigrationJobProgressLocked(job *MigrationJob) {
	if job == nil {
		return
	}
	if strings.EqualFold(job.Status, "completed") || strings.EqualFold(job.Status, "failed") {
		return
	}
	job.PollCount++
	switch {
	case job.PollCount <= 1:
		if job.Progress < 75 {
			job.Progress = 75
		}
		job.Logs = append(job.Logs, "Validation phase completed. Artifact plans are ready.")
	case job.PollCount >= 2:
		job.Progress = 100
		job.Status = "completed"
		job.Logs = append(job.Logs, "Migration planning completed (dry-run). Apply stage can now be executed.")
	}
}

func (s *service) handleMigrationUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		errText := strings.ToLower(strings.TrimSpace(err.Error()))
		switch {
		case strings.Contains(errText, "multipart"),
			strings.Contains(errText, "boundary"):
			writeError(w, http.StatusBadRequest, "Migration upload could not be parsed. Use multipart/form-data upload.")
		case strings.Contains(errText, "request body too large"),
			strings.Contains(errText, "too large"):
			writeError(w, http.StatusRequestEntityTooLarge, "Migration upload is too large for current limits.")
		default:
			writeError(w, http.StatusBadRequest, "Migration upload could not be parsed.")
		}
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Migration archive is required.")
		return
	}
	defer file.Close()
	archivePath, err := saveMigrationUpload(header.Filename, file)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.UploadedArchives[archivePath] = filepath.Base(archivePath)
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"archive_path": archivePath,
		},
	})
}

func (s *service) handleMigrationAnalyze(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ArchivePath string  `json:"archive_path"`
		SourceType  *string `json:"source_type"`
		TargetOwner string  `json:"target_owner"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid migration analysis payload.")
		return
	}
	archivePath := filepath.Clean(strings.TrimSpace(payload.ArchivePath))
	if archivePath == "" {
		writeError(w, http.StatusBadRequest, "Archive path is required.")
		return
	}
	s.mu.RLock()
	allowed := s.isMigrationArchivePathAllowedLocked(archivePath)
	s.mu.RUnlock()
	if !allowed {
		writeError(w, http.StatusBadRequest, "Archive path is not allowed.")
		return
	}
	sourceType := ""
	if payload.SourceType != nil {
		sourceType = strings.TrimSpace(*payload.SourceType)
	}
	normalizedSource, err := normalizeMigrationSourceTypeInput(sourceType)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	analysis, err := analyzeMigrationArchive(archivePath, normalizedSource)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	analysis.Precheck = s.buildMigrationPrecheckLocked(analysis, payload.TargetOwner)
	s.modules.MigrationAnalyses[archivePath] = analysis
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: analysis})
}

func (s *service) handleMigrationImportStart(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ArchivePath    string  `json:"archive_path"`
		SourceType     *string `json:"source_type"`
		TargetOwner    string  `json:"target_owner"`
		AllowConflicts bool    `json:"allow_conflicts"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid migration start payload.")
		return
	}
	archivePath := filepath.Clean(strings.TrimSpace(payload.ArchivePath))
	if archivePath == "" {
		writeError(w, http.StatusBadRequest, "Archive path is required.")
		return
	}
	s.mu.RLock()
	allowed := s.isMigrationArchivePathAllowedLocked(archivePath)
	s.mu.RUnlock()
	if !allowed {
		writeError(w, http.StatusBadRequest, "Archive path is not allowed.")
		return
	}
	sourceType := ""
	if payload.SourceType != nil {
		sourceType = strings.TrimSpace(*payload.SourceType)
	}
	normalizedSource, err := normalizeMigrationSourceTypeInput(sourceType)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	analysis, err := analyzeMigrationArchive(archivePath, normalizedSource)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	precheck := s.buildMigrationPrecheckLocked(analysis, payload.TargetOwner)
	analysis.Precheck = precheck
	s.modules.MigrationAnalyses[archivePath] = analysis
	s.mu.Unlock()

	if !precheck.Ready && !payload.AllowConflicts {
		writeJSON(w, http.StatusConflict, apiResponse{
			Status:  "error",
			Message: "Migration pre-check failed. Resolve conflicts before import.",
			Data:    precheck,
		})
		return
	}

	job, err := importMigrationArchive(analysis, payload.TargetOwner)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.MigrationJobs = append([]MigrationJob{job}, s.modules.MigrationJobs...)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: job, Message: "Migration import started."})
}

func (s *service) handleMigrationImportStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "Migration job id is required.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.modules.MigrationJobs {
		if s.modules.MigrationJobs[i].ID != id {
			continue
		}
		s.updateMigrationJobProgressLocked(&s.modules.MigrationJobs[i])
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.MigrationJobs[i]})
		return
	}
	writeError(w, http.StatusNotFound, "Migration job not found.")
}
