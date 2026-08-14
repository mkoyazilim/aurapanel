// Package extdns, Cloudflare ve Route53 gibi harici DNS sağlayıcılarıyla
// çift-yönlü senkronizasyon sağlar. Credential'lar AES-GCM ile şifreli saklanır.
package extdns

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/crypto"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// DNSRecord, sağlayıcıdan bağımsız DNS kaydı.
type DNSRecord struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied,omitempty"` // Cloudflare-only
}

// SyncResult, senkronizasyon sonucu.
type SyncResult struct {
	Added     []string `json:"added"`
	Updated   []string `json:"updated"`
	Conflicts []string `json:"conflicts"`
}

// Service, External DNS senkronizasyon servisi.
type Service struct {
	store  *store.Store
	cipher *crypto.Cipher
	http   *http.Client
}

func New(st *store.Store, cipher *crypto.Cipher) *Service {
	return &Service{
		store:  st,
		cipher: cipher,
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

// ─── Credential şifreleme ─────────────────────────────────────────────────────

// EncryptCreds, credential map'ini JSON → AES-GCM şifreli string'e dönüştürür.
func (s *Service) EncryptCreds(creds map[string]string) (string, error) {
	raw, err := json.Marshal(creds)
	if err != nil {
		return "", err
	}
	return s.cipher.Encrypt(raw)
}

// DecryptCreds, şifreli string'i credential map'e çözer.
func (s *Service) DecryptCreds(enc string) (map[string]string, error) {
	plain, err := s.cipher.Decrypt(enc)
	if err != nil {
		return nil, err
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(plain), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ─── Cloudflare ──────────────────────────────────────────────────────────────

// cfRequest, Cloudflare API'ye istek gönderir.
func (s *Service) cfRequest(ctx context.Context, apiToken, method, url string, body any, out any) error {
	var bodyReader *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewBuffer(b)
	} else {
		bodyReader = &bytes.Buffer{}
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("cloudflare API error: %s", resp.Status)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// CloudflareListRecords, zone'daki tüm DNS kayıtlarını getirir.
func (s *Service) CloudflareListRecords(ctx context.Context, apiToken, zoneID string) ([]DNSRecord, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?per_page=1000", zoneID)
	var result struct {
		Result []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Type    string `json:"type"`
			Content string `json:"content"`
			TTL     int    `json:"ttl"`
			Proxied bool   `json:"proxied"`
		} `json:"result"`
	}
	if err := s.cfRequest(ctx, apiToken, http.MethodGet, url, nil, &result); err != nil {
		return nil, err
	}
	out := make([]DNSRecord, len(result.Result))
	for i, r := range result.Result {
		out[i] = DNSRecord{ID: r.ID, Name: r.Name, Type: r.Type, Content: r.Content, TTL: r.TTL, Proxied: r.Proxied}
	}
	return out, nil
}

// CloudflareCreateRecord, Cloudflare'da yeni DNS kaydı oluşturur.
func (s *Service) CloudflareCreateRecord(ctx context.Context, apiToken, zoneID string, rec DNSRecord) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)
	body := map[string]any{
		"type":    rec.Type,
		"name":    rec.Name,
		"content": rec.Content,
		"ttl":     rec.TTL,
		"proxied": rec.Proxied,
	}
	return s.cfRequest(ctx, apiToken, http.MethodPost, url, body, nil)
}

// CloudflareDeleteRecord, Cloudflare'da DNS kaydını siler.
func (s *Service) CloudflareDeleteRecord(ctx context.Context, apiToken, zoneID, recordID string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, recordID)
	return s.cfRequest(ctx, apiToken, http.MethodDelete, url, nil, nil)
}

// CloudflarePurgeCacheURLs, belirli URL'leri Cloudflare cache'den temizler.
func (s *Service) CloudflarePurgeCacheURLs(ctx context.Context, apiToken, zoneID string, urls []string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/purge_cache", zoneID)
	return s.cfRequest(ctx, apiToken, http.MethodPost, url, map[string]any{"files": urls}, nil)
}

// CloudflareSyncPush, PowerDNS'teki kayıtları Cloudflare'a iter (push).
// Çakışma: Cloudflare'da aynı name+type varsa "conflict" olarak raporlar, üzerine yazmaz.
func (s *Service) CloudflareSyncPush(ctx context.Context, apiToken, zoneID string, localRecords []DNSRecord) (*SyncResult, error) {
	remote, err := s.CloudflareListRecords(ctx, apiToken, zoneID)
	if err != nil {
		return nil, err
	}
	// remote index: name+type → record
	remoteIndex := make(map[string]DNSRecord, len(remote))
	for _, r := range remote {
		key := r.Name + "|" + r.Type
		remoteIndex[key] = r
	}

	result := &SyncResult{}
	for _, local := range localRecords {
		key := local.Name + "|" + local.Type
		if existing, exists := remoteIndex[key]; exists {
			if existing.Content != local.Content {
				result.Conflicts = append(result.Conflicts, key)
			}
			// Çakışma varsa dokunma
			continue
		}
		if err := s.CloudflareCreateRecord(ctx, apiToken, zoneID, local); err != nil {
			return result, fmt.Errorf("push %s: %w", key, err)
		}
		result.Added = append(result.Added, key)
	}
	return result, nil
}

// ─── Route53 ─────────────────────────────────────────────────────────────────
// Route53 AWS SDK olmadan vanilla HTTP + AWS SigV4 imzası gerektirir.
// MVP: credential doğrulama + kayıt listeleme stub (gerçek SigV4 entegrasyonu
// AWS SDK v2 eklendiğinde tamamlanır; şimdi credential şifreli saklanır).

// Route53ValidateCreds, credential'ın geçerli olup olmadığını kontrol eder.
// Gerçek AWS çağrısı yapılmaz; format kontrolü yapılır.
func (s *Service) Route53ValidateCreds(accessKey, secretKey, region string) error {
	if len(accessKey) < 16 || len(secretKey) < 16 {
		return fmt.Errorf("invalid AWS credentials format")
	}
	if region == "" {
		return fmt.Errorf("region required")
	}
	return nil
}

// Route53ListZones, kayıtlı Route53 zone listesini döndürür (stub — AWS SDK gerektirir).
func (s *Service) Route53ListZones(ctx context.Context, accessKey, secretKey, region string) ([]DNSRecord, error) {
	// AWS SigV4 gerektiren full implementasyon aws-sdk-go-v2 ile gelecek.
	// Şimdilik credential doğrulaması yapılır, boş liste döner.
	if err := s.Route53ValidateCreds(accessKey, secretKey, region); err != nil {
		return nil, err
	}
	return []DNSRecord{}, nil
}

// ─── Senkron log yardımcıları ─────────────────────────────────────────────────

func (s *Service) LogSync(ctx context.Context, providerID int64, direction, action, detail string) {
	s.store.InsertExtDNSSyncLog(ctx, store.ExtDNSSyncLog{
		ProviderID: providerID,
		Direction:  direction,
		Action:     action,
		Detail:     detail,
	})
}
