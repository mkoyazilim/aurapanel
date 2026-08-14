// Package php, PHP sürüm envanterini, site başına sürüm geçişini ve
// doğrulamalı php.ini yönetimini sağlar (ROADMAP W6).
package php

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/ols"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// privOps, php paketinin priv helper işlemleri.
type privOps interface {
	DetectPHP(ctx context.Context) (map[string]bool, error)
	InstallIni(ctx context.Context, siteID, content string) error
	ReadIni(ctx context.Context, siteID string) (string, error)
}

// vhostApplier, vhost uygulama (ols.Pipeline — rollback güvenceli).
type vhostApplier interface {
	Apply(ctx context.Context, v ols.Vhost) error
}

// Service, PHP yönetim servisi.
type Service struct {
	store     *store.Store
	priv      privOps
	vhost     vhostApplier
	audit     *audit.Service
	sitesRoot string
	certsRoot string
}

// NewService, Service oluşturur.
func NewService(st *store.Store, p privOps, v vhostApplier, au *audit.Service, sitesRoot, certsRoot string) *Service {
	return &Service{store: st, priv: p, vhost: v, audit: au, sitesRoot: sitesRoot, certsRoot: certsRoot}
}

// VersionInfo, tek PHP sürümünün panel görünümü.
type VersionInfo struct {
	Version    string
	BinaryPath string
	Status     string
	Installed  bool
}

// ListVersions, kayıtlı PHP sürümlerini gerçek kurulum durumuyla birlikte
// döndürür (priv php.detect ile birleştirilir).
func (s *Service) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	rows, err := s.store.ListPHPVersions(ctx)
	if err != nil {
		return nil, err
	}
	detected, err := s.priv.DetectPHP(ctx)
	if err != nil {
		return nil, fmt.Errorf("php.detect: %w", err)
	}
	out := make([]VersionInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, VersionInfo{
			Version:    r.Version,
			BinaryPath: r.BinaryPath,
			Status:     r.Status,
			Installed:  detected[r.Version],
		})
	}
	return out, nil
}

// SwitchVersion, bir sitenin PHP sürümünü değiştirir.
//
// Felsefe (ARCHITECTURE §5): SQLite desired state ÖNCE güncellenir,
// sistem sonra yakınsar. Vhost uygulaması rollback güvenceli pipeline
// üzerindendir; uygulama başarısız olursa desired state korunur ve bir
// sonraki drift taraması/Repair ile yeniden denenebilir — site asla
// yarım durumda kalmaz.
func (s *Service) SwitchVersion(ctx context.Context, siteID, version string) error {
	if !ols.SupportedPHP[version] {
		return fmt.Errorf("desteklenmeyen PHP sürümü: %q", version)
	}
	st, err := s.store.GetSite(ctx, siteID)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("site yok: %s", siteID)
	}
	phpID, err := s.store.GetPHPVersionID(ctx, version)
	if err != nil {
		return err
	}

	// Desired state: sites.php_version_id + php_pools.
	if err := s.store.SetSitePHPVersion(ctx, siteID, phpID); err != nil {
		return err
	}
	_, settings, ok, err := s.store.GetPHPool(ctx, siteID)
	if err != nil {
		return err
	}
	if !ok {
		settings = `{}`
	}
	if err := s.store.UpsertPHPool(ctx, siteID, phpID, settings); err != nil {
		return err
	}

	// Sistem yakınsar.
	v, err := s.buildVhost(ctx, st, version)
	if err != nil {
		return err
	}
	if err := s.vhost.Apply(ctx, v); err != nil {
		return fmt.Errorf("vhost uygulanamadı: %w (desired state korundu; Repair ile yinelenebilir)", err)
	}

	s.audit.Write(ctx, audit.Event{
		Action: "php.switch", Target: siteID, Result: "success",
		Extra: map[string]any{"version": version},
	})
	return nil
}

// SetIni, site php.ini ayarlarını doğrular, normalize eder ve uygular.
func (s *Service) SetIni(ctx context.Context, siteID string, settings map[string]string) error {
	st, err := s.store.GetSite(ctx, siteID)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("site yok: %s", siteID)
	}
	norm, err := ValidateSettings(settings)
	if err != nil {
		return err
	}
	content := RenderIni(norm)
	if err := s.priv.InstallIni(ctx, siteID, content); err != nil {
		return fmt.Errorf("php.ini kurulumu: %w", err)
	}
	settingsJSON, err := json.Marshal(norm)
	if err != nil {
		return err
	}
	if err := s.store.UpdatePHPoolSettings(ctx, siteID, string(settingsJSON)); err != nil {
		return err
	}
	s.audit.Write(ctx, audit.Event{Action: "php.ini.update", Target: siteID, Result: "success"})
	return nil
}

// GetIni, site php.ini içeriğini döndürür (editör için).
func (s *Service) GetIni(ctx context.Context, siteID string) (string, error) {
	return s.priv.ReadIni(ctx, siteID)
}

// buildVhost, site kaydından vhost desired state'ini üretir.
func (s *Service) buildVhost(ctx context.Context, st *store.Site, version string) (ols.Vhost, error) {
	domains, err := s.store.ListDomainsBySite(ctx, st.ID)
	if err != nil {
		return ols.Vhost{}, err
	}
	aliases := []string{}
	for _, d := range domains {
		if d.Kind == "alias" {
			aliases = append(aliases, d.Domain)
		}
	}
	return ols.Vhost{
		SiteID:     st.ID,
		Domain:     st.Name,
		Aliases:    aliases,
		PHPVersion: version,
		IndexFiles: []string{"index.php", "index.html"},
	}, nil
}
