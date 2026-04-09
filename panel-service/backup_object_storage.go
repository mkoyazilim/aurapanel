package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awscreds "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type backupObjectStoreConfig struct {
	Provider     string
	Region       string
	Bucket       string
	Endpoint     string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
	Prefix       string
}

func cleanObjectPath(raw string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	normalized = strings.Trim(normalized, "/")
	if normalized == "" {
		return ""
	}
	parts := strings.Split(normalized, "/")
	out := make([]string, 0, len(parts))
	for _, item := range parts {
		segment := strings.TrimSpace(item)
		if segment == "" || segment == "." || segment == ".." {
			continue
		}
		out = append(out, segment)
	}
	return strings.Join(out, "/")
}

func joinObjectPath(parts ...string) string {
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		cleaned := cleanObjectPath(part)
		if cleaned == "" {
			continue
		}
		items = append(items, cleaned)
	}
	return strings.Join(items, "/")
}

func defaultBackupObjectKey(domain string, createdAt int64, backupPath, prefix string) string {
	safeDomain := strings.ReplaceAll(normalizeDomain(domain), ".", "_")
	if safeDomain == "" {
		safeDomain = "unknown"
	}
	day := time.Now().UTC().Format("20060102")
	if createdAt > 0 {
		day = time.UnixMilli(createdAt).UTC().Format("20060102")
	}
	fileName := strings.TrimSpace(filepath.Base(backupPath))
	if fileName == "" || fileName == "." || fileName == string(filepath.Separator) {
		fileName = fmt.Sprintf("backup-%d.tar.gz", time.Now().UTC().Unix())
	}
	return joinObjectPath(prefix, "sites", safeDomain, day, fileName)
}

func shouldUsePathStyleForEndpoint(endpoint string) bool {
	value := strings.ToLower(strings.TrimSpace(endpoint))
	if value == "" {
		return false
	}
	return !strings.Contains(value, "amazonaws.com")
}

func internalMinIOBackupConfigFromEnv() (backupObjectStoreConfig, bool) {
	target := strings.ToLower(strings.TrimSpace(os.Getenv("AURAPANEL_BACKUP_TARGET")))
	endpoint := normalizeS3Endpoint(firstNonEmpty(strings.TrimSpace(os.Getenv("AURAPANEL_BACKUP_MINIO_ENDPOINT")), "http://127.0.0.1:9000"))
	bucket := normalizeS3Bucket(firstNonEmpty(strings.TrimSpace(os.Getenv("AURAPANEL_BACKUP_MINIO_BUCKET")), "aurapanel-backups"))
	access := strings.TrimSpace(os.Getenv("AURAPANEL_BACKUP_MINIO_ACCESS_KEY"))
	secret := strings.TrimSpace(os.Getenv("AURAPANEL_BACKUP_MINIO_SECRET_KEY"))

	if access == "" || secret == "" {
		return backupObjectStoreConfig{}, false
	}
	if target != "internal-minio" && strings.TrimSpace(os.Getenv("AURAPANEL_BACKUP_MINIO_ENDPOINT")) == "" {
		return backupObjectStoreConfig{}, false
	}
	if bucket == "" {
		return backupObjectStoreConfig{}, false
	}

	return backupObjectStoreConfig{
		Provider:     "internal-minio",
		Region:       "us-east-1",
		Bucket:       bucket,
		Endpoint:     endpoint,
		AccessKey:    access,
		SecretKey:    secret,
		UsePathStyle: true,
	}, true
}

