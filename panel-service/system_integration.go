package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

type firewallRuntimeRule struct {
	Number    int
	IPAddress string
	Block     bool
	Reason    string
}

type systemMailbox struct {
	Address string
	Maildir string
}

func sshKeyManagerAvailable() bool {
	return runtimeHostLinux()
}

func runtimeHostLinux() bool {
	return runtime.GOOS == "linux"
}

func listFirewallRuntimeRules() []FirewallRule {
	snapshot := collectSecuritySnapshot()
	switch snapshot.FirewallManager {
	case "ufw":
		return listUFFirewallRules()
	case "firewalld":
		return listFirewalldRules()
	default:
		return []FirewallRule{}
	}
}

func addFirewallRuntimeRule(rule FirewallRule) error {
	normalizedIP, err := normalizeFirewallIPAddress(rule.IPAddress)
	if err != nil {
		return err
	}
	rule.IPAddress = normalizedIP
	rule.Reason = strings.TrimSpace(rule.Reason)

	snapshot := collectSecuritySnapshot()
	switch snapshot.FirewallManager {
	case "ufw":
		return addUFFirewallRule(rule)
	case "firewalld":
		return addFirewalldRule(rule)
	default:
		return fmt.Errorf("no supported active firewall manager detected")
	}
}

func openFirewallPort(port int) error {
	snapshot := collectSecuritySnapshot()
	switch snapshot.FirewallManager {
	case "ufw":
		return exec.Command("ufw", "allow", fmt.Sprintf("%d/tcp", port)).Run()
	case "firewalld":
		if err := exec.Command("firewall-cmd", "--permanent", "--add-port", fmt.Sprintf("%d/tcp", port)).Run(); err != nil {
			return err
		}
		return exec.Command("firewall-cmd", "--reload").Run()
	default:
		return fmt.Errorf("no supported active firewall manager detected")
	}
}

func deleteFirewallRuntimeRule(ipAddress string) error {
	snapshot := collectSecuritySnapshot()
	switch snapshot.FirewallManager {
	case "ufw":
		return deleteUFFirewallRule(ipAddress)
	case "firewalld":
		return deleteFirewalldRule(ipAddress)
	default:
		return fmt.Errorf("no supported active firewall manager detected")
	}
}

func listUFFirewallRules() []FirewallRule {
	cmd := exec.Command("ufw", "status", "numbered")
	output, err := cmd.Output()
	if err != nil {
		return []FirewallRule{}
	}

	rules := []FirewallRule{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		runtimeRule, ok := parseUFWNumberedRule(scanner.Text())
		if !ok {
			continue
		}
		rules = append(rules, FirewallRule{
			IPAddress: runtimeRule.IPAddress,
			Block:     runtimeRule.Block,
			Reason:    runtimeRule.Reason,
		})
	}
	return rules
}

func parseUFWNumberedRule(line string) (firewallRuntimeRule, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") {
		return firewallRuntimeRule{}, false
	}

	endIdx := strings.Index(trimmed, "]")
	if endIdx <= 1 {
		return firewallRuntimeRule{}, false
	}

	number, err := strconv.Atoi(strings.TrimSpace(trimmed[1:endIdx]))
	if err != nil {
		return firewallRuntimeRule{}, false
	}

	comment := ""
	body := strings.TrimSpace(trimmed[endIdx+1:])
	if idx := strings.Index(body, "#"); idx >= 0 {
		comment = strings.TrimSpace(body[idx+1:])
		body = strings.TrimSpace(body[:idx])
	}

	fields := strings.Fields(body)
	if len(fields) < 4 {
		return firewallRuntimeRule{}, false
	}

	actionField := strings.ToUpper(strings.TrimSpace(fields[1]))
	if actionField != "ALLOW" && actionField != "DENY" && actionField != "REJECT" {
		return firewallRuntimeRule{}, false
	}

	from := strings.TrimSpace(fields[len(fields)-1])
	if !looksLikeIPAddress(from) {
		return firewallRuntimeRule{}, false
	}

	return firewallRuntimeRule{
		Number:    number,
		IPAddress: from,
		Block:     actionField == "DENY" || actionField == "REJECT",
		Reason:    comment,
	}, true
}

