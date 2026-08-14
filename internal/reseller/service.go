// Package reseller, reseller iş mantığı katmanını sağlar.
package reseller

import (
	"context"
	"errors"
	"fmt"

	"github.com/mkoyazilim/aurapanel/internal/store"
)

// Service, reseller operasyonları için iş mantığı katmanıdır.
type Service struct {
	store *store.Store
}

// New, verilen store üzerinde çalışan bir Service döner.
func New(s *store.Store) *Service { return &Service{store: s} }

// CheckSiteQuota, resellerID sahibinin site kotasının dolup dolmadığını kontrol eder.
// Kota atanmamışsa veya limit aşılmışsa error döner.
func (svc *Service) CheckSiteQuota(ctx context.Context, resellerID int64) error {
	quota, err := svc.store.GetResellerQuota(ctx, resellerID)
	if err != nil {
		return fmt.Errorf("kota sorgulanamadı: %w", err)
	}
	if quota == nil {
		return errors.New("reseller has no quota assigned")
	}

	usage, err := svc.store.GetResellerUsage(ctx, resellerID)
	if err != nil {
		return fmt.Errorf("kullanım sorgulanamadı: %w", err)
	}

	if usage.Sites >= quota.MaxSites {
		return fmt.Errorf("site quota exceeded (%d/%d)", usage.Sites, quota.MaxSites)
	}
	return nil
}

// CheckDatabaseQuota, resellerID sahibinin veritabanı kotasının dolup dolmadığını kontrol eder.
// Kota atanmamışsa veya limit aşılmışsa error döner.
func (svc *Service) CheckDatabaseQuota(ctx context.Context, resellerID int64) error {
	quota, err := svc.store.GetResellerQuota(ctx, resellerID)
	if err != nil {
		return fmt.Errorf("kota sorgulanamadı: %w", err)
	}
	if quota == nil {
		return errors.New("reseller has no quota assigned")
	}

	usage, err := svc.store.GetResellerUsage(ctx, resellerID)
	if err != nil {
		return fmt.Errorf("kullanım sorgulanamadı: %w", err)
	}

	if usage.Databases >= quota.MaxDatabases {
		return fmt.Errorf("database quota exceeded (%d/%d)", usage.Databases, quota.MaxDatabases)
	}
	return nil
}

// AssignQuota, bir reseller'a kota atar ya da mevcutu günceller.
func (svc *Service) AssignQuota(ctx context.Context, resellerID int64, q store.ResellerQuota) error {
	q.ResellerID = resellerID
	if err := svc.store.UpsertResellerQuota(ctx, q); err != nil {
		return fmt.Errorf("kota kaydedilemedi: %w", err)
	}
	return nil
}

// GetQuotaWithUsage, reseller'ın mevcut kotasını ve anlık kullanımını birlikte döner.
// quota nil olabilir (henüz atanmamış); bu durum hata sayılmaz.
func (svc *Service) GetQuotaWithUsage(ctx context.Context, resellerID int64) (quota *store.ResellerQuota, usage store.ResellerUsage, err error) {
	quota, err = svc.store.GetResellerQuota(ctx, resellerID)
	if err != nil {
		err = fmt.Errorf("kota sorgulanamadı: %w", err)
		return
	}

	usage, err = svc.store.GetResellerUsage(ctx, resellerID)
	if err != nil {
		err = fmt.Errorf("kullanım sorgulanamadı: %w", err)
		return
	}
	return
}