func runtimeMinIOS3BackupConfig(cfg MinIOS3Config) (backupObjectStoreConfig, bool) {
	bucket := normalizeS3Bucket(cfg.Bucket)
	access := strings.TrimSpace(cfg.AccessKey)
	secret := strings.TrimSpace(cfg.SecretKey)
	if bucket == "" || access == "" || secret == "" {
		return backupObjectStoreConfig{}, false
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}
	endpoint := normalizeS3Endpoint(cfg.Endpoint)
	usePathStyle := cfg.UsePathStyle
	if !usePathStyle && shouldUsePathStyleForEndpoint(endpoint) {
		usePathStyle = true
	}
	return backupObjectStoreConfig{
		Provider:     normalizeMinIOS3Provider(cfg.Provider),
		Region:       region,
		Bucket:       bucket,
		Endpoint:     endpoint,
		AccessKey:    access,
		SecretKey:    secret,
		UsePathStyle: usePathStyle,
	}, true
}

func parseS3HTTPRepo(repo string) (endpoint, bucket, prefix string, err error) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(repo), "s3:"))
	parsed, parseErr := url.Parse(raw)
	if parseErr != nil {
		return "", "", "", parseErr
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", "", "", fmt.Errorf("invalid remote repository endpoint")
	}
	pathParts := strings.Split(cleanObjectPath(parsed.Path), "/")
	if len(pathParts) == 0 || strings.TrimSpace(pathParts[0]) == "" {
		return "", "", "", fmt.Errorf("bucket is required in remote repository")
	}
	bucket = normalizeS3Bucket(pathParts[0])
	if bucket == "" {
		return "", "", "", fmt.Errorf("bucket is required in remote repository")
	}
	prefix = ""
	if len(pathParts) > 1 {
		prefix = joinObjectPath(pathParts[1:]...)
	}
	endpoint = normalizeS3Endpoint(parsed.Scheme + "://" + parsed.Host)
	return endpoint, bucket, prefix, nil
}

func parseS3StyleRepo(repo string) (bucket, prefix string, err error) {
	parsed, parseErr := url.Parse(strings.TrimSpace(repo))
	if parseErr != nil {
		return "", "", parseErr
	}
	bucket = normalizeS3Bucket(parsed.Host)
	if bucket == "" {
		return "", "", fmt.Errorf("bucket is required in remote repository")
	}
	prefix = cleanObjectPath(parsed.Path)
	return bucket, prefix, nil
}

func resolveBackupObjectStoreConfig(destination BackupDestination, runtimeCfg MinIOS3Config) (backupObjectStoreConfig, bool, error) {
	remoteRepo := strings.TrimSpace(destination.RemoteRepo)
	if remoteRepo == "" {
		if strings.EqualFold(strings.TrimSpace(os.Getenv("AURAPANEL_BACKUP_TARGET")), "internal-minio") {
			cfg, ok := internalMinIOBackupConfigFromEnv()
			if !ok {
				return backupObjectStoreConfig{}, false, fmt.Errorf("internal MinIO target is enabled but not fully configured")
			}
			return cfg, true, nil
		}
		if cfg, ok := runtimeMinIOS3BackupConfig(runtimeCfg); ok {
			return cfg, true, nil
		}
		return backupObjectStoreConfig{}, false, nil
	}

	lower := strings.ToLower(remoteRepo)
	switch {
	case lower == "disabled" || lower == "local" || lower == "filesystem":
		return backupObjectStoreConfig{}, false, nil

	case lower == "internal-minio" || lower == "minio":
		cfg, ok := internalMinIOBackupConfigFromEnv()
		if !ok {
			return backupObjectStoreConfig{}, false, fmt.Errorf("internal MinIO environment is not configured")
		}
		return cfg, true, nil

	case strings.HasPrefix(lower, "minio://"):
		cfg, ok := internalMinIOBackupConfigFromEnv()
		if !ok {
			return backupObjectStoreConfig{}, false, fmt.Errorf("internal MinIO environment is not configured")
		}
		bucket, prefix, err := parseS3StyleRepo(remoteRepo)
		if err != nil {
			return backupObjectStoreConfig{}, false, err
		}
		cfg.Bucket = bucket
		cfg.Prefix = prefix
		return cfg, true, nil

	case strings.HasPrefix(lower, "s3://"):
		base, ok := runtimeMinIOS3BackupConfig(runtimeCfg)
		if !ok {
			base, ok = internalMinIOBackupConfigFromEnv()
		}
		if !ok {
			return backupObjectStoreConfig{}, false, fmt.Errorf("S3 configuration is not available")
		}
		bucket, prefix, err := parseS3StyleRepo(remoteRepo)
		if err != nil {
			return backupObjectStoreConfig{}, false, err
		}
		base.Bucket = bucket
		base.Prefix = prefix
		return base, true, nil

	case strings.HasPrefix(lower, "s3:http://") || strings.HasPrefix(lower, "s3:https://"):
		base, ok := runtimeMinIOS3BackupConfig(runtimeCfg)
		if !ok {
			base, ok = internalMinIOBackupConfigFromEnv()
		}
		if !ok {
			return backupObjectStoreConfig{}, false, fmt.Errorf("S3 configuration is not available")
		}
		endpoint, bucket, prefix, err := parseS3HTTPRepo(remoteRepo)
		if err != nil {
			return backupObjectStoreConfig{}, false, err
		}
		base.Endpoint = endpoint
		base.Bucket = bucket
		base.Prefix = prefix
		if shouldUsePathStyleForEndpoint(base.Endpoint) {
			base.UsePathStyle = true
		}
		return base, true, nil

	default:
		return backupObjectStoreConfig{}, false, fmt.Errorf("unsupported remote repository format")
	}
}

