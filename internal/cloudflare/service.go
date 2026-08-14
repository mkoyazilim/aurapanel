package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

type Dependencies struct {
	Store *store.Store
	Audit *audit.Service
}

type Service struct {
	deps Dependencies
	httpClient *http.Client
}

func NewService(deps Dependencies) *Service {
	return &Service{
		deps: deps,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// PurgeCache clears everything in the Cloudflare zone cache.
func (s *Service) PurgeCache(ctx context.Context, siteID string) error {
	settings, err := s.deps.Store.GetCloudflareSettings(ctx, siteID)
	if err != nil {
		return err
	}
	if settings == nil {
		return fmt.Errorf("cloudflare not configured for this site")
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/purge_cache", settings.ZoneID)
	
	reqData := map[string]bool{"purge_everything": true}
	bodyBytes, _ := json.Marshal(reqData)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+settings.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cloudflare api returned status: %d", resp.StatusCode)
	}

	s.deps.Audit.Write(ctx, audit.Event{
		Action: "cloudflare.purge_cache",
		Target: siteID,
	})
	return nil
}