func addUFFirewallRule(rule FirewallRule) error {
	action := "allow"
	if rule.Block {
		action = "deny"
	}

	args := []string{"--force", "insert", "1", action, "from", strings.TrimSpace(rule.IPAddress)}
	if reason := strings.TrimSpace(rule.Reason); reason != "" {
		args = append(args, "comment", truncateShellComment(reason, 60))
	}
	return exec.Command("ufw", args...).Run()
}

func deleteUFFirewallRule(ipAddress string) error {
	cmd := exec.Command("ufw", "status", "numbered")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	numbers := []int{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		rule, ok := parseUFWNumberedRule(scanner.Text())
		if ok && strings.EqualFold(rule.IPAddress, strings.TrimSpace(ipAddress)) {
			numbers = append(numbers, rule.Number)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(numbers)))
	if len(numbers) == 0 {
		return fmt.Errorf("firewall rule not found")
	}

	for _, number := range numbers {
		if err := exec.Command("ufw", "--force", "delete", strconv.Itoa(number)).Run(); err != nil {
			return err
		}
	}
	return nil
}

func listFirewalldRules() []FirewallRule {
	cmd := exec.Command("firewall-cmd", "--permanent", "--list-rich-rules")
	output, err := cmd.Output()
	if err != nil {
		return []FirewallRule{}
	}

	rules := []FirewallRule{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, "source address=") {
			continue
		}
		ipAddress := extractBetween(line, `source address="`, `"`)
		if !looksLikeIPAddress(ipAddress) {
			continue
		}
		rules = append(rules, FirewallRule{
			IPAddress: ipAddress,
			Block:     strings.Contains(line, " drop") || strings.Contains(line, " reject"),
			Reason:    "",
		})
	}
	return rules
}

func addFirewalldRule(rule FirewallRule) error {
	action := "accept"
	if rule.Block {
		action = "drop"
	}
	richRule := fmt.Sprintf(`rule family="ipv4" source address="%s" %s`, strings.TrimSpace(rule.IPAddress), action)
	if err := exec.Command("firewall-cmd", "--permanent", "--add-rich-rule", richRule).Run(); err != nil {
		return err
	}
	return exec.Command("firewall-cmd", "--reload").Run()
}

func deleteFirewalldRule(ipAddress string) error {
	for _, action := range []string{"accept", "drop"} {
		richRule := fmt.Sprintf(`rule family="ipv4" source address="%s" %s`, strings.TrimSpace(ipAddress), action)
		_ = exec.Command("firewall-cmd", "--permanent", "--remove-rich-rule", richRule).Run()
	}
	return exec.Command("firewall-cmd", "--reload").Run()
}

func looksLikeIPAddress(value string) bool {
	if value == "" || strings.EqualFold(value, "Anywhere") {
		return false
	}
	_, err := normalizeFirewallIPAddress(value)
	return err == nil
}

func normalizeFirewallIPAddress(value string) (string, error) {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return "", fmt.Errorf("IP address is required.")
	}

	if strings.Contains(candidate, "/") {
		prefix, err := netip.ParsePrefix(candidate)
		if err != nil || !prefix.Addr().Is4() {
			return "", fmt.Errorf("Invalid IP/CIDR format. Use IPv4 like 203.0.113.10 or 203.0.113.0/24.")
		}
		return prefix.Masked().String(), nil
	}

	addr, err := netip.ParseAddr(candidate)
	if err != nil || !addr.Is4() {
		return "", fmt.Errorf("Invalid IP format. Use IPv4 like 203.0.113.10.")
	}
	return addr.String(), nil
}

