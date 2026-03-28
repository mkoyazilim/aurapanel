package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (s *service) handleSREPrediction(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"prediction": "Traffic profile is healthy. Next pressure point is disk-bound backup windows, not CPU saturation.",
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
	answer := map[string]interface{}{
		"answer":          fmt.Sprintf("Query `%s` matched recent access/error samples in Go simulation mode.", payload.Query),
		"confidence":      0.87,
		"matched_sources": []string{"openlitespeed.access", "panel-service.activity", "mariadb.slowlog"},
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: answer})
}

func (s *service) handleSREOptimize(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"actions": []string{
				"Shift nightly backups away from the panel peak window.",
				"Pin Redis memory ceiling per isolated instance.",
				"Promote static cache TTL to 7200s for brochure sites.",
			},
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
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("GitOps deployment queued for %s (%s@%s).", payload.Domain, payload.RepoURL, firstNonEmpty(payload.Branch, "main"))})
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
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("Isolated Redis planned for %s with %d MB max memory.", payload.Domain, maxInt(payload.MaxMemoryMB, 128))})
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
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: "Reseller quota saved.", Data: payload})
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.EBPFEvents = append([]string{fmt.Sprintf("Live patch prepared for %s", firstNonEmpty(payload.Target, "kernel"))}, s.state.EBPFEvents...)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("Live patch scheduled for %s.", firstNonEmpty(payload.Target, "kernel"))})
}

func (s *service) handleMalwareJobs(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.state.MalwareJobs})
}

func (s *service) handleMalwareStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.MalwareJobs {
		if s.state.MalwareJobs[i].ID != id {
			continue
		}
		if s.state.MalwareJobs[i].Progress < 100 {
			s.state.MalwareJobs[i].Progress = minInt(100, s.state.MalwareJobs[i].Progress+35)
			if s.state.MalwareJobs[i].Progress >= 100 {
				s.state.MalwareJobs[i].Status = "completed"
			}
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
	job := MalwareJob{
		ID:            generateSecret(8),
		Status:        "running",
		Progress:      25,
		InfectedFiles: 1,
		TargetPath:    firstNonEmpty(payload.Path, "/home"),
		Findings: []MalwareFinding{
			{ID: "finding-1", FilePath: firstNonEmpty(payload.Path, "/home") + "/suspicious.php", Signature: "webshell.php", Engine: firstNonEmpty(payload.Engine, "auto"), Quarantined: false},
		},
		Logs: []string{"Scan initiated.", "Signature database loaded.", "Potential webshell detected."},
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
			finding.Quarantined = true
			record := QuarantineRecord{
				ID:             generateSecret(8),
				JobID:          payload.JobID,
				FindingID:      payload.FindingID,
				OriginalPath:   finding.FilePath,
				QuarantinePath: "/var/quarantine/" + virtualBaseName(finding.FilePath),
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
	_ = file.Close()
	archivePath := "/var/lib/aurapanel/migrations/uploads/" + header.Filename
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.UploadedArchives[archivePath] = header.Filename
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
	analysis := MigrationAnalysis{
		SourceType: firstNonEmpty(sourceType, "cpanel"),
		Stats: MigrationStats{
			FileCount:     12843,
			DatabaseCount: 2,
			EmailCount:    6,
		},
		MySQLDumps:      []string{"mysql/example_app.sql", "mysql/analytics.sql"},
		EmailAccounts:   []string{"info@example.com", "support@example.com", "billing@example.com"},
		VhostCandidates: []string{"example.com", "blog.example.com"},
		Warnings: []string{
			"One cron job references a legacy /usr/local/bin/php path.",
			"Remote MySQL grants should be recreated on the destination panel.",
		},
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
	job := MigrationJob{
		ID:       "mig-" + generateSecret(6),
		Status:   "running",
		Progress: 15,
		Logs: []string{
			"Archive registered in migration queue.",
			"Filesystem inventory mapped to virtual website layout.",
			"Database conversion plan generated.",
		},
		Summary: MigrationSummary{
			ConvertedDBFiles: []string{"example_app.sql", "analytics.sql"},
			EmailPlanFile:    "email-plan.json",
			VhostPlanFile:    "vhost-plan.json",
			SystemApply:      false,
		},
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modules.MigrationJobs = append([]MigrationJob{job}, s.modules.MigrationJobs...)
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: job, Message: "Migration import queued."})
}

func (s *service) handleMigrationImportStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.modules.MigrationJobs {
		if s.modules.MigrationJobs[i].ID != id {
			continue
		}
		s.modules.MigrationJobs[i].PollCount++
		if s.modules.MigrationJobs[i].Progress < 100 {
			s.modules.MigrationJobs[i].Progress = minInt(100, s.modules.MigrationJobs[i].Progress+25)
			s.modules.MigrationJobs[i].Logs = append(s.modules.MigrationJobs[i].Logs, fmt.Sprintf("Step %d completed.", s.modules.MigrationJobs[i].PollCount))
			if s.modules.MigrationJobs[i].Progress >= 100 {
				s.modules.MigrationJobs[i].Status = "completed"
				s.modules.MigrationJobs[i].Logs = append(s.modules.MigrationJobs[i].Logs, "Migration finished in dry-run mode.")
			}
		}
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: s.modules.MigrationJobs[i]})
		return
	}
	writeError(w, http.StatusNotFound, "Migration job not found.")
}
