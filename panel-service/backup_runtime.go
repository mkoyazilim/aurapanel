package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	backupRetentionDefaultKeep = 14
	backupRetentionMinKeep     = 1
	backupRetentionMaxKeep     = 365
)

func dbBackupDir() string {
	return firstNonEmpty(strings.TrimSpace(os.Getenv("AURAPANEL_DB_BACKUP_DIR")), "/var/backups/aurapanel/db")
}

func siteBackupDir() string {
	return firstNonEmpty(strings.TrimSpace(os.Getenv("AURAPANEL_SITE_BACKUP_DIR")), "/var/backups/aurapanel/sites")
}

func ensureBackupDirectory(path string) error {
	return os.MkdirAll(path, 0o755)
}

func normalizeBackupRetentionKeep(value int) int {
	if value <= 0 {
		return backupRetentionDefaultKeep
	}
	if value < backupRetentionMinKeep {
		return backupRetentionMinKeep
	}
	if value > backupRetentionMaxKeep {
		return backupRetentionMaxKeep
	}
	return value
}

func backupRetentionKeepFromEnv() int {
	raw := strings.TrimSpace(os.Getenv("AURAPANEL_BACKUP_RETENTION_KEEP"))
	if raw == "" {
		return backupRetentionDefaultKeep
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return backupRetentionDefaultKeep
	}
	return normalizeBackupRetentionKeep(value)
}

func pathWithinRoot(target, root string) bool {
	target = filepath.Clean(strings.TrimSpace(target))
	root = filepath.Clean(strings.TrimSpace(root))
	if target == "" || root == "" || root == "." {
		return false
	}
	return target == root || strings.HasPrefix(target, root+string(os.PathSeparator))
}

func backupAllowedRootsForDomain(domain string) []string {
	roots := []string{
		siteBackupDir(),
		"/var/backups/aurapanel",
		"/home/backups",
	}

	normalizedDomain := normalizeDomain(domain)
	if normalizedDomain != "" {
		roots = append(roots, filepath.Join("/home", normalizedDomain))
	}

	if raw := strings.TrimSpace(os.Getenv("AURAPANEL_SITE_BACKUP_ALLOWED_ROOTS")); raw != "" {
		for _, item := range strings.Split(raw, ",") {
			candidate := strings.TrimSpace(item)
			if candidate == "" {
				continue
			}
			roots = append(roots, candidate)
		}
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		cleaned := filepath.Clean(strings.TrimSpace(root))
		if cleaned == "" || cleaned == "." {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		out = append(out, cleaned)
	}
	return out
}

func isSiteBackupPathAllowed(path, domain string) bool {
	for _, root := range backupAllowedRootsForDomain(domain) {
		if pathWithinRoot(path, root) {
			return true
		}
	}
	return false
}

func resolveSiteBackupTargetDir(domain, backupPath string) (string, error) {
	targetDir := strings.TrimSpace(backupPath)
	if targetDir == "" {
		targetDir = siteBackupDir()
	}
	if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(siteBackupDir(), targetDir)
	}
	targetDir = filepath.Clean(targetDir)
	if !isSiteBackupPathAllowed(targetDir, domain) {
		return "", fmt.Errorf("backup path is outside allowed roots: %s", targetDir)
	}
	return targetDir, nil
}

func listBackupArchiveEntries(path string) ([]string, error) {
	lower := strings.ToLower(strings.TrimSpace(path))
	var (
		out string
		err error
	)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		out, err = commandOutputTrimmed("unzip", "-Z1", path)
	case strings.HasSuffix(lower, ".tar"):
		out, err = commandOutputTrimmed("tar", "-tf", path)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		out, err = commandOutputTrimmed("tar", "-tzf", path)
	default:
		return nil, fmt.Errorf("unsupported archive format: %s", filepath.Ext(lower))
	}
	if err != nil {
		return nil, fmt.Errorf("archive integrity check failed: %w", err)
	}
	entries := make([]string, 0)
	for _, line := range strings.Split(out, "\n") {
		entry := strings.TrimSpace(line)
		if entry == "" {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func inspectBackupArchive(path string) (int, error) {
	entries, err := listBackupArchiveEntries(path)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		normalized := strings.ReplaceAll(entry, "\\", "/")
		cleaned := strings.ReplaceAll(filepath.Clean(normalized), "\\", "/")
		if strings.HasPrefix(cleaned, "/") || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
			return 0, fmt.Errorf("archive contains unsafe entry: %s", entry)
		}
		count++
	}
	return count, nil
}