func extractBetween(value, prefix, suffix string) string {
	start := strings.Index(value, prefix)
	if start < 0 {
		return ""
	}
	start += len(prefix)
	end := strings.Index(value[start:], suffix)
	if end < 0 {
		return ""
	}
	return value[start : start+end]
}

func truncateShellComment(value string, maxLen int) string {
	cleaned := strings.TrimSpace(strings.ReplaceAll(value, `"`, ""))
	if maxLen <= 0 || len(cleaned) <= maxLen {
		return cleaned
	}
	return cleaned[:maxLen]
}

func listAuthorizedKeys(userName string) []SSHKey {
	account, err := user.Lookup(strings.TrimSpace(userName))
	if err != nil {
		return []SSHKey{}
	}
	authorizedKeysPath := filepath.Join(account.HomeDir, ".ssh", "authorized_keys")
	raw, err := os.ReadFile(authorizedKeysPath)
	if err != nil {
		return []SSHKey{}
	}

	keys := []SSHKey{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		title := sshKeyComment(line)
		keys = append(keys, SSHKey{
			ID:        stableKeyID(line),
			User:      userName,
			Title:     firstNonEmpty(title, "Imported key"),
			PublicKey: line,
		})
	}
	return keys
}

func addAuthorizedKey(userName, title, publicKey string) (SSHKey, error) {
	account, err := user.Lookup(strings.TrimSpace(userName))
	if err != nil {
		return SSHKey{}, err
	}
	if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey)); err != nil {
		return SSHKey{}, fmt.Errorf("invalid SSH public key")
	}

	sshDir := filepath.Join(account.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return SSHKey{}, err
	}
	uid, _ := strconv.Atoi(account.Uid)
	gid, _ := strconv.Atoi(account.Gid)
	_ = os.Chown(sshDir, uid, gid)

	authorizedKeysPath := filepath.Join(sshDir, "authorized_keys")
	line := strings.TrimSpace(publicKey)
	existing := listAuthorizedKeys(userName)
	for _, item := range existing {
		if strings.TrimSpace(item.PublicKey) == line {
			return item, nil
		}
	}

	fh, err := os.OpenFile(authorizedKeysPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return SSHKey{}, err
	}
	defer fh.Close()
	if _, err := fh.WriteString(line + "\n"); err != nil {
		return SSHKey{}, err
	}
	_ = os.Chown(authorizedKeysPath, uid, gid)

	return SSHKey{
		ID:        stableKeyID(line),
		User:      userName,
		Title:     firstNonEmpty(strings.TrimSpace(title), sshKeyComment(line), "Imported key"),
		PublicKey: line,
	}, nil
}

func deleteAuthorizedKey(userName, keyID string) error {
	account, err := user.Lookup(strings.TrimSpace(userName))
	if err != nil {
		return err
	}
	authorizedKeysPath := filepath.Join(account.HomeDir, ".ssh", "authorized_keys")
	raw, err := os.ReadFile(authorizedKeysPath)
	if err != nil {
		return err
	}

	lines := []string{}
	deleted := false
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if stableKeyID(line) == keyID {
			deleted = true
			continue
		}
		lines = append(lines, line)
	}
	if !deleted {
		return fmt.Errorf("SSH key not found")
	}

	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	return os.WriteFile(authorizedKeysPath, []byte(content), 0600)
}

func stableKeyID(line string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(line)))
	return hex.EncodeToString(sum[:8])
}

func sshKeyComment(line string) string {
	fields := strings.Fields(line)
	if len(fields) >= 3 {
		return strings.Join(fields[2:], " ")
	}
	return ""
}

func mailProvisioningAvailable() bool {
	return strings.EqualFold(mailBackendMode(), "vmail")
}

func mailBackendMode() string {
	return strings.ToLower(strings.TrimSpace(envOr("AURAPANEL_MAIL_BACKEND", "vmail")))
}

func vmailUsersFilePath() string {
	return envOr("AURAPANEL_MAIL_USERS_FILE", "/etc/dovecot/users")
}

