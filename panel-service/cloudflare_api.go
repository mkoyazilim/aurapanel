package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const cloudflareAPIBase = "https://api.cloudflare.com/client/v4"
const cloudflareServiceEnvPath = "/etc/aurapanel/aurapanel-service.env"

type cloudflareCredentials struct {
	Email    string
	APIKey   string
	APIToken string
}

type cloudflareRuntimeStatus struct {
	Configured       bool   `json:"configured"`
	AutoSync         bool   `json:"auto_sync"`
	CredentialSource string `json:"credential_source"`
	EmailHint        string `json:"email_hint,omitempty"`
	HasAPIToken      bool   `json:"has_api_token"`
	HasGlobalKey     bool   `json:"has_global_key"`
}

type cfErr struct {
	Message string `json:"message"`
}

type cloudflareZoneAPIResult struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	NameServers []string `json:"name_servers"`
	Plan        struct {
		Name string `json:"name"`
	} `json:"plan"`
}

type cloudflareDNSRecordAPIResult struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

func cloudflareRequestCredentials(body map[string]interface{}) cloudflareCredentials {
	email := strings.TrimSpace(stringValue(body["email"]))
	key := strings.TrimSpace(stringValue(body["api_key"]))
	token := strings.TrimSpace(stringValue(body["api_token"]))
	if token == "" && email == "" && looksLikeAPIToken(key) {
		token = key
		key = ""
	}
	return cloudflareCredentials{
		Email:    email,
		APIKey:   key,
		APIToken: token,
	}
}

func cloudflareEnvCredentials() cloudflareCredentials {
	return cloudflareCredentials{
		Email:    strings.TrimSpace(os.Getenv("AURAPANEL_CLOUDFLARE_EMAIL")),
		APIKey:   strings.TrimSpace(os.Getenv("AURAPANEL_CLOUDFLARE_API_KEY")),
		APIToken: strings.TrimSpace(os.Getenv("AURAPANEL_CLOUDFLARE_API_TOKEN")),
	}
}

func cloudflareResolveCredentials(body map[string]interface{}) cloudflareCredentials {
	requestCreds := cloudflareRequestCredentials(body)
	if requestCreds.valid() {
		return requestCreds
	}
	return cloudflareEnvCredentials()
}

func (c cloudflareCredentials) valid() bool {
	return c.APIToken != "" || (c.Email != "" && c.APIKey != "")
}

func looksLikeAPIToken(value string) bool {
	if value == "" {
		return false
	}
	if strings.Contains(value, " ") {
		return false
	}
	return len(value) >= 32
}

func cloudflareAutoSyncEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(envOr("AURAPANEL_CLOUDFLARE_AUTO_SYNC", "0")))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func cloudflareRuntimeSnapshot() cloudflareRuntimeStatus {
	creds := cloudflareEnvCredentials()
	source := "none"
	switch {
	case creds.APIToken != "":
		source = "api_token"
	case creds.Email != "" && creds.APIKey != "":
		source = "global_key"
	}
	return cloudflareRuntimeStatus{
		Configured:       creds.valid(),
		AutoSync:         cloudflareAutoSyncEnabled(),
		CredentialSource: source,
		EmailHint:        redactEmailHint(creds.Email),
		HasAPIToken:      creds.APIToken != "",
		HasGlobalKey:     creds.Email != "" && creds.APIKey != "",
	}
}

func persistCloudflareServerCredentials(creds cloudflareCredentials, autoSync bool) error {
	updates := map[string]string{
		"AURAPANEL_CLOUDFLARE_EMAIL":     strings.TrimSpace(creds.Email),
		"AURAPANEL_CLOUDFLARE_API_KEY":   strings.TrimSpace(creds.APIKey),
		"AURAPANEL_CLOUDFLARE_API_TOKEN": strings.TrimSpace(creds.APIToken),
		"AURAPANEL_CLOUDFLARE_AUTO_SYNC": "0",
	}
	if autoSync {
		updates["AURAPANEL_CLOUDFLARE_AUTO_SYNC"] = "1"
	}
	if err := writeEnvFileValues(cloudflareServiceEnvPath, updates); err != nil {
		return err
	}

	// Apply immediately so the running process can use the new credentials
	// without waiting for a service restart.
	_ = os.Setenv("AURAPANEL_CLOUDFLARE_EMAIL", updates["AURAPANEL_CLOUDFLARE_EMAIL"])
	_ = os.Setenv("AURAPANEL_CLOUDFLARE_API_KEY", updates["AURAPANEL_CLOUDFLARE_API_KEY"])
	_ = os.Setenv("AURAPANEL_CLOUDFLARE_API_TOKEN", updates["AURAPANEL_CLOUDFLARE_API_TOKEN"])
	_ = os.Setenv("AURAPANEL_CLOUDFLARE_AUTO_SYNC", updates["AURAPANEL_CLOUDFLARE_AUTO_SYNC"])
	return nil
}

