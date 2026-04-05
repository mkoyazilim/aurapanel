package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

func systemUserExists(username string) bool {
	if strings.TrimSpace(username) == "" {
		return false
	}
	_, err := user.Lookup(username)
	return err == nil
}

func systemGroupExists(group string) bool {
	if strings.TrimSpace(group) == "" {
		return false
	}
	_, err := user.LookupGroup(group)
	return err == nil
}

func ensureSystemGroup(group string) error {
	if systemGroupExists(group) {
		return nil
	}
	_, err := commandOutputTrimmed("groupadd", group)
	return err
}

func resolvedTransferOwner(homeDir, ownerHint string) string {
	ownerHint = sanitizeName(ownerHint)
	if ownerHint != "" && systemUserExists(ownerHint) {
		return ownerHint
	}
	homeDir = filepath.Clean(strings.TrimSpace(homeDir))
	parts := strings.Split(filepath.ToSlash(homeDir), "/")
	if len(parts) >= 3 && parts[1] == "home" && systemUserExists(parts[2]) {
		return parts[2]
	}
	for _, candidate := range []string{"www-data", "nobody"} {
		if systemUserExists(candidate) {
			return candidate
		}
	}
	return "nobody"
}

func ensureOwnedDirectory(path, owner string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	if _, err := commandOutputTrimmed("chown", "-R", fmt.Sprintf("%s:%s", owner, owner), path); err != nil && owner != "root" {
		_, _ = commandOutputTrimmed("chown", "-R", owner, path)
	}
	return nil
}

func pureFTPdList() ([]TransferAccount, error) {
	if _, err := exec.LookPath("pure-pw"); err != nil {
		return nil, fmt.Errorf("pure-pw not found")
	}
	output, err := commandOutputTrimmed("pure-pw", "list")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unable to open") {
			return []TransferAccount{}, nil
		}
		return nil, err
	}
	accounts := []TransferAccount{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		username := sanitizeName(fields[0])
		if username == "" {
			continue
		}
		account := TransferAccount{Username: username}
		showOutput, showErr := commandOutputTrimmed("pure-pw", "show", username)
		if showErr == nil {
			for _, showLine := range strings.Split(showOutput, "\n") {
				if !strings.Contains(showLine, ":") {
					continue
				}
				key, value, _ := strings.Cut(showLine, ":")
				if strings.EqualFold(strings.TrimSpace(key), "Directory") {
					account.HomeDir = strings.TrimSpace(value)
				}
			}
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func createRuntimeFTPAccount(username, password, homeDir, ownerHint string) error {
	if _, err := exec.LookPath("pure-pw"); err != nil {
		return fmt.Errorf("pure-pw not found")
	}
	owner := resolvedTransferOwner(homeDir, ownerHint)
	if err := ensureOwnedDirectory(homeDir, owner); err != nil {
		return err
	}
	cmd := exec.Command("pure-pw", "useradd", username, "-u", owner, "-d", homeDir, "-m")
	cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(output)))
	}
	return nil
}

func updateRuntimeFTPPassword(username, password string) error {
	if _, err := exec.LookPath("pure-pw"); err != nil {
		return fmt.Errorf("pure-pw not found")
	}
	cmd := exec.Command("pure-pw", "passwd", username, "-m")
	cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(output)))
	}
	return nil
}

func deleteRuntimeFTPAccount(username string) error {
	if _, err := exec.LookPath("pure-pw"); err != nil {
		return fmt.Errorf("pure-pw not found")
	}
	_, err := commandOutputTrimmed("pure-pw", "userdel", username, "-m")
	return err
}

func sftpGroupName() string {
	return firstNonEmpty(strings.TrimSpace(os.Getenv("AURAPANEL_SFTP_GROUP")), "aurapanel-sftp")
}

func sftpShellPath() string {
	return firstNonEmpty(strings.TrimSpace(os.Getenv("AURAPANEL_SFTP_SHELL")), "/bin/bash")
}

func parsePasswdEntry(username string) (TransferAccount, bool) {
	output, err := commandOutputTrimmed("getent", "passwd", username)
	if err != nil || strings.TrimSpace(output) == "" {
		return TransferAccount{}, false
	}
	fields := strings.Split(strings.TrimSpace(output), ":")
	if len(fields) < 7 {
		return TransferAccount{}, false
	}
	return TransferAccount{
		Username: sanitizeName(fields[0]),
		HomeDir:  fields[5],
	}, true
}

