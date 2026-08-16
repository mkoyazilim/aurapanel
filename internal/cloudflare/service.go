package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

const cfBase = "https://api.cloudflare.com/client/v4"

type Dependencies struct {
	Store *store.Store
	Audit *audit.Service
}

type Service struct {
	deps       Dependencies
	httpClient *http.Client
}

func NewService(deps Dependencies) *Service {
	return &Service{
		deps:       deps,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// ── CF API yardımcı ───────────────────────────────────────────────────────────

type cfResponse struct {
	Success  bool              `json:"success"`
	Errors   []cfError         `json:"errors"`
	Result   json.RawMessage   `json:"result"`
	ResultInfo *cfResultInfo   `json:"result_info,omitempty"`
}

type cfError  struct { Code int `json:"code"`; Message string `json:"message"` }
type cfResultInfo struct { Page int `json:"page"`; PerPage int `json:"per_page"`; Count int `json:"count"`; TotalCount int `json:"total_count"` }

// token, siteID için kayıtlı token'ı ya da global hesap token'ını döner.
func (s *Service) token(ctx context.Context, siteID string) (string, error) {
	if siteID != "" {
		cfg, err := s.deps.Store.GetCloudflareSettings(ctx, siteID)
		if err == nil && cfg != nil && cfg.APIToken != "" {
			return cfg.APIToken, nil
		}
	}
	acc, err := s.deps.Store.GetCloudflareAccount(ctx)
	if err != nil {
		return "", err
	}
	if acc == nil || acc.APIToken == "" {
		return "", fmt.Errorf("cloudflare api token yapılandırılmamış")
	}
	return acc.APIToken, nil
}

// zoneID, siteID için zone_id döner.
func (s *Service) zoneID(ctx context.Context, siteID string) (string, error) {
	cfg, err := s.deps.Store.GetCloudflareSettings(ctx, siteID)
	if err != nil {
		return "", err
	}
	if cfg == nil || cfg.ZoneID == "" {
		return "", fmt.Errorf("cloudflare zone id yapılandırılmamış")
	}
	return cfg.ZoneID, nil
}

func (s *Service) do(ctx context.Context, method, url, token string, body any) (*cfResponse, error) {
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out cfResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("cloudflare yanıtı çözümlenemedi: %w", err)
	}
	if !out.Success && len(out.Errors) > 0 {
		return nil, fmt.Errorf("cloudflare hata %d: %s", out.Errors[0].Code, out.Errors[0].Message)
	}
	return &out, nil
}

// ── Zone Listesi (global hesap) ───────────────────────────────────────────────

type Zone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Plan   struct{ Name string `json:"name"` } `json:"plan"`
}

func (s *Service) ListZones(ctx context.Context) ([]Zone, error) {
	tok, err := s.token(ctx, "")
	if err != nil {
		return nil, err
	}
	r, err := s.do(ctx, "GET", cfBase+"/zones?per_page=50", tok, nil)
	if err != nil {
		return nil, err
	}
	var zones []Zone
	json.Unmarshal(r.Result, &zones)
	return zones, nil
}

// ── DNS Kayıtları ─────────────────────────────────────────────────────────────

type DNSRecord struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Proxied  bool   `json:"proxied"`
	Modified string `json:"modified_on,omitempty"`
}

func (s *Service) ListDNSRecords(ctx context.Context, siteID string) ([]DNSRecord, error) {
	tok, err := s.token(ctx, siteID)
	if err != nil { return nil, err }
	zID, err := s.zoneID(ctx, siteID)
	if err != nil { return nil, err }

	r, err := s.do(ctx, "GET", fmt.Sprintf("%s/zones/%s/dns_records?per_page=100", cfBase, zID), tok, nil)
	if err != nil { return nil, err }
	var recs []DNSRecord
	json.Unmarshal(r.Result, &recs)
	return recs, nil
}