func postfixVmailboxPath() string {
	return envOr("AURAPANEL_POSTFIX_VMAILBOX_FILE", "/etc/postfix/vmailbox")
}

func postfixVirtualPath() string {
	return envOr("AURAPANEL_POSTFIX_VIRTUAL_FILE", "/etc/postfix/virtual")
}

func postfixVirtualRegexpPath() string {
	return envOr("AURAPANEL_POSTFIX_VIRTUAL_REGEXP_FILE", "/etc/postfix/virtual_regexp")
}

func postfixVmailboxDomainsPath() string {
	return envOr("AURAPANEL_POSTFIX_VMAILBOX_DOMAINS_FILE", "/etc/postfix/vmailbox_domains")
}

func dovecotMasterUsersFilePath() string {
	return envOr("AURAPANEL_MAIL_MASTER_USERS_FILE", "/etc/dovecot/master-users")
}

func mailVmailBaseDir() string {
	return envOr("AURAPANEL_MAIL_VMAIL_BASE", "/var/mail/vhosts")
}

func mailVmailUID() int {
	value, _ := strconv.Atoi(envOr("AURAPANEL_MAIL_VMAIL_UID", "5000"))
	if value <= 0 {
		return 5000
	}
	return value
}

func mailVmailGID() int {
	value, _ := strconv.Atoi(envOr("AURAPANEL_MAIL_VMAIL_GID", "5000"))
	if value <= 0 {
		return 5000
	}
	return value
}