func previewRuntimeSiteRestore(snapshot BackupSnapshot, domain string) (map[string]interface{}, error) {
	targetDomain := normalizeDomain(firstNonEmpty(domain, snapshot.Domain))
	if targetDomain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	archivePath := filepath.Clean(strings.TrimSpace(snapshot.BackupPath))
	if archivePath == "" || !fileExists(archivePath) {
		return nil, fmt.Errorf("backup snapshot not found")
	}
	if !isSiteBackupPathAllowed(archivePath, targetDomain) {
		return nil, fmt.Errorf("backup snapshot is outside allowed roots")
	}
	entryCount, err := inspectBackupArchive(archivePath)
	if err != nil {
		return nil, err
	}

	docroot := domainDocroot(targetDomain)
	parentDir := filepath.Dir(docroot)
	parentExists := fileExists(parentDir)

	var sizeBytes int64
	if info, statErr := os.Stat(archivePath); statErr == nil {
		sizeBytes = info.Size()
	}

	return map[string]interface{}{
		"domain":               targetDomain,
		"docroot":              docroot,
		"target_parent":        parentDir,
		"target_parent_exists": parentExists,
		"backup_path":          archivePath,
		"archive_entries":      entryCount,
		"size_bytes":           sizeBytes,
	}, nil
}

func streamCommandToGzipFile(path string, command string, args ...string) error {
	if err := ensureBackupDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	gz := gzip.NewWriter(file)
	defer gz.Close()

	cmd := exec.Command(command, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := io.Copy(gz, stdout); err != nil {
		_ = cmd.Wait()
		return err
	}
	errOutput, _ := io.ReadAll(stderr)
	if err := gz.Close(); err != nil {
		_ = cmd.Wait()
		return err
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(errOutput)))
	}
	return nil
}

func streamGzipFileToCommand(path string, command string, args ...string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()

	cmd := exec.Command(command, args...)
	cmd.Stdin = gz
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(output)))
	}
	return nil
}