func (s *Service) CreateDNSRecord(ctx context.Context, siteID string, rec DNSRecord) (*DNSRecord, error) {
	tok, err := s.token(ctx, siteID)
	if err != nil { return nil, err }
	zID, err := s.zoneID(ctx, siteID)
	if err != nil { return nil, err }

	r, err := s.do(ctx, "POST", fmt.Sprintf("%s/zones/%s/dns_records", cfBase, zID), tok, rec)
	if err != nil { return nil, err }
	var out DNSRecord
	json.Unmarshal(r.Result, &out)
	s.deps.Audit.Write(ctx, audit.Event{Action: "cf.dns.create", Target: siteID})
	return &out, nil
}

func (s *Service) UpdateDNSRecord(ctx context.Context, siteID, recID string, rec DNSRecord) (*DNSRecord, error) {
	tok, err := s.token(ctx, siteID)
	if err != nil { return nil, err }
	zID, err := s.zoneID(ctx, siteID)
	if err != nil { return nil, err }

	r, err := s.do(ctx, "PATCH", fmt.Sprintf("%s/zones/%s/dns_records/%s", cfBase, zID, recID), tok, rec)
	if err != nil { return nil, err }
	var out DNSRecord
	json.Unmarshal(r.Result, &out)
	s.deps.Audit.Write(ctx, audit.Event{Action: "cf.dns.update", Target: siteID})
	return &out, nil
}

func (s *Service) DeleteDNSRecord(ctx context.Context, siteID, recID string) error {
	tok, err := s.token(ctx, siteID)
	if err != nil { return err }
	zID, err := s.zoneID(ctx, siteID)
	if err != nil { return err }

	_, err = s.do(ctx, "DELETE", fmt.Sprintf("%s/zones/%s/dns_records/%s", cfBase, zID, recID), tok, nil)
	if err != nil { return err }
	s.deps.Audit.Write(ctx, audit.Event{Action: "cf.dns.delete", Target: siteID})
	return nil
}

// ── Zone Ayarları ─────────────────────────────────────────────────────────────

type ZoneSettings struct {
	SSL           string `json:"ssl"`            // "off"|"flexible"|"full"|"strict"
	AlwaysHTTPS   string `json:"always_https"`   // "on"|"off"
	MinTLSVersion string `json:"min_tls_version"`// "1.0"|"1.1"|"1.2"|"1.3"
	SecurityLevel string `json:"security_level"` // "essentially_off"|"low"|"medium"|"high"|"under_attack"
	BotFightMode  string `json:"bot_fight_mode"` // "on"|"off"
	RocketLoader  string `json:"rocket_loader"`  // "on"|"off"|"auto"
	Minify        struct {
		CSS  bool `json:"css"`
		HTML bool `json:"html"`
		JS   bool `json:"js"`
	} `json:"minify"`
	CacheLevel    string `json:"cache_level"`    // "aggressive"|"basic"|"simplified"
	BrowserCacheTTL int  `json:"browser_cache_ttl"` // saniye
}

