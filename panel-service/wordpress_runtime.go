package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func createRuntimeWordPressBackup(site WordPressSite, backupType string) (WordPressBackup, error) {
	_ = backupType // Full-backup policy: WordPress backups are always full.

	domain := normalizeDomain(site.Domain)
	if domain == "" {
		return WordPressBackup{}, fmt.Errorf("domain is required")
	}
	snapshot, err := createRuntimeSiteBackup(domain, siteBackupDir(), false)
	if err != nil {
		return WordPressBackup{}, err
	}
	info, statErr := os.Stat(snapshot.BackupPath)
	if statErr != nil {
		return WordPressBackup{}, statErr
	}
	return WordPressBackup{
		ID:         generateSecret(8),
		Domain:     domain,
		FileName:   filepath.Base(snapshot.BackupPath),
		BackupType: "full",
		SizeBytes:  info.Size(),
		CreatedAt:  time.Now().UTC().Unix(),
		Path:       snapshot.BackupPath,
	}, nil
}

func restoreRuntimeWordPressBackup(record WordPressBackup) error {
	snapshot := BackupSnapshot{
		Domain:     normalizeDomain(record.Domain),
		BackupPath: record.Path,
	}
	if snapshot.BackupPath == "" {
		snapshot.BackupPath = filepath.Join(siteBackupDir(), record.FileName)
	}
	return restoreRuntimeSiteBackup(snapshot, record.Domain)
}
