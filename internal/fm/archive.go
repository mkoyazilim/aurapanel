package fm

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// ArchiveLimits, archive bomb korumaları (FILE_MANAGER §12).
type ArchiveLimits struct {
	MaxTotalSize   int64 // açılan toplam bayt
	MaxFiles       int   // dosya sayısı
	MaxRatio       int   // sıkıştırma oranı (arşiv:çıktı)
	MaxEntryPath   int   // tek girdi yol uzunluğu
}

// DefaultArchiveLimits: 10 GiB çıktı, 100 bin dosya, 200:1 oran.
func DefaultArchiveLimits() ArchiveLimits {
	return ArchiveLimits{MaxTotalSize: 10 << 30, MaxFiles: 100_000, MaxRatio: 200, MaxEntryPath: 4096}
}

// ArchiveService, ZIP/TAR.GZ oluşturma ve çıkarma (FILE_MANAGER §12).
// Extraction HİÇBİR KOŞULDA hedef dizin dışına yazamaz (zip-slip koruması);
// symlink girdileri reddedilir; boyut/sayı/oran sınırları uygulanır.
type ArchiveService struct {
	fs     *FileService
	limits ArchiveLimits
}

// NewArchiveService, ArchiveService oluşturur.
func NewArchiveService(fs *FileService) *ArchiveService {
	return &ArchiveService{fs: fs, limits: DefaultArchiveLimits()}
}

// SetLimits, limitleri günceller (testlerde küçültülür).
func (a *ArchiveService) SetLimits(l ArchiveLimits) { a.limits = l }

