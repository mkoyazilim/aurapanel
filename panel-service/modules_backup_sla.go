package main

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const backupRestoreDrillHistoryLimit = 200

type backupSLADomainReport struct {
	Domain                   string   `json:"domain"`
	SnapshotCount            int      `json:"snapshot_count"`
	LatestSnapshotID         string   `json:"latest_snapshot_id,omitempty"`
	LatestSnapshotTime       string   `json:"latest_snapshot_time,omitempty"`
	LatestSnapshotAgeMinutes int64    `json:"latest_snapshot_age_minutes"`
	RetentionKeep            int      `json:"retention_keep"`
	WithinRPO                bool     `json:"within_rpo"`
	LastDrillStatus          string   `json:"last_drill_status,omitempty"`
	LastDrillAt              int64    `json:"last_drill_at,omitempty"`
	LastDrillAgeHours        int64    `json:"last_drill_age_hours,omitempty"`
	DrillHealthy             bool     `json:"drill_healthy"`
	Score                    int      `json:"score"`
	Status                   string   `json:"status"`
	Recommendations          []string `json:"recommendations"`
}

func parsePositiveInt(value string, fallback int) int {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func (s *service) latestBackupSnapshotForDomainLocked(domain string) (BackupSnapshot, bool) {
	target := normalizeDomain(domain)
	var latest BackupSnapshot
	found := false
	for _, item := range s.modules.BackupSnapshots {
		if normalizeDomain(item.Domain) != target {
			continue
		}
		if !found || backupSnapshotTimestamp(item) > backupSnapshotTimestamp(latest) {
			latest = item
			found = true
		}
	}
	return latest, found
}

func (s *service) findBackupSnapshotLocked(idOrShort string) (BackupSnapshot, bool) {
	key := strings.TrimSpace(idOrShort)
	if key == "" {
		return BackupSnapshot{}, false
	}
	for _, item := range s.modules.BackupSnapshots {
		if item.ID == key || item.ShortID == key {
			return item, true
		}
	}
	return BackupSnapshot{}, false
}

func (s *service) lastRestoreDrillForDomainLocked(domain string) (BackupRestoreDrill, bool) {
	target := normalizeDomain(domain)
	for _, item := range s.modules.BackupRestoreDrills {
		if normalizeDomain(item.Domain) == target {
			return item, true
		}
	}
	return BackupRestoreDrill{}, false
}

func backupReportStatusFromScore(score int) string {
	switch {
	case score >= 80:
		return "healthy"
	case score >= 50:
		return "warning"
	default:
		return "risk"
	}
}

func clampScore(score int) int {
	switch {
	case score < 0:
		return 0
	case score > 100:
		return 100
	default:
		return score
	}
}

func (s *service) handleBackupRestoreDrill(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain       string `json:"domain"`
		SnapshotID   string `json:"snapshot_id"`
		TargetDomain string `json:"target_domain"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid restore drill payload.")
		return
	}

	snapshotID := strings.TrimSpace(payload.SnapshotID)
	domain := normalizeDomain(payload.Domain)
	targetDomain := normalizeDomain(payload.TargetDomain)

	s.mu.RLock()
	var snapshot BackupSnapshot
	var found bool
	if snapshotID != "" {
		snapshot, found = s.findBackupSnapshotLocked(snapshotID)
	} else if domain != "" {
		snapshot, found = s.latestBackupSnapshotForDomainLocked(domain)
	}
	s.mu.RUnlock()

	if !found {
		writeError(w, http.StatusNotFound, "Backup snapshot was not found for restore drill.")
		return
	}

	if domain == "" {
		domain = normalizeDomain(snapshot.Domain)
	}
	if domain == "" {
		writeError(w, http.StatusBadRequest, "Domain is required for restore drill.")
		return
	}
	if targetDomain == "" {
		targetDomain = domain
	}

	snapshotPrepared, prepErr := s.prepareSnapshotForRestore(snapshot, BackupDestination{ID: snapshot.DestinationID, Enabled: true})
	if prepErr != nil {
		writeError(w, http.StatusBadRequest, prepErr.Error())
		return
	}
	snapshot = snapshotPrepared

	startedAt := time.Now().UTC()
	preview, previewErr := previewRuntimeSiteRestore(snapshot, targetDomain)

	drill := BackupRestoreDrill{
		ID:              generateSecret(8),
		Domain:          domain,
		TargetDomain:    targetDomain,
		SnapshotID:      snapshot.ID,
		SnapshotShortID: snapshot.ShortID,
		Status:          "failed",
		Message:         "Restore drill failed.",
		CheckedAt:       startedAt.UnixMilli(),
		DurationMs:      time.Since(startedAt).Milliseconds(),
		SizeBytes:       snapshot.SizeBytes,
	}

	if previewErr == nil {
		drill.Status = "success"
		drill.Message = "Restore drill preflight passed."
		if entryCount, ok := preview["archive_entry_count"].(int); ok {
			drill.EntryCount = entryCount
		}
		if sizeBytes, ok := preview["archive_size_bytes"].(int64); ok {
			drill.SizeBytes = sizeBytes
		}
	} else {
		drill.Message = previewErr.Error()
	}

	s.mu.Lock()
	s.modules.BackupRestoreDrills = append([]BackupRestoreDrill{drill}, s.modules.BackupRestoreDrills...)
	if len(s.modules.BackupRestoreDrills) > backupRestoreDrillHistoryLimit {
		s.modules.BackupRestoreDrills = s.modules.BackupRestoreDrills[:backupRestoreDrillHistoryLimit]
	}
	if drill.Status == "success" {
		s.appendActivityLocked("system", "backup_restore_drill_ok", "Restore drill passed for "+domain+".", "")
	} else {
		s.appendActivityLocked("system", "backup_restore_drill_failed", "Restore drill failed for "+domain+": "+drill.Message, "")
	}
	s.mu.Unlock()

	if previewErr != nil {
		writeJSON(w, http.StatusOK, apiResponse{
			Status:  "error",
			Message: "Restore drill failed.",
			Data: map[string]interface{}{
				"drill": drill,
			},
		})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: "Restore drill completed.",
		Data: map[string]interface{}{
			"drill":   drill,
			"preview": preview,
		},
	})
}

func (s *service) handleBackupRestoreDrillHistory(w http.ResponseWriter, r *http.Request) {
	domain := normalizeDomain(r.URL.Query().Get("domain"))
	limit := parsePositiveInt(r.URL.Query().Get("limit"), 50)

	s.mu.RLock()
	items := make([]BackupRestoreDrill, 0, len(s.modules.BackupRestoreDrills))
	for _, item := range s.modules.BackupRestoreDrills {
		if domain == "" || normalizeDomain(item.Domain) == domain {
			items = append(items, item)
		}
		if len(items) >= limit {
			break
		}
	}
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
}

func (s *service) handleBackupSLAReport(w http.ResponseWriter, r *http.Request) {
	rpoTargetMinutes := parsePositiveInt(r.URL.Query().Get("rpo_minutes"), 24*60)
	drillFreshnessHours := parsePositiveInt(r.URL.Query().Get("drill_freshness_hours"), 24*7)
	domainFilter := normalizeDomain(r.URL.Query().Get("domain"))
	now := time.Now().UTC()

	s.mu.RLock()
	domainSet := map[string]struct{}{}
	for _, site := range s.state.Websites {
		domain := normalizeDomain(site.Domain)
		if domain != "" {
			if domainFilter != "" && domain != domainFilter {
				continue
			}
			domainSet[domain] = struct{}{}
		}
	}
	for _, snapshot := range s.modules.BackupSnapshots {
		domain := normalizeDomain(snapshot.Domain)
		if domain != "" {
			if domainFilter != "" && domain != domainFilter {
				continue
			}
			domainSet[domain] = struct{}{}
		}
	}
	if domainFilter != "" {
		if _, ok := domainSet[domainFilter]; !ok {
			domainSet[domainFilter] = struct{}{}
		}
	}

	domains := make([]string, 0, len(domainSet))
	for domain := range domainSet {
		domains = append(domains, domain)
	}
	sort.Strings(domains)

	reports := make([]backupSLADomainReport, 0, len(domains))
	healthyDomains := 0
	backupHealthyDomains := 0
	drillHealthyDomains := 0

	for _, domain := range domains {
		latestSnapshot, hasSnapshot := s.latestBackupSnapshotForDomainLocked(domain)
		lastDrill, hasDrill := s.lastRestoreDrillForDomainLocked(domain)

		snapshotCount := 0
		for _, item := range s.modules.BackupSnapshots {
			if normalizeDomain(item.Domain) == domain {
				snapshotCount++
			}
		}

		report := backupSLADomainReport{
			Domain:                   domain,
			SnapshotCount:            snapshotCount,
			LatestSnapshotAgeMinutes: -1,
			RetentionKeep:            backupRetentionKeepFromEnv(),
			Status:                   "risk",
		}

		score := 0
		if hasSnapshot {
			createdAt := backupSnapshotTimestamp(latestSnapshot)
			report.LatestSnapshotID = firstNonEmpty(latestSnapshot.ShortID, latestSnapshot.ID)
			report.LatestSnapshotTime = latestSnapshot.Time
			if latestSnapshot.RetentionKeep > 0 {
				report.RetentionKeep = latestSnapshot.RetentionKeep
			}
			age := now.Sub(time.UnixMilli(createdAt))
			report.LatestSnapshotAgeMinutes = int64(age / time.Minute)
			report.WithinRPO = report.LatestSnapshotAgeMinutes <= int64(rpoTargetMinutes)
			score += 40
			if report.WithinRPO {
				score += 25
				backupHealthyDomains++
			} else {
				report.Recommendations = append(report.Recommendations, "Latest snapshot is outside RPO target.")
				score += 5
			}
		} else {
			report.Recommendations = append(report.Recommendations, "No backup snapshot exists for this domain.")
		}

		if report.RetentionKeep >= 7 {
			score += 10
		} else {
			score += 5
			report.Recommendations = append(report.Recommendations, "Retention is low; keep at least 7 snapshots.")
		}

		if hasDrill {
			report.LastDrillStatus = lastDrill.Status
			report.LastDrillAt = lastDrill.CheckedAt
			drillAge := now.Sub(time.UnixMilli(lastDrill.CheckedAt))
			report.LastDrillAgeHours = int64(drillAge / time.Hour)
			report.DrillHealthy = lastDrill.Status == "success" && report.LastDrillAgeHours <= int64(drillFreshnessHours)
			if report.DrillHealthy {
				score += 25
				drillHealthyDomains++
			} else if lastDrill.Status == "success" {
				score += 10
				report.Recommendations = append(report.Recommendations, "Restore drill is stale; run a fresh drill.")
			} else {
				report.Recommendations = append(report.Recommendations, "Latest restore drill failed.")
			}
		} else {
			report.Recommendations = append(report.Recommendations, "No restore drill history found.")
		}

		report.Score = clampScore(score)
		report.Status = backupReportStatusFromScore(report.Score)
		if report.Status == "healthy" {
			healthyDomains++
		}
		reports = append(reports, report)
	}
	s.mu.RUnlock()

	total := len(reports)
	coveragePct := 0
	if total > 0 {
		coveragePct = int(float64(healthyDomains) / float64(total) * 100)
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Status: "success",
		Data: map[string]interface{}{
			"generated_at":          now.UnixMilli(),
			"rpo_target_minutes":    rpoTargetMinutes,
			"drill_freshness_hours": drillFreshnessHours,
			"domains":               reports,
			"summary": map[string]interface{}{
				"total_domains":          total,
				"healthy_domains":        healthyDomains,
				"backup_healthy_domains": backupHealthyDomains,
				"drill_healthy_domains":  drillHealthyDomains,
				"coverage_percent":       coveragePct,
			},
		},
	})
}