func newObjectStoreClient(cfg backupObjectStoreConfig) (*s3.S3, error) {
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}
	awsCfg := aws.NewConfig().
		WithRegion(region).
		WithCredentials(awscreds.NewStaticCredentials(strings.TrimSpace(cfg.AccessKey), strings.TrimSpace(cfg.SecretKey), "")).
		WithS3ForcePathStyle(cfg.UsePathStyle)
	if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
		awsCfg = awsCfg.WithEndpoint(endpoint)
		if strings.HasPrefix(strings.ToLower(endpoint), "http://") {
			awsCfg = awsCfg.WithDisableSSL(true)
		}
	}
	sess, err := session.NewSession(awsCfg)
	if err != nil {
		return nil, err
	}
	return s3.New(sess), nil
}

func bucketMissingError(err error) bool {
	if err == nil {
		return false
	}
	if requestErr, ok := err.(awserr.RequestFailure); ok {
		return requestErr.StatusCode() == http.StatusNotFound
	}
	if awsErr, ok := err.(awserr.Error); ok {
		switch strings.ToLower(strings.TrimSpace(awsErr.Code())) {
		case "notfound", "nosuchbucket":
			return true
		}
	}
	return false
}

func ensureObjectStoreBucket(cfg backupObjectStoreConfig) error {
	client, err := newObjectStoreClient(cfg)
	if err != nil {
		return err
	}
	bucket := normalizeS3Bucket(cfg.Bucket)
	if bucket == "" {
		return fmt.Errorf("bucket is required")
	}
	_, err = client.HeadBucket(&s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		return nil
	}
	if !bucketMissingError(err) {
		return err
	}

	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}
	if strings.TrimSpace(cfg.Region) != "" && strings.TrimSpace(cfg.Region) != "us-east-1" && cfg.Provider != "internal-minio" {
		input.CreateBucketConfiguration = &s3.CreateBucketConfiguration{
			LocationConstraint: aws.String(strings.TrimSpace(cfg.Region)),
		}
	}
	_, err = client.CreateBucket(input)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			code := strings.TrimSpace(awsErr.Code())
			if code == "BucketAlreadyOwnedByYou" || code == "BucketAlreadyExists" {
				return nil
			}
		}
		return err
	}
	return nil
}

