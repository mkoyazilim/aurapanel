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