// Create, site dosyalarından arşiv üretir (format: "zip" | "targz").
// Arşiv, site root dışındaki bir staging alanında kurulur ve bitince
// hedefe atomik taşınır — kısmi arşiv siteye yazılmaz.
func (a *ArchiveService) Create(ctx context.Context, siteID, targetRel, format string, sources []string) error {
	if err := a.fs.allow("archive"); err != nil {
		return err
	}
	if format != "zip" && format != "targz" {
		return fmt.Errorf("desteklenmeyen format: %q", format)
	}
	// Kaynaklar önce doğrulanır (hepsi root içinde).
	absSources := make([]string, 0, len(sources))
	for _, src := range sources {
		abs, err := a.fs.resolveAbs(ctx, siteID, src)
		if err != nil {
			return err
		}
		absSources = append(absSources, abs)
	}

	staging, err := os.MkdirTemp("", "aurapanel-archive-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	tmpArchive := path.Join(staging, "out")

	switch format {
	case "zip":
		err = a.createZip(ctx, siteID, tmpArchive, sources, absSources)
	case "targz":
		err = a.createTarGz(ctx, siteID, tmpArchive, sources, absSources)
	}
	if err != nil {
		return err
	}

	if err := a.fs.copyAbsFile(ctx, siteID, tmpArchive, targetRel); err != nil {
		return err
	}
	a.fs.auditEvent(ctx, siteID, "archive", targetRel, "success", map[string]any{"format": format, "sources": len(sources)})
	return nil
}

func (a *ArchiveService) createZip(ctx context.Context, siteID, out string, rels, abses []string) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()

	home, _ := a.fs.siteHome(siteID)
	for i, abs := range abses {
		prefix := path.Dir(rels[i])
		if err := a.fs.walkArchive(ctx, home, abs, func(rel, full string, info os.FileInfo) error {
			entryName := path.Join(prefix, rel)
			if info.IsDir() {
				_, err := zw.Create(entryName + "/")
				return err
			}
			w, err := zw.Create(entryName)
			if err != nil {
				return err
			}
			return streamFile(w, full)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (a *ArchiveService) createTarGz(ctx context.Context, siteID, out string, rels, abses []string) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	home, _ := a.fs.siteHome(siteID)
	for i, abs := range abses {
		prefix := path.Dir(rels[i])
		if err := a.fs.walkArchive(ctx, home, abs, func(rel, full string, info os.FileInfo) error {
			entryName := path.Join(prefix, rel)
			if info.IsDir() {
				entryName += "/"
			}
			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			hdr.Name = entryName
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			return streamFile(tw, full)
		}); err != nil {
			return err
		}
	}
	return nil
}

// walkArchive, site içindeki bir kaynağı kök-göreli yollarla dolaşır.
func (s *FileService) walkArchive(ctx context.Context, home, absRoot string, fn func(rel, full string, info os.FileInfo) error) error {
	st, err := s.backend.Stat(ctx, absRoot)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fn(st.Name(), absRoot, st)
	}
	entries, err := s.backend.ListDir(ctx, absRoot)
	if err != nil {
		return err
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		full := path.Join(absRoot, e.Name())
		rel := strings.TrimPrefix(full, path.Clean(home)+"/")
		if info.IsDir() {
			// Dizin başlığı + özyineleme.
			if err := fn(rel, full, info); err != nil {
				return err
			}
			if err := s.walkArchive(ctx, home, full, fn); err != nil {
				return err
			}
			continue
		}
		if err := fn(rel, full, info); err != nil {
			return err
		}
	}
	return nil
}

// Extract, arşivi hedef dizine çıkarır (zip-slip + bomb korumalı).
func (a *ArchiveService) Extract(ctx context.Context, siteID, archiveRel, destRel, format string) error {
	if err := a.fs.allow("extract"); err != nil {
		return err
	}
	absArchive, err := a.fs.resolveAbs(ctx, siteID, archiveRel)
	if err != nil {
		return err
	}
	absDest, err := a.fs.resolveAbs(ctx, siteID, destRel)
	if err != nil {
		return err
	}
	if err := a.fs.backend.MkdirAll(ctx, absDest, 0o755); err != nil {
		return err
	}

	switch format {
	case "zip":
		return a.extractZip(ctx, absArchive, absDest)
	case "targz":
		return a.extractTarGz(ctx, absArchive, absDest)
	default:
		return fmt.Errorf("desteklenmeyen format: %q", format)
	}
}

// safeDest, zip-slip koruması: girdi yolu hedef içinde KALMAK ZORUNDA.
func (a *ArchiveService) safeDest(destDir, entryName string) (string, error) {
	if len(entryName) > a.limits.MaxEntryPath {
		return "", errors.New("girdi yolu çok uzun")
	}
	clean := path.Clean(entryName)
	if clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return "", fmt.Errorf("arşiv girdisi hedef dışına çıkıyor: %q", entryName)
	}
	return path.Join(destDir, clean), nil
}

func (a *ArchiveService) extractZip(ctx context.Context, absArchive, destDir string) error {
	zr, err := zip.OpenReader(absArchive)
	if err != nil {
		return err
	}
	defer zr.Close()

	var total int64
	if len(zr.File) > a.limits.MaxFiles {
		return fmt.Errorf("dosya sayısı sınırı aşıldı: %d", len(zr.File))
	}
	for _, zf := range zr.File {
		// Symlink girdileri reddedilir (FILE_MANAGER §12).
		if zf.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink girdisi reddedildi: %q", zf.Name)
		}
		dst, err := a.safeDest(destDir, zf.Name)
		if err != nil {
			return err
		}
		if zf.FileInfo().IsDir() {
			if err := a.fs.backend.MkdirAll(ctx, dst, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := a.fs.backend.MkdirAll(ctx, path.Dir(dst), 0o755); err != nil {
			return err
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		w, err := a.fs.backend.CreateFile(ctx, dst, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		n, err := io.Copy(w, rc)
		rc.Close()
		w.Close()
		if err != nil {
			return err
		}
		total += n
		if total > a.limits.MaxTotalSize {
			return fmt.Errorf("açılan toplam boyut sınırı aşıldı (%d)", a.limits.MaxTotalSize)
		}
		if a.limits.MaxRatio > 0 && total > int64(a.limits.MaxRatio)*compressedSize(zr.File) {
			return errors.New("sıkıştırma oranı sınırı aşıldı (archive bomb)")
		}
	}
	return nil
}

func (a *ArchiveService) extractTarGz(ctx context.Context, absArchive, destDir string) error {
	f, err := os.Open(absArchive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	var total int64
	files := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		files++
		if files > a.limits.MaxFiles {
			return fmt.Errorf("dosya sayısı sınırı aşıldı: %d", a.limits.MaxFiles)
		}
		if hdr.Typeflag == tar.TypeSymlink || hdr.Typeflag == tar.TypeLink {
			return fmt.Errorf("symlink/hardlink girdisi reddedildi: %q", hdr.Name)
		}
		dst, err := a.safeDest(destDir, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := a.fs.backend.MkdirAll(ctx, dst, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := a.fs.backend.MkdirAll(ctx, path.Dir(dst), 0o755); err != nil {
				return err
			}
			w, err := a.fs.backend.CreateFile(ctx, dst, 0o644)
			if err != nil {
				return err
			}
			n, err := io.Copy(w, io.LimitReader(tr, a.limits.MaxTotalSize-total+1))
			w.Close()
			if err != nil {
				return err
			}
			total += n
			if total > a.limits.MaxTotalSize {
				return fmt.Errorf("açılan toplam boyut sınırı aşıldı (%d)", a.limits.MaxTotalSize)
			}
			if a.limits.MaxRatio > 0 && total > int64(a.limits.MaxRatio)*archiveFileSize(absArchive) {
				return errors.New("sıkıştırma oranı sınırı aşıldı (archive bomb)")
			}
		default:
			return fmt.Errorf("desteklenmeyen girdi türü: %q", hdr.Name)
		}
	}
	return nil
}

func compressedSize(files []*zip.File) int64 {
	var total int64
	for _, f := range files {
		total += int64(f.CompressedSize64)
	}
	return total
}

func archiveFileSize(p string) int64 {
	st, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return st.Size()
}

func streamFile(w io.Writer, full string) error {
	f, err := os.Open(full)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}
