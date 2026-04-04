package main

import (
	"archive/zip"
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

func managedRoots() []string {
	raw := strings.TrimSpace(os.Getenv("AURAPANEL_ALLOWED_PATHS"))
	if raw == "" {
		raw = "/home,/var/www,/usr/local/lsws,/etc/letsencrypt,/var/log,/opt/aurapanel"
	}
	parts := strings.Split(raw, ",")
	roots := make([]string, 0, len(parts))
	for _, item := range parts {
		cleaned := filepath.Clean(strings.TrimSpace(item))
		if cleaned == "." || cleaned == "" {
			continue
		}
		roots = append(roots, cleaned)
	}
	sort.Strings(roots)
	return roots
}

func resolveManagedPath(input string) (string, error) {
	target := strings.TrimSpace(input)
	if target == "" {
		target = "/home"
	}
	if !filepath.IsAbs(target) && !strings.HasPrefix(target, "/") {
		target = filepath.Join("/home", target)
	}
	target = filepath.Clean(target)
	for _, root := range managedRoots() {
		if target == root || strings.HasPrefix(target, root+string(os.PathSeparator)) {
			return target, nil
		}
	}
	return "", fmt.Errorf("path is outside managed roots: %s", target)
}

func listManagedEntries(path string) ([]virtualFileEntry, error) {
	resolved, err := resolveManagedPath(path)
	if err != nil {
		return nil, err
	}
	items, err := os.ReadDir(resolved)
	if err != nil {
		return nil, err
	}
	entries := make([]virtualFileEntry, 0, len(items))
	for _, item := range items {
		info, err := item.Info()
		if err != nil {
			continue
		}
		entries = append(entries, virtualFileEntry{
			Name:        item.Name(),
			IsDir:       item.IsDir(),
			Size:        info.Size(),
			Permissions: info.Mode().Perm().String(),
			Modified:    info.ModTime().Unix(),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

func readManagedFile(path string) (string, error) {
	resolved, err := resolveManagedPath(path)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(resolved)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func writeManagedFile(path, content string) error {
	resolved, err := resolveManagedPath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(resolved, []byte(content), 0o644); err != nil {
		return err
	}
	applyManagedPathOwnershipFromParent(resolved)
	return nil
}

func renameManagedPath(oldPath, newPath string) error {
	from, err := resolveManagedPath(oldPath)
	if err != nil {
		return err
	}
	to, err := resolveManagedPath(newPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	if err := os.Rename(from, to); err != nil {
		return err
	}
	applyManagedPathOwnershipFromParent(to)
	return nil
}

func trashManagedPath(path string) error {
	source, err := resolveManagedPath(path)
	if err != nil {
		return err
	}
	base := filepath.Base(source)
	destDir := filepath.Join("/home", "backups", ".trash")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(destDir, fmt.Sprintf("%d-%s", time.Now().UTC().Unix(), base))
	return os.Rename(source, dest)
}

func deleteManagedPath(path string) error {
	resolved, err := resolveManagedPath(path)
	if err != nil {
		return err
	}
	return os.RemoveAll(resolved)
}

func createManagedDir(path string) error {
	resolved, err := resolveManagedPath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(resolved, 0o755); err != nil {
		return err
	}
	applyManagedPathOwnershipFromParent(resolved)
	return nil
}

func setManagedPermissions(path, modeValue string) error {
	resolved, err := resolveManagedPath(path)
	if err != nil {
		return err
	}
	mode, err := parseOctalFileMode(modeValue)
	if err != nil {
		return err
	}
	return os.Chmod(resolved, mode)
}

func parseOctalFileMode(value string) (os.FileMode, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return 0, errors.New("permission mode is required")
	}
	if len(raw) == 3 {
		raw = "0" + raw
	}
	if len(raw) != 4 {
		return 0, errors.New("permission mode must be 3 or 4 octal digits")
	}
	for _, ch := range raw {
		if ch < '0' || ch > '7' {
			return 0, errors.New("permission mode must contain only digits 0-7")
		}
	}
	parsed, err := strconv.ParseUint(raw, 8, 32)
	if err != nil {
		return 0, errors.New("invalid permission mode")
	}
	return os.FileMode(parsed), nil
}

func runManagedArchiveCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %s", command, strings.TrimSpace(string(output)))
	}
	return nil
}

func binaryAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func createZipArchive(dest string, sources []string) error {
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	defer writer.Close()

	for _, source := range sources {
		sourceInfo, err := os.Stat(source)
		if err != nil {
			return err
		}
		baseParent := filepath.Dir(source)

		if !sourceInfo.IsDir() {
			if err := addPathToZip(writer, source, baseParent); err != nil {
				return err
			}
			continue
		}

		if err := filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			return addPathToZip(writer, path, baseParent)
		}); err != nil {
			return err
		}
	}

	return nil
}

func addPathToZip(writer *zip.Writer, path, baseParent string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(baseParent, path)
	if err != nil {
		return err
	}
	name := filepath.ToSlash(rel)
	if name == "." || name == "" {
		return nil
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = name
	if info.IsDir() {
		if !strings.HasSuffix(header.Name, "/") {
			header.Name += "/"
		}
		_, err = writer.CreateHeader(header)
		return err
	}

	header.Method = zip.Deflate
	out, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(out, in)
	return err
}

func extractZipArchive(archivePath, destDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	destClean := filepath.Clean(destDir)
	for _, entry := range reader.File {
		entryName := filepath.Clean(entry.Name)
		if entryName == "." || entryName == "" {
			continue
		}
		target := filepath.Clean(filepath.Join(destClean, entryName))
		if target != destClean && !strings.HasPrefix(target, destClean+string(os.PathSeparator)) {
			return fmt.Errorf("zip contains invalid path: %s", entry.Name)
		}

		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		in, err := entry.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, entry.Mode())
		if err != nil {
			in.Close()
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			in.Close()
			return err
		}
		out.Close()
		in.Close()
	}
	return nil
}

func compressManagedFiles(destPath string, sources []string, format string) error {
	if len(sources) == 0 {
		return errors.New("at least one source path is required")
	}
	dest, err := resolveManagedPath(destPath)
	if err != nil {
		return err
	}
	resolvedSources := make([]string, 0, len(sources))
	for _, source := range sources {
		resolved, err := resolveManagedPath(source)
		if err != nil {
			return err
		}
		resolvedSources = append(resolvedSources, resolved)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "zip":
		if binaryAvailable("zip") {
			args := append([]string{"-r", dest}, resolvedSources...)
			return runManagedArchiveCommand("zip", args...)
		}
		return createZipArchive(dest, resolvedSources)
	default:
		args := append([]string{"-czf", dest}, resolvedSources...)
		return runManagedArchiveCommand("tar", args...)
	}
}

func extractManagedArchive(archivePath, destDir string) error {
	archive, err := resolveManagedPath(archivePath)
	if err != nil {
		return err
	}
	dest, err := resolveManagedPath(destDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	lower := strings.ToLower(archive)
	var extractErr error
	switch {
	case strings.HasSuffix(lower, ".zip"):
		if binaryAvailable("unzip") {
			extractErr = runManagedArchiveCommand("unzip", "-o", archive, "-d", dest)
			break
		}
		extractErr = extractZipArchive(archive, dest)
	case strings.HasSuffix(lower, ".tar"):
		extractErr = runManagedArchiveCommand("tar", "-xf", archive, "-C", dest)
	default:
		extractErr = runManagedArchiveCommand("tar", "-xzf", archive, "-C", dest)
	}
	if extractErr != nil {
		return extractErr
	}
	applyManagedTreeOwnershipFromParent(dest)
	return nil
}

func applyManagedPathOwnershipFromParent(path string) {
	if runtime.GOOS == "windows" {
		return
	}
	parent := filepath.Dir(path)
	uid, gid, err := managedPathOwnerIDs(parent)
	if err != nil {
		return
	}
	_ = os.Chown(path, uid, gid)
}

func applyManagedTreeOwnershipFromParent(path string) {
	if runtime.GOOS == "windows" {
		return
	}
	parent := filepath.Dir(path)
	uid, gid, err := managedPathOwnerIDs(parent)
	if err != nil {
		return
	}
	_ = filepath.Walk(path, func(current string, _ os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		_ = os.Chown(current, uid, gid)
		return nil
	})
}

func managedPathOwnerIDs(path string) (int, int, error) {
	output, err := commandOutputTrimmed("stat", "-c", "%u:%g", path)
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Split(strings.TrimSpace(output), ":")
	if len(parts) != 2 {
		return 0, 0, errors.New("path ownership metadata unavailable")
	}
	uid, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	gid, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return uid, gid, nil
}

func tailManagedFile(path string, limit int) ([]string, error) {
	resolved, err := resolveManagedPath(path)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(resolved)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	lines := make([]string, 0, limit)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > limit {
			lines = lines[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func discoverSiteLogPaths(domain, kind string) []string {
	candidates := []string{
		fmt.Sprintf("/usr/local/lsws/logs/%s.%s.log", domain, kind),
		fmt.Sprintf("/usr/local/lsws/logs/%s_%s.log", domain, kind),
		fmt.Sprintf("/usr/local/lsws/logs/%s/%s.log", domain, kind),
		fmt.Sprintf("/var/log/nginx/%s.%s.log", domain, kind),
		fmt.Sprintf("/var/log/apache2/%s.%s.log", domain, kind),
	}
	filtered := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func realSiteLogs(domain, kind string) ([]string, error) {
	paths := discoverSiteLogPaths(domain, kind)
	if len(paths) == 0 {
		return nil, fmt.Errorf("no %s log file found for %s", kind, domain)
	}
	return tailManagedFile(paths[0], 200)
}

func parseAccessLogLine(line string) (time.Time, string, bool) {
	start := strings.Index(line, "[")
	end := strings.Index(line, "]")
	if start == -1 || end == -1 || end <= start+1 {
		return time.Time{}, "", false
	}
	timestamp, err := time.Parse("02/Jan/2006:15:04:05 -0700", line[start+1:end])
	if err != nil {
		return time.Time{}, "", false
	}
	parts := strings.Split(line, "\"")
	if len(parts) < 2 {
		return time.Time{}, "", true
	}
	requestParts := strings.Fields(parts[1])
	path := "/"
	if len(requestParts) >= 2 {
		path = requestParts[1]
	}
	return timestamp, path, true
}

func collectWebsiteTraffic(domain string, hours int) (map[string]interface{}, error) {
	paths := discoverSiteLogPaths(domain, "access")
	if len(paths) == 0 {
		return nil, fmt.Errorf("no access log file found for %s", domain)
	}
	lines, err := tailManagedFile(paths[0], 5000)
	if err != nil {
		return nil, err
	}

	type bucketStats struct {
		Hits      int
		Visitors  int
		Bandwidth int64
	}
	cutoff := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	buckets := map[string]*bucketStats{}
	topPaths := map[string]int{}
	totalHits := 0
	totalBandwidth := int64(0)

	for _, line := range lines {
		timestamp, path, ok := parseAccessLogLine(line)
		if !ok || timestamp.Before(cutoff) {
			continue
		}
		key := timestamp.UTC().Truncate(time.Hour).Format("02 Jan 15:04")
		stats := buckets[key]
		if stats == nil {
			stats = &bucketStats{}
			buckets[key] = stats
		}
		stats.Hits++
		stats.Visitors++
		stats.Bandwidth += int64(len(line))
		topPaths[path]++
		totalHits++
		totalBandwidth += int64(len(line))
	}

	keys := make([]string, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	series := make([]map[string]interface{}, 0, len(keys))
	totalVisitors := 0
	for _, key := range keys {
		stats := buckets[key]
		totalVisitors += stats.Visitors
		series = append(series, map[string]interface{}{
			"bucket":          key,
			"hits":            stats.Hits,
			"visitors":        stats.Visitors,
			"bandwidth_bytes": stats.Bandwidth,
		})
	}

	type topPathEntry struct {
		Path  string
		Count int
	}
	top := make([]topPathEntry, 0, len(topPaths))
	for path, count := range topPaths {
		top = append(top, topPathEntry{Path: path, Count: count})
	}
	sort.Slice(top, func(i, j int) bool { return top[i].Count > top[j].Count })
	if len(top) > 10 {
		top = top[:10]
	}
	topPayload := make([]map[string]interface{}, 0, len(top))
	for _, item := range top {
		topPayload = append(topPayload, map[string]interface{}{
			"path":            item.Path,
			"hits":            item.Count,
			"bandwidth_bytes": int64(item.Count * 512),
		})
	}

	return map[string]interface{}{
		"totals": map[string]interface{}{
			"hits":            totalHits,
			"visitors":        totalVisitors,
			"bandwidth_bytes": totalBandwidth,
		},
		"series":    series,
		"top_paths": topPayload,
	}, nil
}

func runInteractiveShell(command, cwd string) (string, string) {
	if strings.TrimSpace(command) == "" {
		return "", cwd
	}
	if cwd == "" {
		cwd = "/home"
	}
	if strings.HasPrefix(strings.TrimSpace(command), "cd ") {
		target := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(command), "cd"))
		if !filepath.IsAbs(target) {
			target = filepath.Join(cwd, target)
		}
		resolved, err := resolveManagedPath(target)
		if err != nil {
			return err.Error(), cwd
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			return "directory not found", cwd
		}
		return "", resolved
	}

	resolvedCwd, err := resolveManagedPath(cwd)
	if err != nil {
		return err.Error(), cwd
	}

	shell := "/bin/sh"
	args := []string{"-lc", command}
	if os.PathSeparator == '\\' {
		shell = "powershell"
		args = []string{"-NoProfile", "-Command", command}
	}
	cmd := exec.Command(shell, args...)
	cmd.Dir = resolvedCwd
	output, err := cmd.CombinedOutput()
	nextCwd := resolvedCwd
	if err != nil {
		if len(output) == 0 {
			return err.Error(), nextCwd
		}
		return string(output), nextCwd
	}
	return string(output), nextCwd
}