func redactEmailHint(email string) string {
	email = strings.TrimSpace(email)
	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	local := parts[0]
	if len(local) <= 2 {
		return local[:1] + "***@" + parts[1]
	}
	return local[:1] + "***" + local[len(local)-1:] + "@" + parts[1]
}

func cloudflareHTTPClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

func cloudflareAPICall[T any](creds cloudflareCredentials, method, path string, payload interface{}, resultTarget *T) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, cloudflareAPIBase+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if creds.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+creds.APIToken)
	} else {
		req.Header.Set("X-Auth-Email", creds.Email)
		req.Header.Set("X-Auth-Key", creds.APIKey)
	}

	resp, err := cloudflareHTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Cloudflare API returned %d", resp.StatusCode)
	}

	type envelope struct {
		Success bool            `json:"success"`
		Errors  []cfErr         `json:"errors"`
		Result  json.RawMessage `json:"result"`
	}
	var decoded envelope
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	if !decoded.Success {
		return fmt.Errorf(cloudflareAPIError(decoded.Errors))
	}
	if resultTarget == nil || len(decoded.Result) == 0 {
		return nil
	}
	if err := json.Unmarshal(decoded.Result, resultTarget); err != nil {
		return err
	}
	return nil
}

func cloudflareAPIError(errors []cfErr) string {
	if len(errors) == 0 {
		return "Cloudflare API request failed"
	}
	return firstNonEmpty(strings.TrimSpace(errors[0].Message), "Cloudflare API request failed")
}

func cloudflareListZones(creds cloudflareCredentials) ([]CloudflareZone, error) {
	var results []cloudflareZoneAPIResult
	if err := cloudflareAPICall(creds, http.MethodGet, "/zones?per_page=100", nil, &results); err != nil {
		return nil, err
	}
	out := make([]CloudflareZone, 0, len(results))
	for _, item := range results {
		out = append(out, CloudflareZone{
			ID:          item.ID,
			Name:        item.Name,
			Status:      item.Status,
			Plan:        item.Plan.Name,
			NameServers: item.NameServers,
		})
	}
	return out, nil
}

func cloudflareListDNSRecords(creds cloudflareCredentials, zoneID string) ([]CloudflareDNSRecord, error) {
	var results []cloudflareDNSRecordAPIResult
	if err := cloudflareAPICall(creds, http.MethodGet, fmt.Sprintf("/zones/%s/dns_records?per_page=500", zoneID), nil, &results); err != nil {
		return nil, err
	}
	out := make([]CloudflareDNSRecord, 0, len(results))
	for _, item := range results {
		out = append(out, CloudflareDNSRecord{
			ID:      item.ID,
			Type:    item.Type,
			Name:    item.Name,
			Content: item.Content,
			TTL:     item.TTL,
			Proxied: item.Proxied,
		})
	}
	return out, nil
}

func cloudflareCreateDNSRecord(creds cloudflareCredentials, zoneID string, record CloudflareDNSRecord) (CloudflareDNSRecord, error) {
	payload := map[string]interface{}{
		"type":    record.Type,
		"name":    record.Name,
		"content": record.Content,
		"ttl":     maxInt(record.TTL, 1),
		"proxied": record.Proxied,
	}
	var created cloudflareDNSRecordAPIResult
	if err := cloudflareAPICall(creds, http.MethodPost, fmt.Sprintf("/zones/%s/dns_records", zoneID), payload, &created); err != nil {
		return CloudflareDNSRecord{}, err
	}
	return CloudflareDNSRecord{
		ID:      created.ID,
		Type:    created.Type,
		Name:    created.Name,
		Content: created.Content,
		TTL:     created.TTL,
		Proxied: created.Proxied,
	}, nil
}

