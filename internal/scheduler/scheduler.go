package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/backup"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// Service arka planda otomatik görevleri (global ve site bazlı yedeklemeler) yürütür.
type Service struct {
	st      *store.Store
	backups *backup.Service
	log     *slog.Logger
	stop    chan struct{}
}

func New(st *store.Store, backups *backup.Service, log *slog.Logger) *Service {
	return &Service{
		st:      st,
		backups: backups,
		log:     log,
		stop:    make(chan struct{}),
	}
}

// Start, arka plan döngüsünü ayrı bir goroutine olarak başlatır.
func (s *Service) Start() {
	go s.loop()
}

// Stop, döngüyü düzgünce kapatır.
func (s *Service) Stop() {
	close(s.stop)
}

func (s *Service) loop() {
	// Dakika başına bir kez kontrol: bir sonraki tam dakikaya kadar bekle.
	now := time.Now()
	initial := time.Until(now.Truncate(time.Minute).Add(time.Minute))
	select {
	case <-s.stop:
		return
	case <-time.After(initial):
	}

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	s.log.Info("zamanlayıcı servisi başlatıldı")

	for {
		select {
		case <-s.stop:
			return
		case t := <-ticker.C:
			s.checkAndRunBackups(t)
		}
	}
}

// checkAndRunBackups, hem global hem de site bazlı zamanlanmış yedekleri kontrol eder.
func (s *Service) checkAndRunBackups(now time.Time) {
	ctx := context.Background()
	// "15:04" → "03:00" gibi HH:MM; "monday" gibi haftalık gün adı
	currentHHMM := now.Format("15:04")
	currentDay := strings.ToLower(now.Weekday().String()) // "monday" … "sunday"

	// ── 1. Sunucu/Admin Global Yedeklemesi ──────────────────────────────────
	if gEnabled, ok, _ := s.st.GetSetting(ctx, "backup_global_enabled"); ok && gEnabled == "1" {
		gTime, _, _ := s.st.GetSetting(ctx, "backup_global_time")
		gFreq, _, _ := s.st.GetSetting(ctx, "backup_global_frequency") // "daily"|"monday"|"sunday"…

		if gTime == currentHHMM && freqMatches(gFreq, currentDay) {
			s.log.Info("global oto-yedek tetiklendi", "saat", currentHHMM)
			go s.runGlobalBackup()
		}
	}

	// ── 2. Site Bazlı Kullanıcı Yedeklemesi ─────────────────────────────────
	sites, err := s.st.ListSites(ctx)
	if err != nil {
		s.log.Error("zamanlayıcı: siteler alınamadı", "error", err)
		return
	}
	for _, site := range sites {
		siteID := site.ID
		enabled, ok, _ := s.st.GetSetting(ctx, fmt.Sprintf("site_backup_enabled_%s", siteID))
		if !ok || enabled != "1" {
			continue
		}
		sTime, _, _ := s.st.GetSetting(ctx, fmt.Sprintf("site_backup_time_%s", siteID))
		sFreq, _, _ := s.st.GetSetting(ctx, fmt.Sprintf("site_backup_frequency_%s", siteID))

		if sTime == currentHHMM && freqMatches(sFreq, currentDay) {
			s.log.Info("site oto-yedek tetiklendi", "site", siteID, "saat", currentHHMM)
			go s.runSiteBackup(siteID)
		}
	}
}

// freqMatches, "daily" veya belirli bir gün adı (ör. "monday") eşleşmesini kontrol eder.
func freqMatches(freq, currentDay string) bool {
	return freq == "daily" || freq == currentDay || freq == ""
}

func (s *Service) runGlobalBackup() {
	ctx := context.Background()
	sites, err := s.st.ListSites(ctx)
	if err != nil {
		s.log.Error("global yedek: siteler alınamadı", "error", err)
		return
	}
	for _, site := range sites {
		s.log.Info("global yedek: başlatılıyor", "site", site.ID)
		if _, err := s.backups.Run(ctx, site.ID, "full"); err != nil {
			s.log.Error("global yedek: hata", "site", site.ID, "error", err)
		}
	}
}

func (s *Service) runSiteBackup(siteID string) {
	ctx := context.Background()
	if _, err := s.backups.Run(ctx, siteID, "full"); err != nil {
		s.log.Error("site oto-yedek: hata", "site", siteID, "error", err)
	}
}
