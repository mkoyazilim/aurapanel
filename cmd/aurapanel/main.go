// AuraPanel — OpenLiteSpeed için hafif, güvenlik öncelikli hosting control panel.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql" // MariaDB sürücüsü (clean-room, MPL-2.0)

	"github.com/mkoyazilim/aurapanel/internal/api"
	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/auth"
	"github.com/mkoyazilim/aurapanel/internal/backup"
	"github.com/mkoyazilim/aurapanel/internal/config"
	"github.com/mkoyazilim/aurapanel/internal/crypto"
	"github.com/mkoyazilim/aurapanel/internal/db"
	"github.com/mkoyazilim/aurapanel/internal/drift"
	"github.com/mkoyazilim/aurapanel/internal/fm"
	"github.com/mkoyazilim/aurapanel/internal/logger"
	"github.com/mkoyazilim/aurapanel/internal/ols"
	"github.com/mkoyazilim/aurapanel/internal/php"
	"github.com/mkoyazilim/aurapanel/internal/priv"
	"github.com/mkoyazilim/aurapanel/internal/privclient"
	"github.com/mkoyazilim/aurapanel/internal/site"
	"github.com/mkoyazilim/aurapanel/internal/ssl"
	"github.com/mkoyazilim/aurapanel/internal/store"
	"github.com/mkoyazilim/aurapanel/internal/update"
	"github.com/mkoyazilim/aurapanel/internal/webui"
)

var (
	version = "dev"
	commit  = "none"
	built   = "unknown"
)

const privSocket = "/run/aurapanel/priv.sock"

