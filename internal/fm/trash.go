package fm

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

// TrashService, geri dönüşüm kutusu (FILE_MANAGER §6):
// silme varsayılan olarak KALICI DEĞİLDİR — dosyalar panel alanındaki
// trash köküne taşınır; retention (7/14/30 gün) sonrası otomatik temizlenir.
type TrashService struct {
	fs        *FileService
	root      string // panel alanı: <dataDir>/trash
	retention time.Duration
}

// NewTrashService, TrashService oluşturur (varsayılan retention: 14 gün).
func NewTrashService(fs *FileService, trashRoot string) *TrashService {
	return &TrashService{fs: fs, root: trashRoot, retention: 14 * 24 * time.Hour}
}

// SetRetention, saklama süresini günceller.
func (t *TrashService) SetRetention(d time.Duration) { t.retention = d }

// trashSiteDir, site trash dizini.
func (t *TrashService) trashSiteDir(siteID string) string { return path.Join(t.root, siteID) }

// MoveToTrash, dosya/dizini çöp kutusuna taşır. Hedef site FS'ten farklı
// bir dosya sisteminde olabileceğinden (üretim: /var/lib/aurapanel/trash)
// önce rename denenir, cross-device hatada kopyala+sil uygulanır.
func (t *TrashService) MoveToTrash(ctx context.Context, siteID, relPath string) (string, error) {
	if err := t.fs.allow("delete"); err != nil {
		return "", err
	}
	abs, err := t.fs.resolveAbs(ctx, siteID, relPath)
	if err != nil {
		return "", err
	}
	home, _ := t.fs.siteHome(siteID)
	if abs == path.Clean(home) {
		return "", ErrInvalidPath
	}

	destDir := t.trashSiteDir(siteID)
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return "", err
	}
	// Zaman damgalı benzersiz ad: çakışmasız, geri yükleme için izlenebilir.
	ts := strconv.FormatInt(time.Now().UnixNano(), 10)
	dest := path.Join(destDir, ts+"-"+path.Base(relPath))

	if err := os.Rename(abs, dest); err != nil {
		// Cross-device (üretim: site FS → /var/lib/aurapanel/trash):
		// özyinelemeli kopyala + kaynağı sil.
		if err := copyPath(abs, dest); err != nil {
			return "", err
		}
		if err := os.RemoveAll(abs); err != nil {
			return "", err
		}
	}
	t.fs.auditEvent(ctx, siteID, "trash", relPath, "success", map[string]any{"trash_path": dest})
	return dest, nil
}

// Restore, çöp kutusundaki girişi site root'una geri koyar.
func (t *TrashService) Restore(ctx context.Context, siteID, trashName string) error {
	destDir := t.trashSiteDir(siteID)
	src := path.Join(destDir, trashName)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("trash girişi yok: %s", trashName)
	}
	// trashName biçimi: <ts>-<orijinal_ad> → geri yükleme adı orijinal ad.
	orig := trashName[strings.Index(trashName, "-")+1:]
	if orig == "" {
		orig = trashName
	}
	destAbs, err := t.fs.resolveAbs(ctx, siteID, orig)
	if err != nil {
		return err
	}
	if err := os.Rename(src, destAbs); err != nil {
		return err
	}
	t.fs.auditEvent(ctx, siteID, "restore", orig, "success", nil)
	return nil
}

// Empty, sitenin tüm çöpünü boşaltır.
func (t *TrashService) Empty(ctx context.Context, siteID string) error {
	if err := t.fs.allow("delete"); err != nil {
		return err
	}
	if err := os.RemoveAll(t.trashSiteDir(siteID)); err != nil {
		return err
	}
	t.fs.auditEvent(ctx, siteID, "trash.empty", "", "success", nil)
	return nil
}

// copyPath, yerel özyinelemeli kopyalama (trash cross-device fallback).
func copyPath(src, dst string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		rf, err := os.Open(src)
		if err != nil {
			return err
		}
		defer rf.Close()
		w, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, st.Mode())
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, rf); err != nil {
			w.Close()
			return err
		}
		return w.Close()
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyPath(path.Join(src, e.Name()), path.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// Cleanup, retention'ı aşan girişleri siler (tüm siteler). Dönen değer:
// silinen giriş sayısı.
func (t *TrashService) Cleanup(ctx context.Context, now time.Time) (int, error) {
	siteDirs, err := os.ReadDir(t.root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, sd := range siteDirs {
		if !sd.IsDir() {
			continue
		}
		entries, err := os.ReadDir(path.Join(t.root, sd.Name()))
		if err != nil {
			continue
		}
		for _, e := range entries {
			tsPart := e.Name()
			if i := strings.Index(tsPart, "-"); i > 0 {
				tsPart = tsPart[:i]
			}
			ts, err := strconv.ParseInt(tsPart, 10, 64)
			if err != nil {
				continue
			}
			age := now.Sub(time.Unix(0, ts))
			if age > t.retention {
				if os.RemoveAll(path.Join(t.root, sd.Name(), e.Name())) == nil {
					removed++
				}
			}
		}
	}
	return removed, nil
}