func ensureDovecotPasswdAuthConfig() error {
	authPath := "/etc/dovecot/conf.d/10-auth.conf"
	raw, err := os.ReadFile(authPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	content := string(raw)
	if strings.Contains(content, "!include auth-system.conf.ext") {
		content = strings.ReplaceAll(content, "!include auth-system.conf.ext", "#!include auth-system.conf.ext")
	}
	if !strings.Contains(content, "auth-passwdfile.conf.ext") {
		content = strings.TrimRight(content, "\n") + "\n!include auth-passwdfile.conf.ext\n"
	}
	return os.WriteFile(authPath, []byte(content), 0644)
}

func ensureDovecotPostfixAuthSocket() error {
	content := `service auth {
  unix_listener /var/spool/postfix/private/auth {
    mode = 0660
    user = postfix
    group = postfix
  }
}
`
	return os.WriteFile("/etc/dovecot/conf.d/90-aurapanel-auth-socket.conf", []byte(content), 0644)
}

func ensurePostfixSubmissionServices() error {
	const submissionBlock = `submission inet n       -       y       -       -       smtpd
  -o syslog_name=postfix/submission
  -o smtpd_tls_security_level=encrypt
  -o smtpd_sasl_auth_enable=yes
  -o smtpd_recipient_restrictions=permit_sasl_authenticated,reject
  -o milter_macro_daemon_name=ORIGINATING
`
	const smtpsBlock = `smtps     inet n       -       y       -       -       smtpd
  -o syslog_name=postfix/smtps
  -o smtpd_tls_wrappermode=yes
  -o smtpd_sasl_auth_enable=yes
  -o smtpd_recipient_restrictions=permit_sasl_authenticated,reject
  -o milter_macro_daemon_name=ORIGINATING
`

	path := "/etc/postfix/master.cf"
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(raw)
	changed := false

	if !strings.Contains(content, "submission inet") {
		content = strings.TrimRight(content, "\n") + "\n\n" + submissionBlock
		changed = true
	}
	if !strings.Contains(content, "smtps     inet") && !strings.Contains(content, "smtps inet") {
		content = strings.TrimRight(content, "\n") + "\n\n" + smtpsBlock
		changed = true
	}
	if !changed {
		return nil
	}
	return os.WriteFile(path, []byte(strings.TrimRight(content, "\n")+"\n"), 0644)
}

func ensureMailRuntimeBaseline() error {
	if !mailProvisioningAvailable() {
		return nil
	}

	vmailUID := mailVmailUID()
	vmailGID := mailVmailGID()
	vmailBase := mailVmailBaseDir()

	if err := os.MkdirAll(vmailBase, 0750); err != nil {
		return err
	}
	_ = os.Chown(vmailBase, vmailUID, vmailGID)

	for _, path := range []string{"/etc/dovecot", "/etc/dovecot/conf.d", "/etc/postfix"} {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}

	for _, path := range []string{vmailUsersFilePath(), dovecotMasterUsersFilePath(), postfixVmailboxPath(), postfixVmailboxDomainsPath(), postfixVirtualPath(), postfixVirtualRegexpPath()} {
		file, err := os.OpenFile(path, os.O_CREATE, 0640)
		if err != nil {
			return err
		}
		_ = file.Close()
	}
	if err := ensureDovecotPasswdAuthConfig(); err != nil {
		return err
	}
	if err := ensureDovecotPostfixAuthSocket(); err != nil {
		return err
	}
	if err := ensurePostfixSubmissionServices(); err != nil {
		return err
	}

	dovecotConf := fmt.Sprintf(`auth_master_user_separator = *

passdb {
  driver = passwd-file
  args = %s
  master = yes
  pass = yes
}

passdb {
  driver = passwd-file
  args = scheme=SHA512-CRYPT username_format=%%u %s
}

mail_location = maildir:%s/%%d/%%n/Maildir

userdb {
  driver = static
  args = uid=%d gid=%d home=%s/%%d/%%n allow_all_users=yes
}
`, dovecotMasterUsersFilePath(), vmailUsersFilePath(), vmailBase, vmailUID, vmailGID, vmailBase)
	if err := os.WriteFile("/etc/dovecot/conf.d/90-aurapanel-vmail.conf", []byte(dovecotConf), 0644); err != nil {
		return err
	}

	postfixSettings := []string{
		fmt.Sprintf("virtual_mailbox_base=%s", vmailBase),
		fmt.Sprintf("virtual_mailbox_domains=hash:%s", postfixVmailboxDomainsPath()),
		fmt.Sprintf("virtual_mailbox_maps=hash:%s", postfixVmailboxPath()),
		fmt.Sprintf("virtual_alias_maps=hash:%s,regexp:%s", postfixVirtualPath(), postfixVirtualRegexpPath()),
		fmt.Sprintf("virtual_minimum_uid=%d", vmailUID),
		fmt.Sprintf("virtual_uid_maps=static:%d", vmailUID),
		fmt.Sprintf("virtual_gid_maps=static:%d", vmailGID),
		"smtpd_sasl_type=dovecot",
		"smtpd_sasl_path=private/auth",
		"smtpd_sasl_auth_enable=yes",
		"smtpd_tls_security_level=may",
		"smtpd_tls_auth_only=no",
		"smtpd_recipient_restrictions=permit_mynetworks,permit_sasl_authenticated,reject_unauth_destination",
	}
	for _, setting := range postfixSettings {
		if err := exec.Command("postconf", "-e", setting).Run(); err != nil {
			return err
		}
	}

	return nil
}

func provisionMailDomain(domain string) error {
	if !mailProvisioningAvailable() {
		return nil
	}

	normalizedDomain := normalizeDomain(domain)
	if normalizedDomain == "" {
		return fmt.Errorf("domain is required")
	}
	baseDir := filepath.Join(mailVmailBaseDir(), normalizedDomain)
	if err := os.MkdirAll(baseDir, 0750); err != nil {
		return err
	}
	_ = os.Chown(baseDir, mailVmailUID(), mailVmailGID())
	return upsertSimpleMapLine(postfixVmailboxDomainsPath(), normalizedDomain, normalizedDomain)
}

func loadSystemMailboxes(defaultQuotas map[string]int) []Mailbox {
	items := parseSimpleMapFile(postfixVmailboxPath())
	out := make([]Mailbox, 0, len(items))
	for address, maildir := range items {
		parts := strings.SplitN(address, "@", 2)
		if len(parts) != 2 {
			continue
		}
		quota := defaultQuotas[address]
		if quota <= 0 {
			quota = 1024
		}
		out = append(out, Mailbox{
			Address: address,
			Domain:  parts[1],
			User:    parts[0],
			QuotaMB: quota,
			UsedMB:  0,
			Owner:   "",
		})
		_ = maildir
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Address < out[j].Address })
	return out
}