func (s *Service) GetZoneSettings(ctx context.Context, siteID string) (*ZoneSettings, error) {
	tok, err := s.token(ctx, siteID)
	if err != nil { return nil, err }
	zID, err := s.zoneID(ctx, siteID)
	if err != nil { return nil, err }

	type cfSetting struct { ID string `json:"id"`; Value any `json:"value"` }
	r, err := s.do(ctx, "GET", fmt.Sprintf("%s/zones/%s/settings", cfBase, zID), tok, nil)
	if err != nil { return nil, err }

	var settings []cfSetting
	json.Unmarshal(r.Result, &settings)

	out := &ZoneSettings{
		SSL: "flexible", AlwaysHTTPS: "off", MinTLSVersion: "1.2",
		SecurityLevel: "medium", BotFightMode: "off", RocketLoader: "off",
		CacheLevel: "aggressive", BrowserCacheTTL: 14400,
	}
	for _, st := range settings {
		switch st.ID {
		case "ssl":
			if v, ok := st.Value.(string); ok { out.SSL = v }
		case "always_use_https":
			if v, ok := st.Value.(string); ok { out.AlwaysHTTPS = v }
		case "min_tls_version":
			if v, ok := st.Value.(string); ok { out.MinTLSVersion = v }
		case "security_level":
			if v, ok := st.Value.(string); ok { out.SecurityLevel = v }
		case "rocket_loader":
			if v, ok := st.Value.(string); ok { out.RocketLoader = v }
		case "cache_level":
			if v, ok := st.Value.(string); ok { out.CacheLevel = v }
		case "browser_cache_ttl":
			if v, ok := st.Value.(float64); ok { out.BrowserCacheTTL = int(v) }
		case "bot_fight_mode":
			if v, ok := st.Value.(string); ok { out.BotFightMode = v }
		case "minify":
			if m, ok := st.Value.(map[string]any); ok {
				out.Minify.CSS  = m["css"] == "on"
				out.Minify.HTML = m["html"] == "on"
				out.Minify.JS   = m["js"] == "on"
			}
		}
	}
	return out, nil
}

func (s *Service) UpdateZoneSetting(ctx context.Context, siteID, settingID string, value any) error {
	tok, err := s.token(ctx, siteID)
	if err != nil { return err }
	zID, err := s.zoneID(ctx, siteID)
	if err != nil { return err }

	_, err = s.do(ctx, "PATCH",
		fmt.Sprintf("%s/zones/%s/settings/%s", cfBase, zID, settingID),
		tok, map[string]any{"value": value})
	if err != nil { return err }
	s.deps.Audit.Write(ctx, audit.Event{Action: "cf.settings.update", Target: siteID, Extra: map[string]any{"setting": settingID, "value": fmt.Sprintf("%v", value)}})
	return nil
}

// ── Cache ─────────────────────────────────────────────────────────────────────

func (s *Service) PurgeCache(ctx context.Context, siteID string) error {
	tok, err := s.token(ctx, siteID)
	if err != nil { return err }
	zID, err := s.zoneID(ctx, siteID)
	if err != nil { return err }

	_, err = s.do(ctx, "POST",
		fmt.Sprintf("%s/zones/%s/purge_cache", cfBase, zID),
		tok, map[string]bool{"purge_everything": true})
	if err != nil { return err }
	s.deps.Audit.Write(ctx, audit.Event{Action: "cf.cache.purge_all", Target: siteID})
	return nil
}

func (s *Service) PurgeCacheByURLs(ctx context.Context, siteID string, urls []string) error {
	tok, err := s.token(ctx, siteID)
	if err != nil { return err }
	zID, err := s.zoneID(ctx, siteID)
	if err != nil { return err }

	_, err = s.do(ctx, "POST",
		fmt.Sprintf("%s/zones/%s/purge_cache", cfBase, zID),
		tok, map[string][]string{"files": urls})
	if err != nil { return err }
	s.deps.Audit.Write(ctx, audit.Event{Action: "cf.cache.purge_urls", Target: siteID})
	return nil
}

// ── Firewall / WAF Kuralları ──────────────────────────────────────────────────

type FirewallRule struct {
	ID          string `json:"id,omitempty"`
	Description string `json:"description"`
	Expression  string `json:"expression"`
	Action      string `json:"action"` // "block"|"challenge"|"js_challenge"|"managed_challenge"|"allow"|"log"
	Enabled     bool   `json:"enabled"`
}

func (s *Service) ListFirewallRules(ctx context.Context, siteID string) ([]FirewallRule, error) {
	tok, err := s.token(ctx, siteID)
	if err != nil { return nil, err }
	zID, err := s.zoneID(ctx, siteID)
	if err != nil { return nil, err }

	r, err := s.do(ctx, "GET", fmt.Sprintf("%s/zones/%s/firewall/rules", cfBase, zID), tok, nil)
	if err != nil { return nil, err }
	var rules []FirewallRule
	json.Unmarshal(r.Result, &rules)
	return rules, nil
}

