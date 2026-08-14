// AuraPanel — OpenLiteSpeed için hafif, güvenlik öncelikli hosting control panel.
//
// W1 iskeleti: config + logger + SQLite (şema v1) + audit altyapısı.
// HTTP katmanı, priv helper ve OLS entegrasyonu sonraki paketlerde (ROADMAP).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/config"
	"github.com/mkoyazilim/aurapanel/internal/logger"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// Derleme zamanında -ldflags ile doldurulur (bkz. Makefile).
var (
	version = "dev"
	commit  = "none"
	built   = "unknown"
)

func main() {
	cfgPath := flag.String("config", "", "yapılandırma dosyası (varsayılan: /etc/aurapanel/aurapanel.yaml)")
	check := flag.Bool("check", false, "başlatma kontrolünü yap ve çık (smoke test)")
	showVersion := flag.Bool("version", false, "sürümü yazdır ve çık")
	flag.Parse()

	if *showVersion {
		fmt.Printf("aurapanel %s (commit=%s built=%s)\n", version, commit, built)
		return
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Log.Level, cfg.Log.Format, os.Stderr)
	log.Info("aurapanel başlatılıyor",
		"version", version, "commit", commit,
		"listen", cfg.Listen.Address, "mode", cfg.Listen.Mode)

	st, err := store.Open(cfg.Database.Path)
	if err != nil {
		log.Error("veritabanı açılamadı", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	if err := st.Migrate(); err != nil {
		log.Error("migration başarısız", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	au := audit.New(st)
	if err := au.Write(ctx, audit.Event{
		User:   "system",
		Action: "panel.start",
		Target: "panel",
		Extra:  map[string]any{"version": version, "commit": commit},
	}); err != nil {
		log.Error("audit kaydı yazılamadı", "error", err)
		os.Exit(1)
	}

	// DoD kanıtı (ROADMAP W1): yazılan audit kaydı geri okunabilmeli.
	events, err := au.List(ctx, 3)
	if err != nil {
		log.Error("audit kayıtları okunamadı", "error", err)
		os.Exit(1)
	}
	for _, e := range events {
		log.Info("audit",
			"id", e.ID, "action", e.Action, "user", e.User,
			"target", e.Target, "result", e.Result, "request_id", e.RequestID)
	}

	if *check {
		log.Info("kontrol tamam: şema v1 kuruldu, audit yazıldı ve okundu")
		return
	}

	log.Info("çalışıyor; çıkış için Ctrl+C")
	<-ctx.Done()
	log.Info("kapatılıyor")
}
