package dns

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Config, PowerDNS API ayarları.
type Config struct {
	Endpoint string
	APIKey   string
	ServerID string // usually "localhost"
}

// Client, PowerDNS API istemcisi.
type Client struct {
	cfg  Config
	http *http.Client
}

func NewClient(cfg Config) *Client {
	if cfg.ServerID == "" {
		cfg.ServerID = "localhost"
	}
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}

	url := fmt.Sprintf("%s/api/v1/servers/%s%s", c.cfg.Endpoint, c.cfg.ServerID, path)
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("powerdns api error: %s", resp.Status)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode pdns response: %w", err)
		}
	}
	return nil
}

// CreateZone, yeni bir DNS zone (alan adı) oluşturur.
func (c *Client) CreateZone(ctx context.Context, domain string, nameservers []string) error {
	payload := map[string]any{
		"name":        domain + ".",
		"kind":        "Native",
		"nameservers": nameservers,
	}
	return c.do(ctx, http.MethodPost, "/zones", payload, nil)
}

// DeleteZone, bir zone siler.
func (c *Client) DeleteZone(ctx context.Context, domain string) error {
	return c.do(ctx, http.MethodDelete, "/zones/"+domain+".", nil, nil)
}

// AddRecord, bir DNS kaydı ekler/günceller.
func (c *Client) AddRecord(ctx context.Context, domain, name, rtype string, ttl int, content string) error {
	payload := map[string]any{
		"rrsets": []map[string]any{
			{
				"name":       name + ".",
				"type":       rtype,
				"ttl":        ttl,
				"changetype": "REPLACE",
				"records": []map[string]any{
					{"content": content, "disabled": false},
				},
			},
		},
	}
	return c.do(ctx, http.MethodPatch, "/zones/"+domain+".", payload, nil)
}

// Zone, PowerDNS zone kaydı.
type Zone struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Kind           string   `json:"kind"`
	DNSSECEnabled  bool     `json:"dnssec"`
	Serial         uint32   `json:"serial"`
	Nameservers    []string `json:"nameservers,omitempty"`
}

// RRSet, bir DNS kayıt seti.
type RRSet struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	TTL        int      `json:"ttl"`
	Changetype string   `json:"changetype,omitempty"`
	Records    []Record `json:"records,omitempty"`
	Comments   []any    `json:"comments,omitempty"`
}

// Record, tek DNS kaydı.
type Record struct {
	Content  string `json:"content"`
	Disabled bool   `json:"disabled"`
}

// ListZones, tüm zone'ları listeler.
func (c *Client) ListZones(ctx context.Context) ([]Zone, error) {
	var zones []Zone
	if err := c.do(ctx, http.MethodGet, "/zones", nil, &zones); err != nil {
		return nil, err
	}
	return zones, nil
}

// GetZone, tek zone döndürür (rrsets dahil).
func (c *Client) GetZone(ctx context.Context, domain string) (*Zone, error) {
	var zone Zone
	if err := c.do(ctx, http.MethodGet, "/zones/"+domain+".", nil, &zone); err != nil {
		return nil, err
	}
	return &zone, nil
}

// DeleteRecord, belirtilen tip+isim için rrset'i siler.
func (c *Client) DeleteRecord(ctx context.Context, domain, name, rtype string) error {
	payload := map[string]any{
		"rrsets": []map[string]any{
			{"name": name + ".", "type": rtype, "changetype": "DELETE"},
		},
	}
	return c.do(ctx, http.MethodPatch, "/zones/"+domain+".", payload, nil)
}

// PatchRRSets, zone'a birden fazla rrset uygular (REPLACE veya DELETE).
func (c *Client) PatchRRSets(ctx context.Context, domain string, rrsets []RRSet) error {
	return c.do(ctx, http.MethodPatch, "/zones/"+domain+".", map[string]any{"rrsets": rrsets}, nil)
}

// UpdateSOA, zone'un SOA kaydını günceller.
func (c *Client) UpdateSOA(ctx context.Context, domain, mname, rname string, ttl int) error {
	content := fmt.Sprintf("%s %s 1 10800 3600 604800 3600", mname, rname)
	return c.AddRecord(ctx, domain, domain, "SOA", ttl, content)
}

// EnableDNSSEC, zone için DNSSEC'i etkinleştirir.
func (c *Client) EnableDNSSEC(ctx context.Context, domain string) error {
	return c.do(ctx, http.MethodPut, "/zones/"+domain+".", map[string]any{"dnssec": true}, nil)
}

// DisableDNSSEC, zone için DNSSEC'i devre dışı bırakır.
func (c *Client) DisableDNSSEC(ctx context.Context, domain string) error {
	return c.do(ctx, http.MethodPut, "/zones/"+domain+".", map[string]any{"dnssec": false}, nil)
}

// RectifyZone, DNSSEC zone'unu yeniden hesaplar (key rollover sonrası).
func (c *Client) RectifyZone(ctx context.Context, domain string) error {
	return c.do(ctx, http.MethodPut, "/zones/"+domain+"/rectify", nil, nil)
}

// CryptoKey, DNSSEC anahtar kaydı.
type CryptoKey struct {
	ID        int    `json:"id"`
	KeyType   string `json:"keytype"`
	Active    bool   `json:"active"`
	Published bool   `json:"published"`
	Algorithm string `json:"algorithm"`
	Bits      int    `json:"bits"`
	DS        []string `json:"ds,omitempty"`
	DNSKEY    string `json:"dnskey,omitempty"`
}

// ListCryptoKeys, zone'un DNSSEC anahtarlarını listeler.
func (c *Client) ListCryptoKeys(ctx context.Context, domain string) ([]CryptoKey, error) {
	var keys []CryptoKey
	if err := c.do(ctx, http.MethodGet, "/zones/"+domain+"/cryptokeys", nil, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

// AddCryptoKey, yeni DNSSEC anahtarı ekler.
func (c *Client) AddCryptoKey(ctx context.Context, domain, keyType, algorithm string, bits int) (*CryptoKey, error) {
	body := map[string]any{
		"keytype":   keyType,   // "ksk" | "zsk"
		"active":    true,
		"published": true,
		"algorithm": algorithm, // "ecdsa256" | "rsasha256" vb.
		"bits":      bits,
	}
	var key CryptoKey
	if err := c.do(ctx, http.MethodPost, "/zones/"+domain+"/cryptokeys", body, &key); err != nil {
		return nil, err
	}
	return &key, nil
}

// DeleteCryptoKey, DNSSEC anahtarını siler (key rollover).
func (c *Client) DeleteCryptoKey(ctx context.Context, domain string, keyID int) error {
	return c.do(ctx, fmt.Sprintf("%s", http.MethodDelete),
		fmt.Sprintf("/zones/%s./cryptokeys/%d", domain, keyID), nil, nil)
}

// ExportZone, zone'u AXFR formatında döndürür.
func (c *Client) ExportZone(ctx context.Context, domain string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/servers/%s/zones/%s./export", c.cfg.Endpoint, c.cfg.ServerID, domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-API-Key", c.cfg.APIKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("export error: %s", resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}
