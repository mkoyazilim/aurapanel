package fm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/audit"
)

// reSiteID, site kimliği doğrulaması (diğer paketlerle birebir).
var reSiteID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// ErrConflict, optimistic locking çakışması (FILE_MANAGER §13 → HTTP 409).
var ErrConflict = errors.New("dosya diskte değişmiş. Kaydetmeden önce yeniden yükleyin.")

// ErrRateLimited, hız sınırı reddi (FILE_MANAGER §14 → HTTP 429).
var ErrRateLimited = errors.New("işlem sınırı aşıldı (429)")

// RateLimiter, işlem bazlı hız sınırı kancası. nil = sınırsız.
type RateLimiter interface {
	Allow(action string) bool
}

// Entry, tek dizin girişi (kök-göreli POSIX yol).
type Entry struct {
	Name    string
	Path    string
	Size    int64
	Mode    os.FileMode
	ModTime time.Time
	IsDir   bool
}

// FileService, Filesystem Access Layer (FILE_MANAGER §27): tüm dosya
// işlemleri bu servisten geçer. İşlem başına akış:
//
//	resolve (canonical + symlink policy) → rate limit → backend → audit
type FileService struct {
	backend   Backend
	audit     *audit.Service
	limiter   RateLimiter
	sitesRoot string
	maxList   int
	readLimit int64
}

// New, FileService oluşturur.
func New(backend Backend, au *audit.Service, sitesRoot string) *FileService {
	return &FileService{
		backend:   backend,
		audit:     au,
		sitesRoot: sitesRoot,
		maxList:   1000,      // pagination W9.3'te; üst sınır koruma
		readLimit: 4 << 20,   // editör okuma sınırı (4 MiB)
	}
}

// SetRateLimiter, hız sınırı kancasını bağlar.
func (s *FileService) SetRateLimiter(l RateLimiter) { s.limiter = l }

// siteHome, sitenin document root'unu türetir (kullanıcıdan ASLA alınmaz).
func (s *FileService) siteHome(siteID string) (string, error) {
	if !reSiteID.MatchString(siteID) {
		return "", fmt.Errorf("site kimliği geçersiz: %q", siteID)
	}
	return path.Join(s.sitesRoot, siteID, "home"), nil
}

// canon, backend üzerinden kanonik çözümleme.
func (s *FileService) canon(ctx context.Context) canonicalizer {
	return func(p string) (string, error) { return s.backend.EvalSymlinks(ctx, p) }
}

// resolveAbs, site kökünde doğrulanmış mutlak yolu döndürür.
func (s *FileService) resolveAbs(ctx context.Context, siteID, relPath string) (string, error) {
	home, err := s.siteHome(siteID)
	if err != nil {
		return "", err
	}
	return resolve(s.canon(ctx), home, relPath)
}

// allow, hız sınırı denetimi.
func (s *FileService) allow(action string) error {
	if s.limiter != nil && !s.limiter.Allow(action) {
		return ErrRateLimited
	}
	return nil
}

// auditEvent, içerik ASLA yazılmaz (FILE_MANAGER §18).
func (s *FileService) auditEvent(ctx context.Context, siteID, action, relPath, result string, extra map[string]any) {
	if s.audit == nil {
		return
	}
	if extra == nil {
		extra = map[string]any{}
	}
	extra["path"] = relPath
	s.audit.Write(ctx, audit.Event{
		Action: "file." + action, Target: siteID, Result: result, Extra: extra,
	})
}

// --- Okuma ---