func upsertSystemMailbox(address, password string) error {
	if !mailProvisioningAvailable() {
		return nil
	}
	address = strings.ToLower(strings.TrimSpace(address))
	parts := strings.SplitN(address, "@", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid mailbox address")
	}
	domain := normalizeDomain(parts[1])
	username := sanitizeName(parts[0])
	if username == "" || domain == "" {
		return fmt.Errorf("invalid mailbox address")
	}
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("mailbox password is required")
	}

	if err := provisionMailDomain(domain); err != nil {
		return err
	}

	maildir := filepath.Join(mailVmailBaseDir(), domain, username)
	for _, dirName := range []string{"", "Maildir", filepath.Join("Maildir", "cur"), filepath.Join("Maildir", "new"), filepath.Join("Maildir", "tmp")} {
		target := filepath.Join(maildir, dirName)
		if err := os.MkdirAll(target, 0750); err != nil {
			return err
		}
		_ = os.Chown(target, mailVmailUID(), mailVmailGID())
	}

	hashed, err := hashMailPassword(password)
	if err != nil {
		return err
	}

	if err := upsertPasswdFileLine(vmailUsersFilePath(), address, "{SHA512-CRYPT}"+hashed); err != nil {
		return err
	}
	if err := upsertSimpleMapLine(postfixVmailboxPath(), address, fmt.Sprintf("%s/%s/Maildir/", domain, username)); err != nil {
		return err
	}

	return reloadMailRuntime()
}

func deleteSystemMailbox(address string) error {
	if !mailProvisioningAvailable() {
		return nil
	}
	address = strings.ToLower(strings.TrimSpace(address))
	if err := deletePasswdFileLine(vmailUsersFilePath(), address); err != nil {
		return err
	}
	if err := deleteSimpleMapLine(postfixVmailboxPath(), address); err != nil {
		return err
	}
	return reloadMailRuntime()
}

func updateSystemMailboxPassword(address, newPassword string) error {
	if !mailProvisioningAvailable() {
		return nil
	}
	address = strings.ToLower(strings.TrimSpace(address))
	if address == "" || strings.TrimSpace(newPassword) == "" {
		return fmt.Errorf("address and password are required")
	}
	hashed, err := hashMailPassword(newPassword)
	if err != nil {
		return err
	}
	if err := upsertPasswdFileLine(vmailUsersFilePath(), address, "{SHA512-CRYPT}"+hashed); err != nil {
		return err
	}
	return reloadMailRuntime()
}

func upsertSystemForward(domain, source, target string) error {
	key := strings.ToLower(strings.TrimSpace(source)) + "@" + normalizeDomain(domain)
	if err := upsertSimpleMapLine(postfixVirtualPath(), key, strings.TrimSpace(target)); err != nil {
		return err
	}
	return reloadMailRuntime()
}

func deleteSystemForward(domain, source string) error {
	key := strings.ToLower(strings.TrimSpace(source)) + "@" + normalizeDomain(domain)
	if err := deleteSimpleMapLine(postfixVirtualPath(), key); err != nil {
		return err
	}
	return reloadMailRuntime()
}