func cloudflareUpdateDNSRecord(creds cloudflareCredentials, zoneID, recordID string, record CloudflareDNSRecord) error {
	payload := map[string]interface{}{
		"type":    record.Type,
		"name":    record.Name,
		"content": record.Content,
		"ttl":     maxInt(record.TTL, 1),
		"proxied": record.Proxied,
	}
	return cloudflareAPICall[map[string]interface{}](creds, http.MethodPut, fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID), payload, nil)
}

func cloudflareDeleteDNSRecord(creds cloudflareCredentials, zoneID, recordID string) error {
	return cloudflareAPICall[map[string]interface{}](creds, http.MethodDelete, fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID), nil, nil)
}

func cloudflarePatchSetting(creds cloudflareCredentials, zoneID, setting, value string) error {
	return cloudflareAPICall[map[string]interface{}](creds, http.MethodPatch, fmt.Sprintf("/zones/%s/settings/%s", zoneID, setting), map[string]string{"value": value}, nil)
}

func cloudflarePurgeCache(creds cloudflareCredentials, zoneID string, payload map[string]interface{}) error {
	return cloudflareAPICall[map[string]interface{}](creds, http.MethodPost, fmt.Sprintf("/zones/%s/purge_cache", zoneID), payload, nil)
}

func cloudflareAnalytics(creds cloudflareCredentials, zoneID string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := cloudflareAPICall(creds, http.MethodGet, fmt.Sprintf("/zones/%s/analytics/dashboard", zoneID), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func cloudflareZoneIDForDomain(creds cloudflareCredentials, domain string) (string, error) {
	zones, err := cloudflareListZones(creds)
	if err != nil {
		return "", err
	}
	normalizedDomain := normalizeDomain(domain)
	for _, zone := range zones {
		if normalizeDomain(zone.Name) == normalizedDomain {
			return zone.ID, nil
		}
	}
	return "", fmt.Errorf("Cloudflare zone not found for %s", domain)
}

func cloudflareAbsoluteRecordName(zoneName, recordName string) string {
	zoneName = normalizeDomain(zoneName)
	name := strings.TrimSpace(recordName)
	switch {
	case name == "", name == "@", normalizeDomain(name) == zoneName:
		return zoneName
	case strings.Contains(name, "."):
		return normalizeDomain(name)
	default:
		return normalizeDomain(name + "." + zoneName)
	}
}

func shouldProxyCloudflareRecord(recordType, recordName string) bool {
	upperType := strings.ToUpper(strings.TrimSpace(recordType))
	if upperType != "A" && upperType != "AAAA" && upperType != "CNAME" {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(recordName))
	return name == "@" || name == "" || name == "www" || name == "panel"
}

func (s *service) syncCloudflareZoneRecordsLocked(domain string) error {
	if !cloudflareAutoSyncEnabled() {
		return nil
	}

	creds := cloudflareEnvCredentials()
	if !creds.valid() {
		return nil
	}

	zoneID, err := cloudflareZoneIDForDomain(creds, domain)
	if err != nil {
		return err
	}
	existing, err := cloudflareListDNSRecords(creds, zoneID)
	if err != nil {
		return err
	}

	existingByKey := map[string]CloudflareDNSRecord{}
	for _, item := range existing {
		key := strings.ToUpper(item.Type) + "|" + normalizeDomain(item.Name)
		existingByKey[key] = item
	}

	for _, localRecord := range s.modules.DNSRecords[normalizeDomain(domain)] {
		recordName := cloudflareAbsoluteRecordName(domain, localRecord.Name)
		record := CloudflareDNSRecord{
			Type:    strings.ToUpper(localRecord.RecordType),
			Name:    recordName,
			Content: localRecord.Content,
			TTL:     maxInt(localRecord.TTL, 1),
			Proxied: shouldProxyCloudflareRecord(localRecord.RecordType, localRecord.Name),
		}
		key := record.Type + "|" + normalizeDomain(record.Name)
		if existingItem, ok := existingByKey[key]; ok {
			if existingItem.Content != record.Content || existingItem.TTL != record.TTL || existingItem.Proxied != record.Proxied {
				if err := cloudflareUpdateDNSRecord(creds, zoneID, existingItem.ID, record); err != nil {
					return err
				}
			}
			continue
		}
		if _, err := cloudflareCreateDNSRecord(creds, zoneID, record); err != nil {
			return err
		}
	}

	return nil
}

func stringValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func intValue(value interface{}, fallback int) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return fallback
	}
}

func boolValue(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	default:
		return false
	}
}
