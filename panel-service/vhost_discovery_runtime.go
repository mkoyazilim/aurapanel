package main

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type DiscoveredWebsite struct {
	Domain       string `json:"domain"`
	Path         string `json:"path"`
	Docroot      string `json:"docroot"`
	Owner        string `json:"owner"`
	Managed      bool   `json:"managed"`
	HasDocroot   bool   `json:"has_docroot"`
	HasIndex     bool   `json:"has_index"`
	SuggestedPHP string `json:"suggested_php"`
}

func boolFromQuery(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func detectFilesystemOwner(path string) string {
	output, err := commandOutputTrimmed("stat", "-c", "%U", path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(output)
}

func hasSiteIndex(docroot string) bool {
	return fileExists(filepath.Join(docroot, "index.php")) || fileExists(filepath.Join(docroot, "index.html"))
}

func (s *service) discoverWebsitesFromFilesystem(includeManaged bool) ([]DiscoveredWebsite, error) {
	entries, err := os.ReadDir("/home")
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	managed := make(map[string]Website, len(s.state.Websites))
	for _, site := range s.state.Websites {
		managed[normalizeDomain(site.Domain)] = site
	}
	s.mu.RUnlock()

	defaultPHP := firstInstalledPHPVersion()
	discovered := make([]DiscoveredWebsite, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		domain := normalizeDomain(entry.Name())
		if !isValidDomainName(domain) {
			continue
		}

		siteRoot := filepath.Join("/home", domain)
		docroot := domainDocroot(domain)
		managedSite, isManaged := managed[domain]
		if isManaged && !includeManaged {
			continue
		}

		owner := sanitizeName(detectFilesystemOwner(siteRoot))
		if owner == "" {
			owner = "aura"
		}
		suggestedPHP := defaultPHP
		if isManaged {
			suggestedPHP = firstNonEmpty(strings.TrimSpace(managedSite.PHPVersion), strings.TrimSpace(managedSite.PHP), defaultPHP)
		}

		discovered = append(discovered, DiscoveredWebsite{
			Domain:       domain,
			Path:         siteRoot,
			Docroot:      docroot,
			Owner:        owner,
			Managed:      isManaged,
			HasDocroot:   fileExists(docroot),
			HasIndex:     hasSiteIndex(docroot),
			SuggestedPHP: suggestedPHP,
		})
	}

	sort.Slice(discovered, func(i, j int) bool {
		return discovered[i].Domain < discovered[j].Domain
	})

	return discovered, nil
}

func (s *service) handleVhostDiscovery(w http.ResponseWriter, r *http.Request) {
	includeManaged := boolFromQuery(r.URL.Query().Get("include_managed"))
	items, err := s.discoverWebsitesFromFilesystem(includeManaged)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: items})
}

func (s *service) importWebsiteArtifactsLocked(site Website) error {
	s.ensureDefaultSiteArtifactsLocked(site.Domain)
	return s.syncOLSVhostsLocked()
}

func (s *service) handleVhostImport(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Domain     string `json:"domain"`
		Owner      string `json:"owner"`
		User       string `json:"user"`
		PHPVersion string `json:"php_version"`
		Package    string `json:"package"`
		Email      string `json:"email"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid website import payload.")
		return
	}

	domain := normalizeDomain(payload.Domain)
	if !isValidDomainName(domain) {
		writeError(w, http.StatusBadRequest, "A valid domain is required.")
		return
	}
	siteRoot := filepath.Join("/home", domain)
	rootInfo, err := os.Stat(siteRoot)
	if err != nil || !rootInfo.IsDir() {
		writeError(w, http.StatusNotFound, "Website directory under /home was not found.")
		return
	}

	installedVersions := installedPHPVersionsSet()
	phpVersion := normalizePHPVersion(payload.PHPVersion)
	if phpVersion == "" {
		phpVersion = firstInstalledPHPVersion()
	}
	if _, ok := installedVersions[phpVersion]; !ok {
		writeError(w, http.StatusBadRequest, "Selected PHP version is not installed.")
		return
	}

	owner := sanitizeName(firstNonEmpty(strings.TrimSpace(payload.Owner), strings.TrimSpace(payload.User), detectFilesystemOwner(siteRoot)))
	if owner == "" {
		owner = "aura"
	}

	email := firstNonEmpty(strings.TrimSpace(payload.Email), "webmaster@"+domain)
	site := Website{
		Domain:        domain,
		Owner:         owner,
		User:          owner,
		PHP:           phpVersion,
		PHPVersion:    phpVersion,
		Package:       firstNonEmpty(strings.TrimSpace(payload.Package), "default"),
		Email:         email,
		Status:        "active",
		SSL:           false,
		DiskUsage:     "0.0 GB",
		Quota:         "10 GB",
		MailDomain:    false,
		ApacheBackend: false,
		CreatedAt:     time.Now().UTC().Unix(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.findWebsiteLocked(domain) != nil {
		writeError(w, http.StatusConflict, "Website already exists.")
		return
	}
	site.Quota = quotaForPackage(s.state.Packages, site.Package)
	if certPath, _ := findCertificatePair(domain); certPath != "" {
		site.SSL = true
	}

	snapshot, err := s.captureRuntimeSnapshotLocked()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to prepare import rollback state.")
		return
	}

	s.state.Websites = append(s.state.Websites, site)
	s.ensureUserLocked(owner, firstNonEmpty(strings.TrimSpace(payload.Email), owner+"@example.com"), "user", site.Package, "")
	s.recountSitesLocked()
	if err := s.importWebsiteArtifactsLocked(site); err != nil {
		s.restoreRuntimeSnapshotLocked(snapshot)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.appendActivityLocked("system", "vhost_import", domain+" imported from filesystem.", "")
	s.saveRuntimeStateLocked()

	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: "Website imported from filesystem.",
		Data:    site,
	})
}