func runtimeSFTPAccounts() ([]TransferAccount, error) {
	group := sftpGroupName()
	groupOut, err := commandOutputTrimmed("getent", "group", group)
	if err != nil || strings.TrimSpace(groupOut) == "" {
		return []TransferAccount{}, nil
	}
	fields := strings.Split(strings.TrimSpace(groupOut), ":")
	if len(fields) < 4 {
		return []TransferAccount{}, nil
	}
	memberSet := map[string]struct{}{}
	if fields[2] != "" {
		passwdOut, passwdErr := commandOutputTrimmed("getent", "passwd")
		if passwdErr == nil {
			for _, line := range strings.Split(passwdOut, "\n") {
				parts := strings.Split(strings.TrimSpace(line), ":")
				if len(parts) >= 7 && parts[3] == fields[2] {
					memberSet[sanitizeName(parts[0])] = struct{}{}
				}
			}
		}
	}
	for _, member := range strings.Split(fields[3], ",") {
		member = sanitizeName(member)
		if member != "" {
			memberSet[member] = struct{}{}
		}
	}
	accounts := make([]TransferAccount, 0, len(memberSet))
	for username := range memberSet {
		if account, ok := parsePasswdEntry(username); ok {
			accounts = append(accounts, account)
		}
	}
	return accounts, nil
}

func createRuntimeSFTPAccount(username, password, homeDir string) error {
	group := sftpGroupName()
	if err := ensureSystemGroup(group); err != nil {
		return err
	}
	if systemUserExists(username) {
		return fmt.Errorf("system user already exists")
	}
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		return err
	}
	args := []string{"-m", "-d", homeDir, "-s", sftpShellPath(), "-g", group, username}
	if _, err := commandOutputTrimmed("useradd", args...); err != nil {
		return err
	}
	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(username + ":" + password + "\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(output)))
	}
	_, _ = commandOutputTrimmed("usermod", "-aG", group, username)
	_, _ = commandOutputTrimmed("chown", "-R", fmt.Sprintf("%s:%s", username, group), homeDir)
	return nil
}

func updateRuntimeSFTPPassword(username, password string) error {
	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(username + ":" + password + "\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(output)))
	}
	return nil
}

func deleteRuntimeSFTPAccount(username string) error {
	_, err := commandOutputTrimmed("userdel", "-r", username)
	return err
}

func runtimeTransferAccounts(kind string) ([]TransferAccount, error) {
	if kind == "sftp" {
		return runtimeSFTPAccounts()
	}
	return pureFTPdList()
}

func createRuntimeTransferAccount(kind, username, password, homeDir, ownerHint string) error {
	if kind == "sftp" {
		return createRuntimeSFTPAccount(username, password, homeDir)
	}
	return createRuntimeFTPAccount(username, password, homeDir, ownerHint)
}

func updateRuntimeTransferPassword(kind, username, password string) error {
	if kind == "sftp" {
		return updateRuntimeSFTPPassword(username, password)
	}
	return updateRuntimeFTPPassword(username, password)
}

func deleteRuntimeTransferAccount(kind, username string) error {
	if kind == "sftp" {
		return deleteRuntimeSFTPAccount(username)
	}
	return deleteRuntimeFTPAccount(username)
}

func mergeTransferMetadata(runtimeItems, existing []TransferAccount) []TransferAccount {
	meta := make(map[string]TransferAccount, len(existing))
	for _, item := range existing {
		meta[item.Username] = item
	}
	for i := range runtimeItems {
		if stored, ok := meta[runtimeItems[i].Username]; ok {
			if runtimeItems[i].Domain == "" {
				runtimeItems[i].Domain = stored.Domain
			}
			if runtimeItems[i].HomeDir == "" {
				runtimeItems[i].HomeDir = stored.HomeDir
			}
			if !runtimeItems[i].Primary {
				runtimeItems[i].Primary = stored.Primary
			}
			if runtimeItems[i].CreatedAt == 0 {
				runtimeItems[i].CreatedAt = stored.CreatedAt
			}
		}
		if runtimeItems[i].CreatedAt == 0 {
			runtimeItems[i].CreatedAt = inferTransferCreatedAt(runtimeItems[i].HomeDir)
		}
	}
	return runtimeItems
}

func inferTransferCreatedAt(homeDir string) int64 {
	if info, err := os.Stat(homeDir); err == nil {
		return info.ModTime().UTC().Unix()
	}
	return 0
}
