package fm

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/privclient"
)

// PrivBackend, Backend'in Tier-1 üretim uygulaması (FILE_MANAGER §14):
// tüm dosya işlemleri priv helper'ın file.op'u üzerinden, SİTE UID'siyle
// ve site cgroup'u içinde yürür — panel süreci asla dosyalara dokunmaz.
type PrivBackend struct {
	client    *privclient.Client
	sitesRoot string
}

// NewPrivBackend, PrivBackend oluşturur.
func NewPrivBackend(c *privclient.Client, sitesRoot string) *PrivBackend {
	return &PrivBackend{client: c, sitesRoot: sitesRoot}
}

// split, mutlak yolu (siteID, kök-göreli yol) olarak ayırır.
func (p *PrivBackend) split(abs string) (string, string, error) {
	root := path.Clean(p.sitesRoot)
	if abs != root && !strings.HasPrefix(abs, root+"/") {
		return "", "", fmt.Errorf("yol sitesRoot dışında: %s", abs)
	}
	rest := strings.TrimPrefix(abs, root+"/")
	parts := strings.Split(rest, "/")
	if len(parts) < 1 || parts[0] == "" {
		return "", "", fmt.Errorf("yol biçimi bozuk: %s", abs)
	}
	siteID := parts[0]
	var rel string
	if len(parts) > 1 && parts[1] == "home" {
		if len(parts) == 2 {
			rel = "."
		} else {
			rel = strings.Join(parts[2:], "/")
		}
	} else if len(parts) > 1 {
		rel = strings.Join(parts[1:], "/")
	} else {
		rel = "."
	}
	return siteID, rel, nil
}

func (p *PrivBackend) call(ctx context.Context, verb string, siteID string, paths []string) (map[string]any, error) {
	return p.client.Call(ctx, "file.op", map[string]any{"site": siteID, "verb": verb, "paths": paths})
}

func (p *PrivBackend) ListDir(ctx context.Context, absDir string) ([]os.DirEntry, error) {
	siteID, rel, err := p.split(absDir)
	if err != nil {
		return nil, err
	}
	data, err := p.call(ctx, "list", siteID, []string{rel})
	if err != nil {
		return nil, err
	}
	raw, _ := data["entries"].([]any)
	out := make([]os.DirEntry, 0, len(raw))
	for _, e := range raw {
		m, _ := e.(map[string]any)
		out = append(out, workerEntry{
			name: strOr(m, "name"), isDir: boolOr(m, "is_dir"),
			size: intOr(m, "size"), mode: os.FileMode(intOr(m, "mode")),
			mtime: strOr(m, "mtime"),
		})
	}
	return out, nil
}

// workerEntry, worker JSON'undan os.DirEntry.
type workerEntry struct {
	name  string
	isDir bool
	size  int64
	mode  os.FileMode
	mtime string
}

func (w workerEntry) Name() string      { return w.name }
func (w workerEntry) IsDir() bool       { return w.isDir }
func (w workerEntry) Type() os.FileMode { return 0 }
func (w workerEntry) Info() (os.FileInfo, error) {
	return workerInfo{name: w.name, size: w.size, isDir: w.isDir, mode: w.mode, mtime: w.mtime}, nil
}

func (p *PrivBackend) ReadFile(ctx context.Context, absPath string, limit int64) ([]byte, error) {
	siteID, rel, err := p.split(absPath)
	if err != nil {
		return nil, err
	}
	data, err := p.call(ctx, "read", siteID, []string{rel})
	if err != nil {
		return nil, err
	}
	b64, _ := data["content_b64"].(string)
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	if limit > 0 && int64(len(b)) > limit {
		return b[:limit], nil
	}
	return b, nil
}