func main() {
	// file.op worker modu: helper, kendini site UID'siyle yeniden başlatır;
	// env yalnızca helper TARAFINDAN konur (argv[0] burada "aurapanel"
	// olduğundan priv dispatch'ine girmez).
	if os.Getenv("AURAPANEL_FILE_WORKER") == "1" {
		os.Exit(priv.WorkerMain(os.Args[1:]))
	}

	// Tek binary, iki mod: "aurapanel-priv" symlink'i → priv helper.
	if filepath.Base(os.Args[0]) == "aurapanel-priv" {
		os.Exit(priv.Main(os.Args[1:]))
	}

	cfgPath := flag.String("config", "", "yapılandırma dosyası (varsayılan: /etc/aurapanel/aurapanel.yaml)")
	check := flag.Bool("check", false, "başlatma kontrolünü yap ve çık")
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

	// Master key: encrypted-at-rest sırlarının anahtarı (kurulum üretir;
	// yoksa ilk başlatmada üretilir — dosya DB'de ASLA bulunmaz).
	keyPath := filepath.Join(cfg.Paths.DataDir, "keys", "master.key")
	if _, err := os.Stat(keyPath); errors.Is(err, os.ErrNotExist) {
		key, err := crypto.GenerateKey()
		if err != nil {
			log.Error("anahtar üretilemedi", "error", err)
			os.Exit(1)
		}
		if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
			log.Error("keys dizini", "error", err)
			os.Exit(1)
		}
		if err := os.WriteFile(keyPath, key, 0o600); err != nil {
			log.Error("anahtar yazılamadı", "error", err)
			os.Exit(1)
		}
		log.Warn("master key üretildi", "path", keyPath)
	}
	cipher, err := crypto.LoadKey(keyPath)
	if err != nil {
		log.Error("master key yüklenemedi", "error", err)
		os.Exit(1)
	}

	// Bootstrap admin: ilk kurulumda rastgele kimlik bilgileri üretilir,
	// yalnızca BİR KEZ yazdırılır, ilk girişte şifre değişimi ZORUNLUDUR.
	if err := bootstrapAdmin(ctx, st, au, log); err != nil {
		log.Error("bootstrap admin", "error", err)
		os.Exit(1)
	}

	if *check {
		log.Info("kontrol tamam: şema kuruldu, bootstrap hazır")
		return
	}

	sessions := auth.NewSessionStore(st)
	privC := privclient.New(privSocket, 30*time.Second)

	// OLS pipeline: priv üzerinden, loopback health probe'lu.
	sitesRoot := cfg.Paths.SitesRoot
	certsRoot := filepath.Join(cfg.Paths.DataDir, "state", "certs")
	pipeline := ols.NewPipeline(sitesRoot, certsRoot, ols.NewPrivInstaller(privC),
		&ols.HTTPProber{Timeout: 10 * time.Second})

	siteMgr := site.NewManager(st, site.NewPrivOps(privC), pipeline, au, sitesRoot)

	// Dosya yöneticisi backend seçimi: priv helper erişilebilirse Tier-1
	// (site UID + cgroup — ÜRETİM), değilse LocalBackend (yalnızca DEV).
	var fmBackend fm.Backend
	if _, err := privC.Call(ctx, "priv.ping", nil); err == nil {
		fmBackend = fm.NewPrivBackend(privC, sitesRoot)
		log.Info("fm backend: Tier-1 (priv file.op, site UID + cgroup)")
	} else {
		fmBackend = fm.NewLocalBackend()
		log.Warn("fm backend: LocalBackend (DEV) — priv helper erişilemedi; üretimde Tier-1 şart")
	}
	files := fm.New(fmBackend, au, sitesRoot)
	uploads := fm.NewUploadService(files, filepath.Join(cfg.Paths.DataDir, "uploads"))
	archives := fm.NewArchiveService(files)
	trash := fm.NewTrashService(files, cfg.Paths.TrashDir)

	// MariaDB: yönetim bağlantısı unix socket üzerinden.
	mysqlDB, err := sql.Open("mysql", fmt.Sprintf("%s:%s@unix(%s)/",
		cfg.MariaDB.User, cfg.MariaDB.Password, cfg.MariaDB.Socket))
	if err != nil {
		log.Error("mariadb bağlantısı", "error", err)
		os.Exit(1)
	}
	dbSvc := db.NewService(st, db.NewMariaDBEngine(mysqlDB), cipher, au)
	phpSvc := php.NewService(st, php.NewPrivOps(privC), pipeline, au, sitesRoot, certsRoot)
	sslSvc := ssl.NewService(st, ssl.NewCertStore(certsRoot), nil, pipeline, au, sitesRoot, certsRoot, 30*24*time.Hour)

	// Yedekler: master key'i yedek anahtarı olarak kullan (Faz 2'de ayrı
	// backup key yönetimi gelecek).
	backupKey, err := os.ReadFile(keyPath)
	if err != nil {
		log.Error("backup key", "error", err)
		os.Exit(1)
	}
	bkSvc, err := backup.NewService(st, backup.NewLocalStorage(cfg.Paths.BackupDir),
		backupKey, archives, nil, au, 7)
	if err != nil {
		log.Error("backup servisi", "error", err)
		os.Exit(1)
	}

	dc := drift.NewPrivCollector(privC, sitesRoot, certsRoot)
	repairer := drift.NewRepairer(st, dc, sitesRoot, certsRoot)
	scanner := drift.NewScanner(st, dc, sitesRoot, certsRoot, au, repairer)

	// Güncelleme merkezi: manifest, ikili dağıtım reposundan (pinned).
	exe, err := os.Executable()
	if err != nil {
		exe = "/usr/local/sbin/aurapanel"
	}
	upd := update.NewService(&update.HTTPFetcher{},
		"https://github.com/mkoyazilim/downloadaurapanel/releases/latest/download/versions.json",
		version, exe)

	srv := api.New(api.Deps{
		Store: st, Audit: au, Sessions: sessions, Cipher: cipher, Cfg: cfg, Log: log,
		Web: webui.Handler(),
		Sites: siteMgr, Files: files, Uploads: uploads, Archive: archives, Trash: trash,
		PHP: phpSvc, DB: dbSvc, SSL: sslSvc, Backups: bkSvc,
		DriftScan: scanner, DriftFix: repairer, Updates: upd,
	})

	httpSrv := &http.Server{
		Addr:              cfg.Listen.Address,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Info("panel dinliyor", "addr", cfg.Listen.Address, "mode", cfg.Listen.Mode)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http sunucusu", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("kapatılıyor")
	shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpSrv.Shutdown(shCtx)
}

// bootstrapAdmin, kullanıcı yoksa admin üretir ve yalnızca bir kez yazdırır.
func bootstrapAdmin(ctx context.Context, st *store.Store, au *audit.Service, log *slog.Logger) error {
	n, err := st.CountUsers(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	roleID, err := st.GetRoleIDByName(ctx, "admin")
	if err != nil {
		return err
	}
	username := "admin-" + auth.NewRequestID()[:8]
	password := auth.NewRequestID() + auth.NewRequestID()
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if _, err := st.InsertUser(ctx, store.User{
		Username: username, PasswordHash: hash, RoleID: roleID,
		MustChangePassword: true, Status: "active",
	}); err != nil {
		return err
	}
	au.Write(ctx, audit.Event{Action: "auth.bootstrap", Target: "panel", Result: "success"})
	fmt.Fprintf(os.Stderr, "\n==============================================\n")
	fmt.Fprintf(os.Stderr, " AuraPanel ilk kurulum bilgileri\n")
	fmt.Fprintf(os.Stderr, " Kullanıcı adı: %s\n", username)
	fmt.Fprintf(os.Stderr, " Şifre:         %s\n", password)
	fmt.Fprintf(os.Stderr, " İLK GİRİŞTE ŞİFRE DEĞİŞTİRMEK ZORUNLUDUR.\n")
	fmt.Fprintf(os.Stderr, " Bu bilgiler yalnızca BİR KEZ gösterilir.\n")
	fmt.Fprintf(os.Stderr, "==============================================\n\n")
	return nil
}