func setSystemCatchAll(domain, target string, enabled bool) error {
	normalizedDomain := normalizeDomain(domain)
	if normalizedDomain == "" {
		return fmt.Errorf("domain is required")
	}
	pattern := fmt.Sprintf("/^(.+)@%s$/", regexp.QuoteMeta(normalizedDomain))
	if !enabled || strings.TrimSpace(target) == "" {
		if err := deleteSimpleMapLine(postfixVirtualRegexpPath(), pattern); err != nil {
			return err
		}
		return reloadMailRuntime()
	}
	if err := upsertSimpleMapLine(postfixVirtualRegexpPath(), pattern, strings.TrimSpace(target)); err != nil {
		return err
	}
	return reloadMailRuntime()
}

func reloadMailRuntime() error {
	if err := ensureMailRuntimeBaseline(); err != nil {
		return err
	}
	if err := normalizeVmailboxMaildirMappings(); err != nil {
		return err
	}
	for _, mapPath := range []string{postfixVmailboxDomainsPath(), postfixVmailboxPath(), postfixVirtualPath()} {
		_ = exec.Command("postmap", mapPath).Run()
		ensurePostfixMapReadable(mapPath + ".db")
	}
	for _, unit := range []string{"postfix", "dovecot"} {
		_ = exec.Command("systemctl", "restart", unit).Run()
	}
	return nil
}

func normalizeVmailboxMaildirMappings() error {
	items := parseSimpleMapFile(postfixVmailboxPath())
	changed := false
	for key, value := range items {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || strings.Contains(trimmed, " ") {
			continue
		}
		for strings.Contains(trimmed, "/Maildir/Maildir/") {
			trimmed = strings.ReplaceAll(trimmed, "/Maildir/Maildir/", "/Maildir/")
			items[key] = trimmed
			changed = true
		}
		if strings.HasSuffix(trimmed, "/Maildir/") {
			continue
		}
		if strings.HasSuffix(trimmed, "/") {
			items[key] = trimmed + "Maildir/"
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return writeSimpleMapFile(postfixVmailboxPath(), items)
}

func ensurePostfixMapReadable(path string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}
	_ = os.Chmod(path, 0644)
	if grp, lookupErr := user.LookupGroup("postfix"); lookupErr == nil {
		if gid, convErr := strconv.Atoi(grp.Gid); convErr == nil {
			_ = os.Chown(path, -1, gid)
		}
	}
}

func hashMailPassword(password string) (string, error) {
	if err := exec.Command("openssl", "version").Run(); err != nil {
		return "", err
	}
	cmd := exec.Command("openssl", "passwd", "-6", strings.TrimSpace(password))
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func parseSimpleMapFile(path string) map[string]string {
	items := map[string]string{}
	raw, err := os.ReadFile(path)
	if err != nil {
		return items
	}

	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := fields[0]
		value := strings.TrimSpace(strings.Join(fields[1:], " "))
		items[key] = value
	}
	return items
}

func parsePasswdFile(path string) map[string]string {
	items := map[string]string{}
	raw, err := os.ReadFile(path)
	if err != nil {
		return items
	}

	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		items[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return items
}

func upsertSimpleMapLine(path, key, value string) error {
	items := parseSimpleMapFile(path)
	items[key] = value
	return writeSimpleMapFile(path, items)
}

func upsertPasswdFileLine(path, key, value string) error {
	items := parsePasswdFile(path)
	items[key] = value
	return writePasswdFile(path, items)
}

func deleteSimpleMapLine(path, key string) error {
	items := parseSimpleMapFile(path)
	delete(items, key)
	return writeSimpleMapFile(path, items)
}

func deletePasswdFileLine(path, key string) error {
	items := parsePasswdFile(path)
	delete(items, key)
	return writePasswdFile(path, items)
}

func writeSimpleMapFile(path string, items map[string]string) error {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte(' ')
		builder.WriteString(items[key])
		builder.WriteByte('\n')
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(builder.String()), 0640)
}

func writePasswdFile(path string, items map[string]string) error {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte(':')
		builder.WriteString(items[key])
		builder.WriteByte('\n')
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(builder.String()), 0640)
}
