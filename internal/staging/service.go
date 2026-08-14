package staging

import (
	"context"
	"fmt"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/auth"
	"github.com/mkoyazilim/aurapanel/internal/privclient"
	"github.com/mkoyazilim/aurapanel/internal/site"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

type Dependencies struct {
	Store   *store.Store
	Priv    *privclient.Client
	Audit   *audit.Service
	SiteMgr interface {
		Create(ctx context.Context, req site.CreateRequest) (string, error)
		Delete(ctx context.Context, id string) error
	}
}

type Service struct {
	deps Dependencies
}

func NewService(deps Dependencies) *Service {
	return &Service{deps: deps}
}

// CreateStaging creates a clone of the production site.
func (s *Service) CreateStaging(ctx context.Context, prodSiteID string, stagingDomain string) (*store.Site, error) {
	prod, err := s.deps.Store.GetSite(ctx, prodSiteID)
	if err != nil {
		return nil, fmt.Errorf("prod site error: %w", err)
	}

	var limits site.Limits
	
	// 1. Create a new site for staging
	stgSiteID, err := s.deps.SiteMgr.Create(ctx, site.CreateRequest{
		Domain:     stagingDomain,
		PHPVersion: "8.1",
		Limits:     limits,
	})
	if err != nil {
		return nil, fmt.Errorf("create staging site: %w", err)
	}

	stgSite, err := s.deps.Store.GetSite(ctx, stgSiteID)
	if err != nil {
		return nil, fmt.Errorf("get staging site: %w", err)
	}

	// 2. Clone database if exists (simplification: we just copy files for now)

	// 3. Clone files using privclient
	req := map[string]any{
		"src_site": prodSiteID,
		"dst_site": stgSite.ID,
	}
	_, err = s.deps.Priv.Call(ctx, "site.clone_files", req)
	if err != nil {
		// Rollback on failure
		_ = s.deps.SiteMgr.Delete(ctx, stgSite.ID)
		return nil, fmt.Errorf("file clone error: %w", err)
	}

	// 4. Record relationship
	env := &store.StagingEnvironment{
		ID:               auth.NewRequestID(),
		ProductionSiteID: prod.ID,
		StagingSiteID:    stgSite.ID,
		Status:           "active",
	}
	if err := s.deps.Store.InsertStagingEnvironment(ctx, env); err != nil {
		return nil, err
	}
	
	return stgSite, nil
}

// PushToProduction pushes files and DB from staging back to production.
func (s *Service) PushToProduction(ctx context.Context, prodSiteID string) error {
	env, err := s.deps.Store.GetActiveStagingEnvironment(ctx, prodSiteID)
	if err != nil {
		return fmt.Errorf("no active staging found: %w", err)
	}

	// Sync files from staging to production
	req := map[string]any{
		"src_site": env.StagingSiteID,
		"dst_site": prodSiteID,
	}
	_, err = s.deps.Priv.Call(ctx, "site.clone_files", req)
	if err != nil {
		return fmt.Errorf("file push error: %w", err)
	}
	
	return nil
}