func (s *Service) CreateFirewallRule(ctx context.Context, siteID string, rule FirewallRule) (*FirewallRule, error) {
	tok, err := s.token(ctx, siteID)
	if err != nil { return nil, err }
	zID, err := s.zoneID(ctx, siteID)
	if err != nil { return nil, err }

	// CF firewall rules API list tipinde alıyor
	type fwPayload struct {
		Filter struct { Expression string `json:"expression"` } `json:"filter"`
		Action      string `json:"action"`
		Description string `json:"description"`
		Enabled     bool   `json:"enabled"`
	}
	payload := []fwPayload{{Action: rule.Action, Description: rule.Description, Enabled: rule.Enabled}}
	payload[0].Filter.Expression = rule.Expression

	r, err := s.do(ctx, "POST", fmt.Sprintf("%s/zones/%s/firewall/rules", cfBase, zID), tok, payload)
	if err != nil { return nil, err }
	var out []FirewallRule
	json.Unmarshal(r.Result, &out)
	if len(out) == 0 { return &rule, nil }
	s.deps.Audit.Write(ctx, audit.Event{Action: "cf.firewall.create", Target: siteID})
	return &out[0], nil
}

func (s *Service) DeleteFirewallRule(ctx context.Context, siteID, ruleID string) error {
	tok, err := s.token(ctx, siteID)
	if err != nil { return err }
	zID, err := s.zoneID(ctx, siteID)
	if err != nil { return err }

	_, err = s.do(ctx, "DELETE", fmt.Sprintf("%s/zones/%s/firewall/rules/%s", cfBase, zID, ruleID), tok, nil)
	if err != nil { return err }
	s.deps.Audit.Write(ctx, audit.Event{Action: "cf.firewall.delete", Target: siteID})
	return nil
}

// ── Analytics ────────────────────────────────────────────────────────────────

type ZoneAnalytics struct {
	Requests struct {
		Total    int `json:"total"`
		Cached   int `json:"cached"`
		Uncached int `json:"uncached"`
	} `json:"requests"`
	Bandwidth struct {
		Total    int64 `json:"total"`
		Cached   int64 `json:"cached"`
		Uncached int64 `json:"uncached"`
	} `json:"bandwidth"`
	Threats struct {
		Total int `json:"total"`
	} `json:"threats"`
	Pageviews struct {
		Total int `json:"total"`
	} `json:"pageviews"`
	UniqueVisitors struct {
		All int `json:"all"`
	} `json:"uniques"`
}

func (s *Service) GetAnalytics(ctx context.Context, siteID, since string) (*ZoneAnalytics, error) {
	tok, err := s.token(ctx, siteID)
	if err != nil { return nil, err }
	zID, err := s.zoneID(ctx, siteID)
	if err != nil { return nil, err }

	// since: "-1440" (son 24 saat), "-10080" (son 7 gün) dakika cinsinden
	url := fmt.Sprintf("%s/zones/%s/analytics/dashboard?since=%s&until=0&continuous=true", cfBase, zID, since)
	r, err := s.do(ctx, "GET", url, tok, nil)
	if err != nil { return nil, err }

	var out struct {
		Totals ZoneAnalytics `json:"totals"`
	}
	json.Unmarshal(r.Result, &out)
	return &out.Totals, nil
}

// ── Hesap Doğrulama ───────────────────────────────────────────────────────────

func (s *Service) VerifyToken(ctx context.Context, token string) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", cfBase+"/user/tokens/verify", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	var out cfResponse
	json.NewDecoder(resp.Body).Decode(&out)
	if !out.Success {
		if len(out.Errors) > 0 {
			return fmt.Errorf("geçersiz token: %s", out.Errors[0].Message)
		}
		return fmt.Errorf("token doğrulanamadı")
	}
	return nil
}
