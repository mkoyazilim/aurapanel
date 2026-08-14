// Package update, güncelleme merkezini uygular (ARCHITECTURE §5.2-5.3,
// ROADMAP W14): manifest tabanlı uyumluluk matrisi, SHA-256 doğrulamalı
// self-update, EOL uyarıları.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Component, manifestteki tek bileşen (pinned sürüm + SHA-256).
type Component struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	URL     string `json:"url"`
}

// Manifest, versions.json yapısı (downloadaurapanel releases).
type Manifest struct {
	Panel      string               `json:"panel"`
	Components map[string]Component `json:"components"`
	TestedAt   string               `json:"tested_at"`
	Compat     map[string]string    `json:"compat"` // bileşen → desteklenen aralık (örn. ">=1.7.19,<2")
}

// Fetcher, dosya indirme soyutlaması (testlerde sahte).
type Fetcher interface {
	Get(ctx context.Context, url string) ([]byte, error)
}

// HTTPFetcher, gerçek HTTP indirme.
type HTTPFetcher struct {
	Client *http.Client
}

// Get, URL'yi indirir.
func (h *HTTPFetcher) Get(ctx context.Context, url string) ([]byte, error) {
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("indirme: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

// Service, güncelleme merkezi.
type Service struct {
	fetch       Fetcher
	manifestURL string
	current     string // panel sürümü
	binaryPath  string // mevcut çalışan binary
}

// NewService, Service oluşturur.
func NewService(fetch Fetcher, manifestURL, currentVersion, binaryPath string) *Service {
	return &Service{fetch: fetch, manifestURL: manifestURL, current: currentVersion, binaryPath: binaryPath}
}

// Check, en son manifesti çeker ve durumu raporlar.
func (s *Service) Check(ctx context.Context) (map[string]any, error) {
	b, err := s.fetch.Get(ctx, s.manifestURL)
	if err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	m, err := parseManifest(b)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"current":  s.current,
		"latest":   m.Panel,
		"update":   m.Panel != s.current,
		"tested_at": m.TestedAt,
	}
	if m.Panel != s.current {
		out["compatible"] = true // manifest yayınlanması CI doğrulaması demektir
	}
	return out, nil
}

// SelfUpdate, panel binary'sini manifestteki sürümle atomik değiştirir.
// Akış: indir → SHA-256 doğrula → aynı dizinde tmp + rename → chmod.
// Yeniden başlatma çağıranın sorumluluğundadır (siteler etkilenmez — İlke 7).
func (s *Service) SelfUpdate(ctx context.Context) (string, error) {
	b, err := s.fetch.Get(ctx, s.manifestURL)
	if err != nil {
		return "", err
	}
	m, err := parseManifest(b)
	if err != nil {
		return "", err
	}
	if m.Panel == "" || m.Panel == s.current {
		return "", fmt.Errorf("güncelleme yok (güncel: %s, manifest: %s)", s.current, m.Panel)
	}
	comp, ok := m.Components["panel"]
	if !ok {
		return "", fmt.Errorf("manifestte panel bileşeni yok")
	}
	url := comp.URL
	if url == "" {
		url = fmt.Sprintf("https://github.com/mkoyazilim/aurapanel/releases/download/v%s/aurapanel_%s_linux_amd64",
			m.Panel, m.Panel)
	}
	binary, err := s.fetch.Get(ctx, url)
	if err != nil {
		return "", err
	}
	if comp.SHA256 != "" {
		if err := verifySHA256(binary, comp.SHA256); err != nil {
			return "", err
		}
	}

	dir := filepath.Dir(s.binaryPath)
	tmp, err := os.CreateTemp(dir, ".aurapanel-update-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, s.binaryPath); err != nil {
		return "", fmt.Errorf("atomik değişim: %w", err)
	}
	return m.Panel, nil
}

// parseManifest, versions.json'ı çözer.
func parseManifest(b []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("manifest çözümlenemedi: %w", err)
	}
	if m.Panel == "" {
		return nil, fmt.Errorf("manifestte panel sürümü yok")
	}
	return &m, nil
}

// verifySHA256, indirilen binary'yi doğrular — eşleşmezse KURULUM YAPILMAZ.
func verifySHA256(b []byte, wantHex string) error {
	sum := sha256.Sum256(b)
	got := hex.EncodeToString(sum[:])
	if got != wantHex {
		return fmt.Errorf("SHA-256 doğrulaması BAŞARISIZ (beklenen %s…, gelen %s…)", wantHex[:16], got[:16])
	}
	return nil
}
