package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/store"
)

// Client, diğer sunucularla haberleşmek için kullanılır (Cluster).
type Client struct {
	http *http.Client
}

// NewClient, mTLS/InsecureSkipVerify destekli cluster istemcisi oluşturur.
// (Phase 3'te basitlik için insecure kullanıyoruz, production'da TLS zorunlu).
func NewClient() *Client {
	return &Client{
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Cluster içi iletişimde (geliştirme için)
			},
		},
	}
}

func (c *Client) request(ctx context.Context, srv store.Server, method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}

	url := fmt.Sprintf("https://%s:8080%s", srv.IPAddress, path)
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+srv.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("agent API error: %s", resp.Status)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// CreateSiteOnServer, hedef sunucuda site oluşturur.
func (c *Client) CreateSiteOnServer(ctx context.Context, srv store.Server, payload any) (string, error) {
	var resp struct {
		ID string `json:"id"`
	}
	if err := c.request(ctx, srv, http.MethodPost, "/api/v1/sites", payload, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

// HealthCheck, hedef sunucunun durumunu kontrol eder.
func (c *Client) HealthCheck(ctx context.Context, srv store.Server) error {
	return c.request(ctx, srv, http.MethodGet, "/api/v1/health", nil, nil)
}
