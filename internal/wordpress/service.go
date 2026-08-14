// Package wordpress, tek tıkla WordPress kurulum motorudur.
// Otomatik veritabanı, kullanıcı ve yetkilendirme sağlar; resmi WordPress
// çekirdeğini indirir, çıkartır, wp-config.php dosyasını kriptografik salt'larla üretir
// ve dosya sahipliğini site kullanıcısına verir.
package wordpress

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/db"
	"github.com/mkoyazilim/aurapanel/internal/privclient"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// Service, WordPress kurulum servisi.
type Service struct {
	st        *store.Store
	dbSvc     *db.Service
	priv      *privclient.Client
	sitesRoot string
	audit     *audit.Service
}

// NewService, WordPress Service oluşturur.
func NewService(st *store.Store, dbSvc *db.Service, priv *privclient.Client, sitesRoot string, au *audit.Service) *Service {
	return &Service{
		st:        st,
		dbSvc:     dbSvc,
		priv:      priv,
		sitesRoot: sitesRoot,
		audit:     au,
	}
}

// InstallRequest, kurulum isteği parametreleri.
type InstallRequest struct {
	Language    string `json:"language"`     // "tr" | "en"
	TablePrefix string `json:"table_prefix"` // varsayılan "wp_"
}

// InstallResult, kurulum sonucu.
type InstallResult struct {
	DBName     string `json:"db_name"`
	DBUser     string `json:"db_user"`
	DBPassword string `json:"db_password"`
	HomeDir    string `json:"home_dir"`
	Language   string `json:"language"`
}

// Install, WordPress'i site home dizinine kurar.
func (s *Service) Install(ctx context.Context, siteID string, req InstallRequest) (*InstallResult, error) {
	siteRecord, err := s.st.GetSite(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("site bilgisi alınamadı: %w", err)
	}
	if siteRecord == nil {
		return nil, fmt.Errorf("site bulunamadı: %s", siteID)
	}

	if req.TablePrefix == "" {
		req.TablePrefix = "wp_"
	}
	if req.Language != "tr" {
		req.Language = "en"
	}

	docRoot := path.Join(s.sitesRoot, siteID, "home")
	if err := os.MkdirAll(docRoot, 0o750); err != nil {
		return nil, fmt.Errorf("home dizini oluşturulamadı: %w", err)
	}

	// 1. Rastgele veritabanı şifresi üret
	dbPass, err := randomString(24)
	if err != nil {
		return nil, fmt.Errorf("şifre üretilemedi: %w", err)
	}

	dbNameSuffix := "wp"
	fullDBName := siteID + "_" + dbNameSuffix
	fullDBUser := fullDBName

	// 2. MariaDB veritabanı ve kullanıcısı oluştur
	_ = s.dbSvc.CreateDatabase(ctx, siteID, dbNameSuffix)
	_, _ = s.dbSvc.CreateUser(ctx, siteID, dbNameSuffix, dbPass)
	_ = s.dbSvc.Grant(ctx, siteID, fullDBName, fullDBUser)

	// 3. WordPress paketini indir ve aç
	downloadURL := "https://wordpress.org/latest.tar.gz"
	if req.Language == "tr" {
		downloadURL = "https://tr.wordpress.org/latest-tr_TR.tar.gz"
	}

	if err := s.downloadAndExtract(ctx, downloadURL, docRoot); err != nil {
		return nil, fmt.Errorf("wordpress indirilemedi/açılamadı: %w", err)
	}

	// 4. wp-config.php üret
	configContent, err := generateWpConfig(fullDBName, fullDBUser, dbPass, req.TablePrefix, req.Language)
	if err != nil {
		return nil, fmt.Errorf("wp-config üretilemedi: %w", err)
	}

	configPath := filepath.Join(docRoot, "wp-config.php")
	if err := os.WriteFile(configPath, []byte(configContent), 0o640); err != nil {
		return nil, fmt.Errorf("wp-config.php yazılamadı: %w", err)
	}

	// 5. Dosya izinlerini ve sahipliğini düzelt (chown -R www-<siteID>)
	user := "www-" + siteID
	_, _ = s.priv.Call(ctx, "site.prepare", map[string]any{
		"site": siteID,
		"user": user,
	})

	s.audit.Write(ctx, audit.Event{
		Action: "wp.install",
		Target: siteID,
		Result: "success",
		Extra: map[string]any{
			"db":       fullDBName,
			"language": req.Language,
		},
	})

	return &InstallResult{
		DBName:     fullDBName,
		DBUser:     fullDBUser,
		DBPassword: dbPass,
		HomeDir:    docRoot,
		Language:   req.Language,
	}, nil
}

// downloadAndExtract, tar.gz arşivini indirir ve 'wordpress/' kökünü kırparak targetDir'e açar.
func (s *Service) downloadAndExtract(ctx context.Context, url, targetDir string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d: %s", resp.StatusCode, resp.Status)
	}

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// WordPress arşivleri 'wordpress/' kök diziniyle gelir. Onu çıkarıyoruz:
		relPath := header.Name
		if strings.HasPrefix(relPath, "wordpress/") {
			relPath = strings.TrimPrefix(relPath, "wordpress/")
		}
		if relPath == "" {
			continue
		}

		destPath := filepath.Join(targetDir, relPath)

		// Path traversal güvenlik kontrolü
		if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(targetDir)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(destPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}

func generateWpConfig(dbName, dbUser, dbPass, tablePrefix, lang string) (string, error) {
	salts := make([]string, 8)
	for i := 0; i < 8; i++ {
		s, err := randomString(64)
		if err != nil {
			return "", err
		}
		salts[i] = s
	}

	return fmt.Sprintf(`<?php
/**
 * WordPress yapılandırma dosyası — AuraPanel otomatik üretimi.
 */

define('DB_NAME', '%s');
define('DB_USER', '%s');
define('DB_PASSWORD', '%s');
define('DB_HOST', 'localhost');
define('DB_CHARSET', 'utf8mb4');
define('DB_COLLATE', '');

/** Güvenlik Anahtarları ve Salt'lar */
define('AUTH_KEY',         '%s');
define('SECURE_AUTH_KEY',  '%s');
define('LOGGED_IN_KEY',    '%s');
define('NONCE_KEY',        '%s');
define('AUTH_SALT',        '%s');
define('SECURE_AUTH_SALT', '%s');
define('LOGGED_IN_SALT',   '%s');
define('NONCE_SALT',       '%s');

$table_prefix = '%s';

define('WP_DEBUG', false);
define('FS_METHOD', 'direct');

if ( ! defined( 'ABSPATH' ) ) {
	define( 'ABSPATH', __DIR__ . '/' );
}

require_once ABSPATH . 'wp-settings.php';
`,
		dbName, dbUser, dbPass,
		salts[0], salts[1], salts[2], salts[3],
		salts[4], salts[5], salts[6], salts[7],
		tablePrefix,
	), nil
}

func randomString(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:n], nil
}