func uploadBackupArchiveToObjectStore(cfg backupObjectStoreConfig, backupPath, objectKey string) error {
	localPath := filepath.Clean(strings.TrimSpace(backupPath))
	if localPath == "" {
		return fmt.Errorf("backup archive path is required")
	}
	key := cleanObjectPath(objectKey)
	if key == "" {
		return fmt.Errorf("object key is required")
	}
	if err := ensureObjectStoreBucket(cfg); err != nil {
		return err
	}
	client, err := newObjectStoreClient(cfg)
	if err != nil {
		return err
	}
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	return err
}

func downloadBackupArchiveFromObjectStore(cfg backupObjectStoreConfig, objectKey, targetPath string) error {
	key := cleanObjectPath(objectKey)
	if key == "" {
		return fmt.Errorf("object key is required")
	}
	target := filepath.Clean(strings.TrimSpace(targetPath))
	if target == "" {
		return fmt.Errorf("download path is required")
	}
	if err := ensureBackupDirectory(filepath.Dir(target)); err != nil {
		return err
	}
	client, err := newObjectStoreClient(cfg)
	if err != nil {
		return err
	}
	out, err := client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer out.Body.Close()

	file, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, out.Body); err != nil {
		_ = file.Close()
		_ = os.Remove(target)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(target)
		return err
	}
	return nil
}

func deleteBackupArchiveFromObjectStore(cfg backupObjectStoreConfig, objectKey string) error {
	key := cleanObjectPath(objectKey)
	if key == "" {
		return nil
	}
	client, err := newObjectStoreClient(cfg)
	if err != nil {
		return err
	}
	_, err = client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	})
	return err
}

func listObjectStoreBuckets(cfg backupObjectStoreConfig) ([]string, error) {
	client, err := newObjectStoreClient(cfg)
	if err != nil {
		return nil, err
	}
	out, err := client.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}
	items := make([]string, 0, len(out.Buckets))
	for _, item := range out.Buckets {
		name := normalizeS3Bucket(aws.StringValue(item.Name))
		if name == "" {
			continue
		}
		items = append(items, name)
	}
	sort.Strings(items)
	return items, nil
}

func createObjectStoreBucket(cfg backupObjectStoreConfig, bucketName string) error {
	bucket := normalizeS3Bucket(bucketName)
	if bucket == "" {
		return fmt.Errorf("bucket name is required")
	}
	cfg.Bucket = bucket
	return ensureObjectStoreBucket(cfg)
}

func ensureBackupSnapshotLocalCopy(snapshot BackupSnapshot, destination BackupDestination, runtimeCfg MinIOS3Config) (BackupSnapshot, error) {
	localPath := filepath.Clean(strings.TrimSpace(snapshot.BackupPath))
	if localPath != "" && fileExists(localPath) {
		return snapshot, nil
	}

	remoteKey := cleanObjectPath(snapshot.RemoteObjectKey)
	if remoteKey == "" {
		return snapshot, fmt.Errorf("backup snapshot not found")
	}

	cfg, enabled, err := resolveBackupObjectStoreConfig(destination, runtimeCfg)
	if err != nil {
		return snapshot, err
	}
	if !enabled {
		return snapshot, fmt.Errorf("backup snapshot not found")
	}
	if bucket := normalizeS3Bucket(snapshot.RemoteBucket); bucket != "" {
		cfg.Bucket = bucket
	}

	targetDomain := normalizeDomain(snapshot.Domain)
	if targetDomain == "" {
		targetDomain = "unknown"
	}
	fileName := filepath.Base(remoteKey)
	if fileName == "" || fileName == "." || fileName == string(filepath.Separator) {
		fileName = fmt.Sprintf("%s-remote-%d.tar.gz", strings.ReplaceAll(targetDomain, ".", "_"), time.Now().UTC().Unix())
	}
	cachePath := filepath.Join(siteBackupDir(), ".remote-cache", targetDomain, fileName)
	if err := downloadBackupArchiveFromObjectStore(cfg, remoteKey, cachePath); err != nil {
		return snapshot, err
	}
	snapshot.BackupPath = cachePath
	return snapshot, nil
}