func (p *PrivBackend) WriteFileAtomic(ctx context.Context, absPath string, data []byte, mode os.FileMode) error {
	if len(data) > 16<<20 {
		return fmt.Errorf("Tier-1 yazma sınırı aşıldı (16 MiB); akış yolu sunucu fazında")
	}
	siteID, rel, err := p.split(absPath)
	if err != nil {
		return err
	}
	_, err = p.client.Call(ctx, "file.op", map[string]any{
		"site": siteID, "verb": "write", "paths": []string{rel},
		"content_b64": base64.StdEncoding.EncodeToString(data),
	})
	return err
}

func (p *PrivBackend) MkdirAll(ctx context.Context, absPath string, mode os.FileMode) error {
	siteID, rel, err := p.split(absPath)
	if err != nil {
		return err
	}
	_, err = p.call(ctx, "mkdir", siteID, []string{rel})
	return err
}

func (p *PrivBackend) Rename(ctx context.Context, from, to string) error {
	siteID, relFrom, err := p.split(from)
	if err != nil {
		return err
	}
	_, relTo, err := p.split(to)
	if err != nil {
		return err
	}
	_, err = p.call(ctx, "rename", siteID, []string{relFrom, relTo})
	return err
}

func (p *PrivBackend) RemoveAll(ctx context.Context, absPath string) error {
	siteID, rel, err := p.split(absPath)
	if err != nil {
		return err
	}
	_, err = p.call(ctx, "remove", siteID, []string{rel})
	return err
}

func (p *PrivBackend) Symlink(ctx context.Context, target, link string) error {
	siteID, relTarget, err := p.split(target)
	if err != nil {
		return err
	}
	_, relLink, err := p.split(link)
	if err != nil {
		return err
	}
	_, err = p.call(ctx, "symlink", siteID, []string{relTarget, relLink})
	return err
}

func (p *PrivBackend) Stat(ctx context.Context, absPath string) (os.FileInfo, error) {
	siteID, rel, err := p.split(absPath)
	if err != nil {
		return nil, err
	}
	data, err := p.call(ctx, "stat", siteID, []string{rel})
	if err != nil {
		return nil, err
	}
	return workerInfo{
		name: strOr(data, "name"), size: intOr(data, "size"),
		isDir: boolOr(data, "is_dir"), mode: os.FileMode(intOr(data, "mode")),
		mtime: strOr(data, "mtime"),
	}, nil
}

func (p *PrivBackend) EvalSymlinks(ctx context.Context, absPath string) (string, error) {
	siteID, rel, err := p.split(absPath)
	if err != nil {
		return "", err
	}
	data, err := p.call(ctx, "eval", siteID, []string{rel})
	if err != nil {
		return "", err
	}
	out, _ := data["path"].(string)
	return out, nil
}

// OpenFile/CreateFile: JSON protokolü akış desteklemez — büyük yükleme
// akışı sunucu fazında (Tier-1 stream) eklenecek; şimdilik açık hata.
func (p *PrivBackend) OpenFile(ctx context.Context, absPath string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("Tier-1 akış okuma sunucu fazında")
}

func (p *PrivBackend) CreateFile(ctx context.Context, absPath string, mode os.FileMode) (io.WriteCloser, error) {
	return nil, fmt.Errorf("Tier-1 akış yazma sunucu fazında")
}

// --- worker yanıt yardımcıları ---

func strOr(m map[string]any, k string) string { s, _ := m[k].(string); return s }
func intOr(m map[string]any, k string) int64 {
	switch v := m[k].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	}
	return 0
}
func boolOr(m map[string]any, k string) bool { b, _ := m[k].(bool); return b }

type workerInfo struct {
	name  string
	size  int64
	isDir bool
	mode  os.FileMode
	mtime string
}

func (w workerInfo) Name() string      { return w.name }
func (w workerInfo) Size() int64       { return w.size }
func (w workerInfo) Mode() os.FileMode { return w.mode }
func (w workerInfo) ModTime() time.Time {
	t, _ := time.Parse(time.RFC3339Nano, w.mtime)
	return t
}
func (w workerInfo) IsDir() bool { return w.isDir }
func (w workerInfo) Sys() any    { return nil }
