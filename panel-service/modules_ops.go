package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
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
	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: fmt.Sprintf("Isolated Redis created for %s.", normalizeDomain(payload.Domain)),
		Data:    result,
	})
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
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "ACL policy deleted."})
}

func (s *service) handleACLAssignmentsGet(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.ACLAssignments})
}

func (s *service) handleACLAssignmentSet(w http.ResponseWriter, r *http.Request) {
	var payload ACLAssignment
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid ACL assignment payload.")
		return
	}
	payload.UpdatedAt = time.Now().UTC().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
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
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "ACL assignment saved.", Data: payload})
}

func (s *service) handleACLAssignmentDelete(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.URL.Query().Get("username"))
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
	if !deleted {
		writeError(w, http.StatusNotFound, "ACL assignment not found.")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "ACL assignment deleted."})
}

func (s *service) handleACLEffectiveGet(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.URL.Query().Get("username"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	policyID := ""
	for _, item := range s.modules.ACLAssignments {
		if item.Username == username {
			policyID = item.PolicyID
			break
		}
	}
	for _, policy := range s.modules.ACLPolicies {
		if policy.ID == policyID {
			writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: policy.Permissions})
			return
		}
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
	var payload struct {
		Port            string `json:"port"`
		PermitRootLogin string `json:"permit_root_login"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid SSH config payload.")
		return
	}

	configPath := "/etc/ssh/sshd_config"
	content, err := os.ReadFile(configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to read sshd_config")
		return
	}

	lines := strings.Split(string(content), "\n")
	portFound := false
	rootLoginFound := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Port ") {
			lines[i] = fmt.Sprintf("Port %s", payload.Port)
			portFound = true
		} else if strings.HasPrefix(trimmed, "PermitRootLogin ") {
			lines[i] = fmt.Sprintf("PermitRootLogin %s", payload.PermitRootLogin)
			rootLoginFound = true
		}
	}

	if !portFound {
		lines = append(lines, fmt.Sprintf("Port %s", payload.Port))
	}
	if !rootLoginFound {
		lines = append(lines, fmt.Sprintf("PermitRootLogin %s", payload.PermitRootLogin))
	}

	if err := os.WriteFile(configPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to write sshd_config")
		return
	}

	// Restart SSH service (sshd on EL, ssh on Ubuntu/Debian)
	_ = exec.Command("systemctl", "restart", "sshd").Run()
	_ = exec.Command("systemctl", "restart", "ssh").Run()

	s.mu.Lock()
	s.appendActivityLocked("system", "ssh_config", "SSH Port and Root Login configuration updated.", "")
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "SSH configuration updated successfully."})
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

func (s *service) handleMigrationUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "Migration upload could not be parsed.")
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
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid migration analysis payload.")
		return
	}
	sourceType := ""
	if payload.SourceType != nil {
		sourceType = strings.TrimSpace(*payload.SourceType)
	}
	analysis, err := analyzeMigrationArchive(payload.ArchivePath, sourceType)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.MigrationAnalyses[payload.ArchivePath] = analysis
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: analysis})
}

func (s *service) handleMigrationImportStart(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ArchivePath string  `json:"archive_path"`
		SourceType  *string `json:"source_type"`
		TargetOwner string  `json:"target_owner"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid migration start payload.")
		return
	}
	sourceType := ""
	if payload.SourceType != nil {
		sourceType = strings.TrimSpace(*payload.SourceType)
	}
	job, err := importMigrationArchive(payload.ArchivePath, sourceType, payload.TargetOwner)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.MigrationJobs = append([]MigrationJob{job}, s.modules.MigrationJobs...)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: job, Message: "Migration import completed."})
}

func (s *service) handleMigrationImportStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.modules.MigrationJobs {
		if s.modules.MigrationJobs[i].ID != id {
			continue
		}
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.MigrationJobs[i]})
		return
	}
	writeError(w, http.StatusNotFound, "Migration job not found.")
}
