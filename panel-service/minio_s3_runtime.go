package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awscreds "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type minioS3ConfigPayload struct {
	Provider     string `json:"provider"`
	Region       string `json:"region"`
	Bucket       string `json:"bucket"`
	Endpoint     string `json:"endpoint"`
	AccessKey    string `json:"access_key"`
	SecretKey    string `json:"secret_key"`
	UsePathStyle bool   `json:"use_path_style"`
}

type minioS3ConfigView struct {
	Provider       string `json:"provider"`
	Region         string `json:"region"`
	Bucket         string `json:"bucket"`
	Endpoint       string `json:"endpoint"`
	AccessKey      string `json:"access_key"`
	MaskedSecret   string `json:"masked_secret,omitempty"`
	HasSecret      bool   `json:"has_secret"`
	UsePathStyle   bool   `json:"use_path_style"`
	SecretKeptHint bool   `json:"secret_kept_hint"`
}

func normalizeMinIOS3Provider(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return "aws"
	}
	return normalized
}

func normalizeS3Bucket(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeS3Endpoint(raw string) string {
	endpoint := strings.TrimSpace(raw)
	endpoint = strings.TrimSuffix(endpoint, "/")
	if endpoint == "" {
		return ""
	}
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	return "https://" + endpoint
}

func maskSecret(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 4 {
		return strings.Repeat("*", len(trimmed))
	}
	return strings.Repeat("*", len(trimmed)-4) + trimmed[len(trimmed)-4:]
}

func minioS3View(cfg MinIOS3Config, secretKeptHint bool) minioS3ConfigView {
	return minioS3ConfigView{
		Provider:       normalizeMinIOS3Provider(cfg.Provider),
		Region:         strings.TrimSpace(cfg.Region),
		Bucket:         strings.TrimSpace(cfg.Bucket),
		Endpoint:       strings.TrimSpace(cfg.Endpoint),
		AccessKey:      strings.TrimSpace(cfg.AccessKey),
		MaskedSecret:   maskSecret(cfg.SecretKey),
		HasSecret:      strings.TrimSpace(cfg.SecretKey) != "",
		UsePathStyle:   cfg.UsePathStyle,
		SecretKeptHint: secretKeptHint,
	}
}

func mergeMinIOS3Config(payload minioS3ConfigPayload, previous MinIOS3Config) (MinIOS3Config, bool, error) {
	cfg := MinIOS3Config{
		Provider:     normalizeMinIOS3Provider(payload.Provider),
		Region:       strings.TrimSpace(payload.Region),
		Bucket:       normalizeS3Bucket(payload.Bucket),
		Endpoint:     normalizeS3Endpoint(payload.Endpoint),
		AccessKey:    strings.TrimSpace(payload.AccessKey),
		SecretKey:    strings.TrimSpace(payload.SecretKey),
		UsePathStyle: payload.UsePathStyle,
	}
	if cfg.Provider != "aws" {
		return MinIOS3Config{}, false, fmt.Errorf("only aws provider is supported right now")
	}

	secretKeptHint := false
	if cfg.SecretKey == "" && strings.TrimSpace(previous.SecretKey) != "" {
		cfg.SecretKey = strings.TrimSpace(previous.SecretKey)
		secretKeptHint = true
	}
	if cfg.Region == "" {
		return MinIOS3Config{}, false, fmt.Errorf("region is required")
	}
	if cfg.Bucket == "" {
		return MinIOS3Config{}, false, fmt.Errorf("bucket is required")
	}
	if cfg.AccessKey == "" {
		return MinIOS3Config{}, false, fmt.Errorf("access key is required")
	}
	if cfg.SecretKey == "" {
		return MinIOS3Config{}, false, fmt.Errorf("secret key is required")
	}

	return cfg, secretKeptHint, nil
}

func testMinIOS3Connection(cfg MinIOS3Config) error {
	awsConfig := aws.NewConfig().
		WithRegion(strings.TrimSpace(cfg.Region)).
		WithCredentials(awscreds.NewStaticCredentials(strings.TrimSpace(cfg.AccessKey), strings.TrimSpace(cfg.SecretKey), "")).
		WithS3ForcePathStyle(cfg.UsePathStyle)

	if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
		awsConfig = awsConfig.WithEndpoint(endpoint)
		if strings.HasPrefix(endpoint, "http://") {
			awsConfig = awsConfig.WithDisableSSL(true)
		}
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return fmt.Errorf("aws config load failed: %w", err)
	}
	client := s3.New(sess)
	_, err = client.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(strings.TrimSpace(cfg.Bucket)),
	})
	if err == nil {
		return nil
	}

	if awsErr, ok := err.(awserr.Error); ok {
		return fmt.Errorf("%s: %s", awsErr.Code(), awsErr.Message())
	}
	return err
}

func (s *service) handleMinIOS3ConfigGet(w http.ResponseWriter) {
	s.mu.RLock()
	cfg := s.modules.MinIOS3Config
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: minioS3View(cfg, false)})
}

func (s *service) handleMinIOS3ConfigSet(w http.ResponseWriter, r *http.Request) {
	var payload minioS3ConfigPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid S3 config payload.")
		return
	}

	s.mu.RLock()
	previous := s.modules.MinIOS3Config
	s.mu.RUnlock()

	cfg, secretKeptHint, err := mergeMinIOS3Config(payload, previous)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	s.modules.MinIOS3Config = cfg
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: "S3 configuration saved.",
		Data:    minioS3View(cfg, secretKeptHint),
	})
}

func (s *service) handleMinIOS3ConfigTest(w http.ResponseWriter, r *http.Request) {
	var payload minioS3ConfigPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid S3 test payload.")
		return
	}

	s.mu.RLock()
	previous := s.modules.MinIOS3Config
	s.mu.RUnlock()

	cfg, _, err := mergeMinIOS3Config(payload, previous)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := testMinIOS3Connection(cfg); err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("S3 connection failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Status:  "success",
		Message: "S3 connection successful.",
		Data: map[string]string{
			"provider": cfg.Provider,
			"bucket":   cfg.Bucket,
			"region":   cfg.Region,
		},
	})
}