func createRuntimeDBBackup(engine, dbName string) (DBBackupRecord, error) {
	engine = normalizeEngine(engine)
	dbName = sanitizeDBName(dbName)
	if dbName == "" {
		return DBBackupRecord{}, fmt.Errorf("database name is required")
	}
	if err := ensureBackupDirectory(dbBackupDir()); err != nil {
		return DBBackupRecord{}, err
	}
	filename := fmt.Sprintf("%s-%s.sql.gz", dbName, time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join(dbBackupDir(), filename)
	switch engine {
	case "mariadb":
		if err := streamCommandToGzipFile(path, "mysqldump", "--single-transaction", "--routines", "--events", dbName); err != nil {
			return DBBackupRecord{}, err
		}
	default:
		if err := streamCommandToGzipFile(path, "runuser", "-u", "postgres", "--", "pg_dump", dbName); err != nil {
			return DBBackupRecord{}, err
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return DBBackupRecord{}, err
	}
	return DBBackupRecord{
		ID:        generateSecret(8),
		DBName:    dbName,
		Filename:  filename,
		Engine:    engine,
		Size:      formatBytesHuman(info.Size()),
		CreatedAt: info.ModTime().UTC().UnixMilli(),
		Path:      path,
	}, nil
}

func resolveDBBackupPath(record DBBackupRecord) string {
	if strings.TrimSpace(record.Path) != "" {
		return record.Path
	}
	return filepath.Join(dbBackupDir(), record.Filename)
}

func restoreRuntimeDBBackup(record DBBackupRecord) error {
	if record.DBName == "" {
		record.DBName = sanitizeDBName(strings.SplitN(record.Filename, "-", 2)[0])
	}
	path := resolveDBBackupPath(record)
	switch normalizeEngine(record.Engine) {
	case "mariadb":
		return streamGzipFileToCommand(path, "mysql", record.DBName)
	default:
		return streamGzipFileToCommand(path, "runuser", "-u", "postgres", "--", "psql", record.DBName)
	}
}

func listRuntimeDBBackups(existing []DBBackupRecord) ([]DBBackupRecord, error) {
	if err := ensureBackupDirectory(dbBackupDir()); err != nil {
		return nil, err
	}
	meta := make(map[string]DBBackupRecord, len(existing))
	for _, item := range existing {
		meta[item.Filename] = item
	}
	entries, err := os.ReadDir(dbBackupDir())
	if err != nil {
		return nil, err
	}
	backups := make([]DBBackupRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql.gz") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil, infoErr
		}
		record := DBBackupRecord{
			ID:        generateSecret(8),
			DBName:    sanitizeDBName(strings.SplitN(entry.Name(), "-", 2)[0]),
			Filename:  entry.Name(),
			Engine:    "mariadb",
			Size:      formatBytesHuman(info.Size()),
			CreatedAt: info.ModTime().UTC().UnixMilli(),
			Path:      filepath.Join(dbBackupDir(), entry.Name()),
		}
		if stored, ok := meta[entry.Name()]; ok {
			if stored.ID != "" {
				record.ID = stored.ID
			}
			if stored.DBName != "" {
				record.DBName = stored.DBName
			}
			if stored.Engine != "" {
				record.Engine = stored.Engine
			}
			if stored.Path != "" {
				record.Path = stored.Path
			}
		}
		if strings.HasPrefix(record.Filename, "pg_") || strings.Contains(record.Filename, ".postgres.") {
			record.Engine = "postgresql"
		}
		backups = append(backups, record)
	}
	sort.Slice(backups, func(i, j int) bool { return backups[i].CreatedAt > backups[j].CreatedAt })
	return backups, nil
}

func deleteRuntimeDBBackup(record DBBackupRecord) error {
	return os.Remove(resolveDBBackupPath(record))
}

func createRuntimeSiteBackup(domain, backupPath string, incremental bool) (BackupSnapshot, error) {
	// Full-backup policy: incremental requests are ignored globally.
	incremental = false

	domain = normalizeDomain(domain)
	if domain == "" {
		return BackupSnapshot{}, fmt.Errorf("domain is required")
	}
	docroot := domainDocroot(domain)
	if !fileExists(docroot) {
		return BackupSnapshot{}, fmt.Errorf("docroot not found")
	}
	targetDir, err := resolveSiteBackupTargetDir(domain, backupPath)
	if err != nil {
		return BackupSnapshot{}, err
	}
	if err := ensureBackupDirectory(targetDir); err != nil {
		return BackupSnapshot{}, err
	}
	filename := fmt.Sprintf("%s-%s.tar.gz", strings.ReplaceAll(domain, ".", "_"), time.Now().UTC().Format("20060102-150405"))
	target := filepath.Join(targetDir, filename)
	parent := filepath.Dir(docroot)
	base := filepath.Base(docroot)
	if _, err := commandOutputTrimmed("tar", "-czf", target, "-C", parent, base); err != nil {
		return BackupSnapshot{}, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return BackupSnapshot{}, err
	}
	createdAt := info.ModTime().UTC().UnixMilli()
	hostname, _ := os.Hostname()
	tags := []string{"website", domain, "full"}
	return BackupSnapshot{
		ID:          generateSecret(8),
		ShortID:     generateSecret(4),
		Time:        time.UnixMilli(createdAt).UTC().Format(time.RFC3339),
		CreatedAt:   createdAt,
		Hostname:    firstNonEmpty(hostname, "aurapanel"),
		Tags:        tags,
		Domain:      domain,
		Incremental: false,
		SizeBytes:   info.Size(),
		BackupPath:  target,
	}, nil
}

func uploadedBackupArchiveExt(fileName string) (string, error) {
	lower := strings.ToLower(strings.TrimSpace(fileName))
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return ".tar.gz", nil
	case strings.HasSuffix(lower, ".tgz"):
		return ".tgz", nil
	case strings.HasSuffix(lower, ".zip"):
		return ".zip", nil
	case strings.HasSuffix(lower, ".tar"):
		return ".tar", nil
	default:
		return "", fmt.Errorf("unsupported archive type; use .tar.gz, .tgz, .tar or .zip")
	}
}

