// Package backup — S3 ve Cloudflare R2 uyumlu uzak depolama motoru.
// AWS Signature Version 4 (SigV4) protokolünü saf Go (pure-Go) ile uygular;
// harici ağır AWS SDK bağımlılığı içermez.
package backup

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
)

// S3Config, S3 / Cloudflare R2 bağlantı yapılandırması.
type S3Config struct {
	Endpoint  string `json:"endpoint"`   // örn: "https://<account_id>.r2.cloudflarestorage.com" veya "https://s3.amazonaws.com"
	Bucket    string `json:"bucket"`     // bucket adı
	Region    string `json:"region"`     // örn: "auto" (R2 için) veya "us-east-1"
	AccessKey string `json:"access_key"` // Access Key ID
	SecretKey string `json:"secret_key"` // Secret Access Key
}

// S3Storage, backup.Storage arayüzünü S3 REST API üzerinden uygular.
type S3Storage struct {
	cfg        S3Config
	httpClient *http.Client
}

// NewS3Storage, S3Storage oluşturur.
func NewS3Storage(cfg S3Config) *S3Storage {
	if cfg.Region == "" {
		cfg.Region = "auto"
	}
	// Endpoint başında protokol yoksa https:// ekle
	if !strings.HasPrefix(cfg.Endpoint, "http://") && !strings.HasPrefix(cfg.Endpoint, "https://") {
		cfg.Endpoint = "https://" + cfg.Endpoint
	}
	cfg.Endpoint = strings.TrimRight(cfg.Endpoint, "/")

	return &S3Storage{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Minute},
	}
}

func (s *S3Storage) objectURL(objectKey string) string {
	cleanKey := strings.TrimLeft(objectKey, "/")
	return fmt.Sprintf("%s/%s/%s", s.cfg.Endpoint, s.cfg.Bucket, cleanKey)
}

// Save, r'den okunan veriyi S3/R2'ye PUT eder.
func (s *S3Storage) Save(ctx context.Context, name string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("veri okunamadı: %w", err)
	}

	u := s.objectURL(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	s.signRequest(req, data)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("s3 put isteği: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("s3 put başarısız (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// Open, S3/R2'den nesneyi indirir (GET).
func (s *S3Storage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	u := s.objectURL(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	s.signRequest(req, nil)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("s3 get isteği: %w", err)
	}

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("s3 get başarısız (%d): %s", resp.StatusCode, string(body))
	}
	return resp.Body, nil
}

// Delete, S3/R2'deki nesneyi siler (DELETE).
func (s *S3Storage) Delete(ctx context.Context, name string) error {
	u := s.objectURL(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}

	s.signRequest(req, nil)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("s3 delete isteği: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("s3 delete başarısız (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// List, bucket içindeki nesneleri listeler (ListObjectsV2).
func (s *S3Storage) List(ctx context.Context) ([]string, error) {
	u := fmt.Sprintf("%s/%s?list-type=2", s.cfg.Endpoint, s.cfg.Bucket)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	s.signRequest(req, nil)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("s3 list isteği: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("s3 list başarısız (%d): %s", resp.StatusCode, string(body))
	}

	type Contents struct {
		Key string `xml:"Key"`
	}
	type ListBucketResult struct {
		Contents []Contents `xml:"Contents"`
	}

	var res ListBucketResult
	if err := xml.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("s3 list xml parse: %w", err)
	}

	var keys []string
	for _, c := range res.Contents {
		keys = append(keys, c.Key)
	}
	return keys, nil
}

// TestConnection, S3/R2 erişim ve yazma yetkisini test eder.
func (s *S3Storage) TestConnection(ctx context.Context) error {
	testFile := "aurapanel-test-" + time.Now().Format("20060102150405") + ".txt"
	testContent := []byte("aurapanel s3 connection test ok")

	// 1. Yazma testi
	if err := s.Save(ctx, testFile, bytes.NewReader(testContent)); err != nil {
		return fmt.Errorf("yazma testi başarısız: %w", err)
	}
	// 2. Silme testi
	_ = s.Delete(ctx, testFile)
	return nil
}

// --- AWS Signature Version 4 (SigV4) ---

func (s *S3Storage) signRequest(req *http.Request, payload []byte) {
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("Host", req.URL.Host)

	// Payload SHA256
	payloadHash := hex.EncodeToString(hashSHA256(payload))
	req.Header.Set("x-amz-content-sha256", payloadHash)

	// Canonical headers
	headers := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	if ct := req.Header.Get("Content-Type"); ct != "" {
		headers = append(headers, "content-type")
	}
	sort.Strings(headers)

	var canonicalHeaders strings.Builder
	for _, h := range headers {
		canonicalHeaders.WriteString(fmt.Sprintf("%s:%s\n", h, strings.TrimSpace(req.Header.Get(h))))
	}
	signedHeaders := strings.Join(headers, ";")

	// Canonical URI & Query
	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := s.buildCanonicalQuery(req.URL.Query())

	canonicalReq := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders.String(),
		signedHeaders,
		payloadHash,
	)

	// String to sign
	service := "s3"
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, s.cfg.Region, service)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		hex.EncodeToString(hashSHA256([]byte(canonicalReq))),
	)

	// Signature
	signingKey := getSignatureKey(s.cfg.SecretKey, dateStamp, s.cfg.Region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	// Authorization Header
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.cfg.AccessKey,
		credentialScope,
		signedHeaders,
		signature,
	)
	req.Header.Set("Authorization", authHeader)
}

func (s *S3Storage) buildCanonicalQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	var keys []string
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(url.QueryEscape(k))
		v := values.Get(k)
		if v != "" {
			buf.WriteByte('=')
			buf.WriteString(url.QueryEscape(v))
		}
	}
	return buf.String()
}

func hashSHA256(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func getSignatureKey(secret, dateStamp, regionName, serviceName string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(regionName))
	kService := hmacSHA256(kRegion, []byte(serviceName))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

// Ensure S3Storage implements Storage
var _ Storage = (*S3Storage)(nil)
var _ = path.Base