// List, dizin içeriğini döndürür (sıralı, maxList sınırlı).
func (s *FileService) List(ctx context.Context, siteID, relDir string) ([]Entry, error) {
	if err := s.allow("list"); err != nil {
		return nil, err
	}
	abs, err := s.resolveAbs(ctx, siteID, relDir)
	if err != nil {
		return nil, err
	}
	entries, err := s.backend.ListDir(ctx, abs)
	if err != nil {
		return nil, err
	}
	if len(entries) > s.maxList {
		entries = entries[:s.maxList]
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue // silinen girdiyi atla
		}
		out = append(out, Entry{
			Name:    e.Name(),
			Path:    path.Join(relDir, e.Name()),
			Size:    info.Size(),
			Mode:    info.Mode(),
			ModTime: info.ModTime().UTC(),
			IsDir:   info.IsDir(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// Read, dosya içeriğini + doğrulama bilgilerini döndürür (optimistic lock).
func (s *FileService) Read(ctx context.Context, siteID, relPath string) (content []byte, hash, mtime string, err error) {
	if err := s.allow("read"); err != nil {
		return nil, "", "", err
	}
	abs, err := s.resolveAbs(ctx, siteID, relPath)
	if err != nil {
		return nil, "", "", err
	}
	content, err = s.backend.ReadFile(ctx, abs, s.readLimit)
	if err != nil {
		return nil, "", "", err
	}
	st, err := s.backend.Stat(ctx, abs)
	if err != nil {
		return nil, "", "", err
	}
	return content, contentHash(content), st.ModTime().UTC().Format(time.RFC3339Nano), nil
}

// Stat, tek girişin bilgisini döndürür.
func (s *FileService) Stat(ctx context.Context, siteID, relPath string) (Entry, error) {
	abs, err := s.resolveAbs(ctx, siteID, relPath)
	if err != nil {
		return Entry{}, err
	}
	st, err := s.backend.Stat(ctx, abs)
	if err != nil {
		return Entry{}, err
	}
	return Entry{
		Name:    st.Name(),
		Path:    relPath,
		Size:    st.Size(),
		Mode:    st.Mode(),
		ModTime: st.ModTime().UTC(),
		IsDir:   st.IsDir(),
	}, nil
}

// --- Yazma ---

// Write, içeriği ATOMİK yazar. expectedHash/expectedMtime verildiyse
// optimistic locking uygulanır: eşleşmezse ErrConflict (FILE_MANAGER §13).
func (s *FileService) Write(ctx context.Context, siteID, relPath string, content []byte, expectedHash, expectedMtime string) error {
	if err := s.allow("write"); err != nil {
		return err
	}
	abs, err := s.resolveAbs(ctx, siteID, relPath)
	if err != nil {
		return err
	}
	if expectedHash != "" || expectedMtime != "" {
		current, err := s.backend.ReadFile(ctx, abs, s.readLimit)
		if err != nil {
			return err
		}
		st, err := s.backend.Stat(ctx, abs)
		if err != nil {
			return err
		}
		gotHash := contentHash(current)
		gotMtime := st.ModTime().UTC().Format(time.RFC3339Nano)
		if (expectedHash != "" && expectedHash != gotHash) || (expectedMtime != "" && expectedMtime != gotMtime) {
			s.auditEvent(ctx, siteID, "write", relPath, "conflict", map[string]any{"reason": "stale"})
			return ErrConflict
		}
	}
	if err := s.backend.WriteFileAtomic(ctx, abs, content, 0o644); err != nil {
		return err
	}
	s.auditEvent(ctx, siteID, "write", relPath, "success", map[string]any{"size": len(content)})
	return nil
}

// Mkdir, dizin oluşturur.
func (s *FileService) Mkdir(ctx context.Context, siteID, relPath string) error {
	if err := s.allow("write"); err != nil {
		return err
	}
	abs, err := s.resolveAbs(ctx, siteID, relPath)
	if err != nil {
		return err
	}
	if err := s.backend.MkdirAll(ctx, abs, 0o755); err != nil {
		return err
	}
	s.auditEvent(ctx, siteID, "mkdir", relPath, "success", nil)
	return nil
}

// Remove, özyinelemeli siler. NOT: W9.2'de trash akışına bağlanacak
// (FILE_MANAGER §6 — kalıcı silme varsayılan DEĞİL).
func (s *FileService) Remove(ctx context.Context, siteID, relPath string) error {
	if err := s.allow("delete"); err != nil {
		return err
	}
	abs, err := s.resolveAbs(ctx, siteID, relPath)
	if err != nil {
		return err
	}
	// Site root'un KENDİSİ asla silinemez.
	home, _ := s.siteHome(siteID)
	if abs == path.Clean(home) {
		return ErrInvalidPath
	}
	if err := s.backend.RemoveAll(ctx, abs); err != nil {
		return err
	}
	s.auditEvent(ctx, siteID, "delete", relPath, "success", nil)
	return nil
}

// Rename/Move, root içinde taşır.
func (s *FileService) Rename(ctx context.Context, siteID, from, to string) error {
	if err := s.allow("write"); err != nil {
		return err
	}
	absFrom, err := s.resolveAbs(ctx, siteID, from)
	if err != nil {
		return err
	}
	absTo, err := s.resolveAbs(ctx, siteID, to)
	if err != nil {
		return err
	}
	if err := s.backend.Rename(ctx, absFrom, absTo); err != nil {
		return err
	}
	s.auditEvent(ctx, siteID, "rename", from, "success", map[string]any{"to": to})
	return nil
}

// Copy, dosya veya dizini özyinelemeli kopyalar (stream ile).
func (s *FileService) Copy(ctx context.Context, siteID, from, to string) error {
	if err := s.allow("copy"); err != nil {
		return err
	}
	absFrom, err := s.resolveAbs(ctx, siteID, from)
	if err != nil {
		return err
	}
	absTo, err := s.resolveAbs(ctx, siteID, to)
	if err != nil {
		return err
	}
	if err := s.copyRecursive(ctx, absFrom, absTo); err != nil {
		return err
	}
	s.auditEvent(ctx, siteID, "copy", from, "success", map[string]any{"to": to})
	return nil
}

func (s *FileService) copyRecursive(ctx context.Context, from, to string) error {
	st, err := s.backend.Stat(ctx, from)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		src, err := s.backend.OpenFile(ctx, from)
		if err != nil {
			return err
		}
		defer src.Close()
		dst, err := s.backend.CreateFile(ctx, to, st.Mode())
		if err != nil {
			return err
		}
		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			return err
		}
		return dst.Close()
	}
	if err := s.backend.MkdirAll(ctx, to, 0o755); err != nil {
		return err
	}
	entries, err := s.backend.ListDir(ctx, from)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := s.copyRecursive(ctx, path.Join(from, e.Name()), path.Join(to, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// Symlink, site root İÇİNDE kalan hedefe symlink oluşturur (FILE_MANAGER §2.3).
func (s *FileService) Symlink(ctx context.Context, siteID, targetRel, linkRel string) error {
	if err := s.allow("write"); err != nil {
		return err
	}
	// Hedef ve link İKİSİ DE aynı pipeline'dan geçer.
	absTarget, err := s.resolveAbs(ctx, siteID, targetRel)
	if err != nil {
		return err
	}
	absLink, err := s.resolveAbs(ctx, siteID, linkRel)
	if err != nil {
		return err
	}
	if err := s.backend.Symlink(ctx, absTarget, absLink); err != nil {
		return err
	}
	s.auditEvent(ctx, siteID, "symlink", linkRel, "success", map[string]any{"target": targetRel})
	return nil
}

// contentHash, optimistic locking için SHA-256.
func contentHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