func createRuntimeSiteBackupFromArchive(domain, backupPath, fileName string, source io.Reader) (BackupSnapshot, error) {
	domain = normalizeDomain(domain)
	if domain == "" {
		return BackupSnapshot{}, fmt.Errorf("domain is required")
	}
	targetDir, err := resolveSiteBackupTargetDir(domain, backupPath)
	if err != nil {
		return BackupSnapshot{}, err
	}
	if err := ensureBackupDirectory(targetDir); err != nil {
		return BackupSnapshot{}, err
	}
	ext, err := uploadedBackupArchiveExt(fileName)
	if err != nil {
		return BackupSnapshot{}, err
	}

	filename := fmt.Sprintf("%s-upload-%s%s", strings.ReplaceAll(domain, ".", "_"), time.Now().UTC().Format("20060102-150405"), ext)
	target := filepath.Join(targetDir, filename)

	file, err := os.Create(target)
	if err != nil {
		return BackupSnapshot{}, err
	}
	if _, err := io.Copy(file, source); err != nil {
		_ = file.Close()
		_ = os.Remove(target)
		return BackupSnapshot{}, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(target)
		return BackupSnapshot{}, err
	}
	if _, err := inspectBackupArchive(target); err != nil {
		_ = os.Remove(target)
		return BackupSnapshot{}, err
	}

	info, err := os.Stat(target)
	if err != nil {
		_ = os.Remove(target)
		return BackupSnapshot{}, err
	}
	createdAt := info.ModTime().UTC().UnixMilli()
	hostname, _ := os.Hostname()

	return BackupSnapshot{
		ID:          generateSecret(8),
		ShortID:     generateSecret(4),
		Time:        time.UnixMilli(createdAt).UTC().Format(time.RFC3339),
		CreatedAt:   createdAt,
		Hostname:    firstNonEmpty(hostname, "aurapanel"),
		Tags:        []string{"website", domain, "uploaded"},
		Domain:      domain,
		Incremental: false,
		SizeBytes:   info.Size(),
		BackupPath:  target,
	}, nil
}

func restoreRuntimeSiteBackup(snapshot BackupSnapshot, domain string) error {
	preview, err := previewRuntimeSiteRestore(snapshot, domain)
	if err != nil {
		return err
	}
	targetDomain := normalizeDomain(firstNonEmpty(domain, snapshot.Domain))
	docroot := domainDocroot(targetDomain)
	if err := ensureBackupDirectory(filepath.Dir(docroot)); err != nil {
		return err
	}
	archivePath, _ := preview["backup_path"].(string)
	lower := strings.ToLower(strings.TrimSpace(archivePath))
	switch {
	case strings.HasSuffix(lower, ".zip"):
		if _, err := commandOutputTrimmed("unzip", "-o", archivePath, "-d", filepath.Dir(docroot)); err != nil {
			return err
		}
	case strings.HasSuffix(lower, ".tar"):
		if _, err := commandOutputTrimmed("tar", "-xf", archivePath, "-C", filepath.Dir(docroot)); err != nil {
			return err
		}
	default:
		if _, err := commandOutputTrimmed("tar", "-xzf", archivePath, "-C", filepath.Dir(docroot)); err != nil {
			return err
		}
	}
	return nil
}
