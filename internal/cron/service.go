// Package cron, site başına cron job yönetimini sağlar.
// Her job, site Linux kullanıcısı (www-<siteID>) ile çalıştırılır.
// Crontab içeriği priv helper üzerinden yazılır (doğrudan dosya yazmaz).
package cron

import (
	"context"
	"fmt"
	"strings"

	"github.com/mkoyazilim/aurapanel/internal/privclient"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// Service, cron job yönetim servisi.
type Service struct {
	st   *store.Store
	priv *privclient.Client
}

// NewService, Service oluşturur.
func NewService(st *store.Store, priv *privclient.Client) *Service {
	return &Service{st: st, priv: priv}
}

// List, bir sitenin cron job'larını listeler.
func (s *Service) List(ctx context.Context, siteID string) ([]store.CronJob, error) {
	return s.st.ListCronJobs(ctx, siteID)
}

// Create, yeni cron job ekler ve crontab'ı günceller.
func (s *Service) Create(ctx context.Context, siteID string, req CreateRequest) (int64, error) {
	if err := validateSchedule(req.Schedule); err != nil {
		return 0, err
	}
	if err := validateCommand(req.Command); err != nil {
		return 0, err
	}

	id, err := s.st.InsertCronJob(ctx, store.CronJob{
		SiteID:   siteID,
		Schedule: req.Schedule,
		Command:  req.Command,
		Label:    req.Label,
		Enabled:  true,
	})
	if err != nil {
		return 0, err
	}
	if err := s.syncCrontab(ctx, siteID); err != nil {
		return id, fmt.Errorf("cron oluşturuldu ama crontab güncellenemedi: %w", err)
	}
	return id, nil
}

// Update, mevcut cron job'ı günceller.
func (s *Service) Update(ctx context.Context, siteID string, id int64, req CreateRequest) error {
	if err := validateSchedule(req.Schedule); err != nil {
		return err
	}
	if err := validateCommand(req.Command); err != nil {
		return err
	}
	if err := s.st.UpdateCronJob(ctx, id, req.Schedule, req.Command, req.Label, req.Enabled); err != nil {
		return err
	}
	return s.syncCrontab(ctx, siteID)
}

// Delete, cron job siler ve crontab'ı günceller.
func (s *Service) Delete(ctx context.Context, siteID string, id int64) error {
	if err := s.st.DeleteCronJob(ctx, siteID, id); err != nil {
		return err
	}
	return s.syncCrontab(ctx, siteID)
}

// syncCrontab, sitenin tüm aktif cron job'larını priv helper üzerinden yazar.
func (s *Service) syncCrontab(ctx context.Context, siteID string) error {
	jobs, err := s.st.ListCronJobs(ctx, siteID)
	if err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString("# AuraPanel yönetimli crontab — elle düzenlemeyin.\n")
	sb.WriteString("SHELL=/bin/bash\n")
	sb.WriteString("PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin\n\n")

	for _, j := range jobs {
		if !j.Enabled {
			continue
		}
		sb.WriteString(fmt.Sprintf("%s %s\n", j.Schedule, j.Command))
	}

	_, err = s.priv.Call(ctx, "cron.apply", map[string]any{
		"site":    siteID,
		"content": sb.String(),
	})
	return err
}

// CreateRequest, yeni cron job isteği.
type CreateRequest struct {
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
	Label    string `json:"label"`
	Enabled  bool   `json:"enabled"`
}

// validateSchedule, cron ifadesini doğrular (5 alan).
func validateSchedule(s string) error {
	fields := strings.Fields(s)
	if len(fields) != 5 {
		return fmt.Errorf("geçersiz cron ifadesi: 5 alan gerekli (dakika saat gün ay haftanın-günü)")
	}
	for _, f := range fields {
		for _, c := range f {
			if !isCronChar(c) {
				return fmt.Errorf("geçersiz cron karakteri: %q", c)
			}
		}
	}
	return nil
}

func isCronChar(c rune) bool {
	return (c >= '0' && c <= '9') || c == '*' || c == '/' || c == '-' || c == ','
}

// validateCommand, komutu güvenlik açısından doğrular.
func validateCommand(cmd string) error {
	if strings.TrimSpace(cmd) == "" {
		return fmt.Errorf("komut boş olamaz")
	}
	forbidden := []string{";", "&&", "||", "|", "`", "$", ">", "<", "&", "$("}
	for _, f := range forbidden {
		if strings.Contains(cmd, f) {
			return fmt.Errorf("güvenli olmayan shell karakteri: %q", f)
		}
	}
	return nil
}
