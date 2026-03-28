use bcrypt::{hash as bcrypt_hash, DEFAULT_COST};
use getrandom::getrandom;
use serde::{Deserialize, Serialize};
use std::collections::hash_map::DefaultHasher;
use std::fs;
use std::hash::{Hash, Hasher};
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MailboxConfig {
    pub domain: String,
    pub username: String,
    pub password: String,
    pub quota_mb: u32,
    #[serde(default)]
    pub owner: Option<String>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct LocalMailbox {
    pub id: u32,
    pub address: String,
    pub domain: String,
    pub quota_mb: u32,
    pub used_mb: u32,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MailForwardConfig {
    pub domain: String,
    pub source: String,
    pub target: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MailForwardRule {
    pub domain: String,
    pub source: String,
    pub target: String,
    pub created_at: u64,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MailCatchAllConfig {
    pub domain: String,
    pub enabled: bool,
    pub target: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MailCatchAllRule {
    pub domain: String,
    pub enabled: bool,
    pub target: String,
    pub updated_at: u64,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MailRoutingConfig {
    pub domain: String,
    pub pattern: String,
    pub target: String,
    pub priority: u32,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MailRoutingRule {
    pub id: String,
    pub domain: String,
    pub pattern: String,
    pub target: String,
    pub priority: u32,
    pub created_at: u64,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MailDkimRecord {
    pub domain: String,
    pub selector: String,
    pub public_key: String,
    pub private_key: String,
    pub updated_at: u64,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MailWebmailSsoRequest {
    pub address: String,
    pub ttl_seconds: Option<u64>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MailWebmailSsoConsumeRequest {
    pub token: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MailboxPasswordResetRequest {
    pub address: String,
    pub new_password: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MailWebmailSsoLink {
    pub url: String,
    pub expires_at: u64,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MailWebmailSsoSession {
    pub address: String,
    pub password: String,
    pub expires_at: u64,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MailForwardDeleteRequest {
    pub domain: String,
    pub source: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MailRoutingDeleteRequest {
    pub domain: String,
    pub id: String,
}

#[derive(Serialize, Deserialize, Debug, Clone, Default)]
struct MailState {
    #[serde(default)]
    mailboxes: Vec<LocalMailbox>,
    #[serde(default)]
    mailbox_secrets: Vec<MailboxSecretRecord>,
    #[serde(default)]
    forwards: Vec<MailForwardRule>,
    #[serde(default)]
    catch_all: Vec<MailCatchAllRule>,
    #[serde(default)]
    routing: Vec<MailRoutingRule>,
    #[serde(default)]
    dkim: Vec<MailDkimRecord>,
    #[serde(default)]
    sso_tokens: Vec<MailWebmailSsoTokenRecord>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
struct MailboxSecretRecord {
    address: String,
    password_cipher: String,
    updated_at: u64,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
struct MailWebmailSsoTokenRecord {
    token: String,
    address: String,
    expires_at: u64,
    created_at: u64,
}

pub struct MailManager;

impl MailManager {
    fn backend() -> String {
        std::env::var("AURAPANEL_MAIL_BACKEND")
            .unwrap_or_else(|_| "local".to_string())
            .trim()
            .to_ascii_lowercase()
    }

    fn now_ts() -> u64 {
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|d| d.as_secs())
            .unwrap_or(0)
    }

    fn sso_secret() -> Vec<u8> {
        std::env::var("AURAPANEL_JWT_SECRET")
            .unwrap_or_else(|_| "aurapanel-mail-sso-secret".to_string())
            .into_bytes()
    }

    fn hex_encode(bytes: &[u8]) -> String {
        bytes.iter().map(|b| format!("{:02x}", b)).collect()
    }

    fn hex_decode(value: &str) -> Result<Vec<u8>, String> {
        let trimmed = value.trim();
        if trimmed.is_empty() {
            return Ok(Vec::new());
        }
        if trimmed.len() % 2 != 0 {
            return Err("Hex decode icin gecersiz uzunluk.".to_string());
        }
        let mut out = Vec::with_capacity(trimmed.len() / 2);
        let bytes = trimmed.as_bytes();
        let mut i = 0;
        while i < bytes.len() {
            let hex = std::str::from_utf8(&bytes[i..i + 2]).map_err(|e| e.to_string())?;
            let val = u8::from_str_radix(hex, 16).map_err(|e| e.to_string())?;
            out.push(val);
            i += 2;
        }
        Ok(out)
    }

    fn encrypt_mailbox_password(password: &str) -> String {
        let input = password.as_bytes();
        if input.is_empty() {
            return String::new();
        }
        let key = Self::sso_secret();
        if key.is_empty() {
            return Self::hex_encode(input);
        }
        let encrypted = input
            .iter()
            .enumerate()
            .map(|(i, b)| b ^ key[i % key.len()])
            .collect::<Vec<u8>>();
        Self::hex_encode(&encrypted)
    }

    fn decrypt_mailbox_password(cipher: &str) -> Result<String, String> {
        let encrypted = Self::hex_decode(cipher)?;
        if encrypted.is_empty() {
            return Ok(String::new());
        }
        let key = Self::sso_secret();
        if key.is_empty() {
            return String::from_utf8(encrypted).map_err(|e| e.to_string());
        }
        let decrypted = encrypted
            .iter()
            .enumerate()
            .map(|(i, b)| b ^ key[i % key.len()])
            .collect::<Vec<u8>>();
        String::from_utf8(decrypted).map_err(|e| e.to_string())
    }

    fn upsert_mailbox_secret(state: &mut MailState, address: &str, password: &str) {
        let now = Self::now_ts();
        let cipher = Self::encrypt_mailbox_password(password);
        if let Some(secret) = state
            .mailbox_secrets
            .iter_mut()
            .find(|x| x.address.eq_ignore_ascii_case(address))
        {
            secret.password_cipher = cipher;
            secret.updated_at = now;
            return;
        }
        state.mailbox_secrets.push(MailboxSecretRecord {
            address: address.to_ascii_lowercase(),
            password_cipher: cipher,
            updated_at: now,
        });
    }

    fn mailbox_password_for_sso(state: &MailState, address: &str) -> Result<String, String> {
        let Some(secret) = state
            .mailbox_secrets
            .iter()
            .find(|x| x.address.eq_ignore_ascii_case(address))
        else {
            return Err(
                "Mailbox SSO hazir degil. Mail sifresini bir kez resetleyip tekrar deneyin."
                    .to_string(),
            );
        };
        let password = Self::decrypt_mailbox_password(&secret.password_cipher)?;
        if password.is_empty() {
            return Err(
                "Mailbox SSO hazir degil. Mail sifresini bir kez resetleyip tekrar deneyin."
                    .to_string(),
            );
        }
        Ok(password)
    }

    fn cleanup_expired_sso_tokens(state: &mut MailState, now: u64) {
        state.sso_tokens.retain(|x| x.expires_at > now);
    }

    fn generate_sso_token() -> Result<String, String> {
        let mut bytes = [0_u8; 24];
        getrandom(&mut bytes).map_err(|e| format!("SSO token uretilemedi: {}", e))?;
        Ok(Self::hex_encode(&bytes))
    }

    fn ensure_roundcube_sso_bridge() -> Result<(), String> {
        if cfg!(windows) {
            return Ok(());
        }
        let bridge_dir = std::env::var("AURAPANEL_WEBMAIL_SSO_BRIDGE_DIR")
            .unwrap_or_else(|_| "/usr/local/lsws/Example/html/webmail/sso".to_string());
        let bridge_file = PathBuf::from(bridge_dir.trim()).join("index.php");
        if let Some(parent) = bridge_file.parent() {
            fs::create_dir_all(parent).map_err(|e| e.to_string())?;
        }

        let script = r#"<?php
declare(strict_types=1);

function aura_sso_fail(int $code, string $message): void {
    http_response_code($code);
    header('Content-Type: text/plain; charset=utf-8');
    echo $message;
    exit;
}

$token = trim((string)($_GET['token'] ?? ''));
if ($token === '') {
    aura_sso_fail(400, 'Missing token.');
}

$consumeUrl = getenv('AURAPANEL_WEBMAIL_SSO_CONSUME_URL');
if (!$consumeUrl) {
    $consumeUrl = 'http://127.0.0.1:8090/api/v1/mail/webmail/sso/consume';
}

$payload = json_encode(['token' => $token], JSON_UNESCAPED_SLASHES);
$ch = curl_init($consumeUrl);
curl_setopt_array($ch, [
    CURLOPT_POST => true,
    CURLOPT_HTTPHEADER => ['Content-Type: application/json'],
    CURLOPT_POSTFIELDS => $payload,
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_TIMEOUT => 10,
]);
$raw = curl_exec($ch);
if ($raw === false) {
    aura_sso_fail(502, 'SSO consume request failed.');
}
$status = curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);
if ($status < 200 || $status >= 300) {
    aura_sso_fail(401, 'SSO token invalid or expired.');
}

$decoded = json_decode($raw, true);
if (!is_array($decoded) || ($decoded['status'] ?? '') !== 'success') {
    aura_sso_fail(401, 'SSO token invalid or expired.');
}

$address = (string)($decoded['data']['address'] ?? '');
$password = (string)($decoded['data']['password'] ?? '');
if ($address === '' || $password === '') {
    aura_sso_fail(401, 'SSO session could not be created.');
}

$hostHeader = trim((string)($_SERVER['HTTP_HOST'] ?? ''));
if ($hostHeader === '') {
    $hostHeader = 'localhost';
}
$requestHeaders = ['Host: ' . $hostHeader];
$base = 'http://127.0.0.1/webmail';
$cookieFile = tempnam(sys_get_temp_dir(), 'rcsso_');
$loginUrl = $base . '/?_task=login';

$ch = curl_init($loginUrl);
curl_setopt_array($ch, [
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_COOKIEJAR => $cookieFile,
    CURLOPT_COOKIEFILE => $cookieFile,
    CURLOPT_HTTPHEADER => $requestHeaders,
    CURLOPT_TIMEOUT => 10,
]);
$loginPage = curl_exec($ch);
curl_close($ch);
if (!is_string($loginPage) || $loginPage === '') {
    @unlink($cookieFile);
    aura_sso_fail(502, 'Roundcube login page could not be loaded.');
}

$tokenField = '';
if (preg_match('/name=["\']_token["\']\s+value=["\']([^"\']+)/', $loginPage, $m)) {
    $tokenField = $m[1];
}

$postFields = [
    '_task' => 'login',
    '_action' => 'login',
    '_user' => $address,
    '_pass' => $password,
];
if ($tokenField !== '') {
    $postFields['_token'] = $tokenField;
}

$ch = curl_init($loginUrl);
curl_setopt_array($ch, [
    CURLOPT_POST => true,
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_POSTFIELDS => http_build_query($postFields),
    CURLOPT_HTTPHEADER => array_merge(
        $requestHeaders,
        ['Content-Type: application/x-www-form-urlencoded']
    ),
    CURLOPT_COOKIEJAR => $cookieFile,
    CURLOPT_COOKIEFILE => $cookieFile,
    CURLOPT_TIMEOUT => 10,
    CURLOPT_HEADER => true,
]);
$loginRaw = curl_exec($ch);
$loginCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);
if ($loginRaw === false || ($loginCode < 200 || $loginCode >= 400)) {
    @unlink($cookieFile);
    aura_sso_fail(401, 'Roundcube login failed.');
}

$cookies = @file($cookieFile, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES) ?: [];
$https = !empty($_SERVER['HTTPS']) && $_SERVER['HTTPS'] !== 'off';
foreach ($cookies as $line) {
    if ($line === '' || $line[0] === '#') {
        continue;
    }
    $parts = explode("\t", $line);
    if (count($parts) < 7) {
        continue;
    }
    $path = $parts[2] !== '' ? $parts[2] : '/webmail';
    $secure = strtoupper($parts[3]) === 'TRUE';
    $name = $parts[5];
    $value = $parts[6];
    if ($name === '') {
        continue;
    }
    setcookie($name, $value, [
        'expires' => 0,
        'path' => $path,
        'secure' => ($secure || $https),
        'httponly' => true,
        'samesite' => 'Lax',
    ]);
}
@unlink($cookieFile);

header('Location: ../?_task=mail');
exit;
"#;

        fs::write(&bridge_file, script).map_err(|e| e.to_string())?;
        Ok(())
    }

    fn state_root() -> PathBuf {
        if let Ok(path) = std::env::var("AURAPANEL_STATE_DIR") {
            let p = PathBuf::from(path.trim());
            if !p.as_os_str().is_empty() {
                return p;
            }
        }

        let prod = Path::new("/var/lib/aurapanel");
        if prod.exists() {
            prod.to_path_buf()
        } else {
            std::env::temp_dir().join("aurapanel")
        }
    }

    fn storage_path() -> PathBuf {
        Self::state_root().join("mail_state.json")
    }

    fn legacy_mailboxes_path() -> PathBuf {
        Self::state_root().join("mailboxes.json")
    }

    fn ensure_parent(path: &Path) -> Result<(), String> {
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).map_err(|e| e.to_string())?;
        }
        Ok(())
    }

    fn normalize_domain(domain: &str) -> String {
        domain.trim().trim_end_matches('.').to_ascii_lowercase()
    }

    fn sanitize_local_part(value: &str) -> String {
        value
            .trim()
            .to_ascii_lowercase()
            .chars()
            .filter(|c| c.is_ascii_alphanumeric() || *c == '.' || *c == '_' || *c == '-')
            .collect()
    }

    fn normalize_address(domain: &str, source: &str) -> String {
        let source = source.trim();
        if source.contains('@') {
            source.to_ascii_lowercase()
        } else {
            format!(
                "{}@{}",
                Self::sanitize_local_part(source),
                Self::normalize_domain(domain)
            )
        }
    }

    fn load_state() -> Result<MailState, String> {
        let path = Self::storage_path();
        if path.exists() {
            let raw = fs::read_to_string(&path).map_err(|e| e.to_string())?;
            return serde_json::from_str::<MailState>(&raw).map_err(|e| e.to_string());
        }

        // Legacy migration: old mailbox-only file
        let legacy = Self::legacy_mailboxes_path();
        if legacy.exists() {
            let raw = fs::read_to_string(&legacy).map_err(|e| e.to_string())?;
            let mailboxes = serde_json::from_str::<Vec<LocalMailbox>>(&raw).unwrap_or_default();
            let state = MailState {
                mailboxes,
                ..MailState::default()
            };
            Self::save_state(&state)?;
            return Ok(state);
        }

        Ok(MailState::default())
    }

    fn save_state(state: &MailState) -> Result<(), String> {
        let path = Self::storage_path();
        Self::ensure_parent(&path)?;
        let payload = serde_json::to_string_pretty(state).map_err(|e| e.to_string())?;
        fs::write(path, payload).map_err(|e| e.to_string())
    }

    fn backend_is_vmail() -> bool {
        matches!(
            Self::backend().as_str(),
            "vmail" | "postfix-dovecot" | "postfix_dovecot"
        )
    }

    fn dovecot_users_file() -> PathBuf {
        PathBuf::from(
            std::env::var("AURAPANEL_MAIL_DOVECOT_USERS_FILE")
                .unwrap_or_else(|_| "/etc/dovecot/users".to_string()),
        )
    }

    fn postfix_mailbox_map_file() -> PathBuf {
        PathBuf::from(
            std::env::var("AURAPANEL_MAIL_POSTFIX_MAILBOX_MAP")
                .unwrap_or_else(|_| "/etc/postfix/vmailbox".to_string()),
        )
    }

    fn postfix_alias_map_file() -> PathBuf {
        PathBuf::from(
            std::env::var("AURAPANEL_MAIL_POSTFIX_ALIAS_MAP")
                .unwrap_or_else(|_| "/etc/postfix/virtual".to_string()),
        )
    }

    fn postfix_routing_map_file() -> PathBuf {
        PathBuf::from(
            std::env::var("AURAPANEL_MAIL_POSTFIX_ROUTING_MAP")
                .unwrap_or_else(|_| "/etc/postfix/virtual_regexp".to_string()),
        )
    }

    fn vmail_base_dir() -> PathBuf {
        PathBuf::from(
            std::env::var("AURAPANEL_MAIL_VMAIL_BASE")
                .unwrap_or_else(|_| "/var/mail/vhosts".to_string()),
        )
    }

    fn vmail_uid() -> String {
        std::env::var("AURAPANEL_MAIL_VMAIL_UID")
            .ok()
            .filter(|x| !x.trim().is_empty())
            .unwrap_or_else(|| "5000".to_string())
    }

    fn vmail_gid() -> String {
        std::env::var("AURAPANEL_MAIL_VMAIL_GID")
            .ok()
            .filter(|x| !x.trim().is_empty())
            .unwrap_or_else(|| "5000".to_string())
    }

    fn vmail_quota_mb(config_quota: u32) -> u32 {
        config_quota.clamp(64, 102400)
    }

    fn dovecot_user_record(
        address: &str,
        pass_hash: &str,
        mail_root: &Path,
        quota_mb: u32,
    ) -> String {
        format!(
            "{}:{}:{}:{}::{}::userdb_mail=maildir:{}/Maildir userdb_quota_rule=*:storage={}M",
            address,
            pass_hash,
            Self::vmail_uid(),
            Self::vmail_gid(),
            mail_root.display(),
            mail_root.display(),
            quota_mb
        )
    }

    fn ensure_maildir_permissions(path: &Path) -> Result<(), String> {
        if cfg!(windows) {
            return Err("maildir permission setup is supported only on Linux hosts.".to_string());
        }

        fs::create_dir_all(path).map_err(|e| e.to_string())?;
        let maildir = path.join("Maildir");
        fs::create_dir_all(maildir.join("cur")).map_err(|e| e.to_string())?;
        fs::create_dir_all(maildir.join("new")).map_err(|e| e.to_string())?;
        fs::create_dir_all(maildir.join("tmp")).map_err(|e| e.to_string())?;

        let uid_gid = format!("{}:{}", Self::vmail_uid(), Self::vmail_gid());
        let _ = Command::new("chown")
            .args(["-R", &uid_gid, path.to_string_lossy().as_ref()])
            .output();
        let _ = Command::new("chmod")
            .args(["750", path.to_string_lossy().as_ref()])
            .output();
        let _ = Command::new("chmod")
            .args(["-R", "750", maildir.to_string_lossy().as_ref()])
            .output();
        Ok(())
    }

    fn ensure_file(path: &Path) -> Result<(), String> {
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).map_err(|e| e.to_string())?;
        }
        if !path.exists() {
            fs::write(path, "").map_err(|e| e.to_string())?;
        }
        Ok(())
    }

    fn hash_password(password: &str) -> String {
        if let Ok(output) = Command::new("doveadm")
            .args(["pw", "-s", "SHA512-CRYPT", "-p", password])
            .output()
        {
            if output.status.success() {
                let hashed = String::from_utf8_lossy(&output.stdout).trim().to_string();
                if !hashed.is_empty() {
                    return hashed;
                }
            }
        }

        if let Ok(output) = Command::new("openssl")
            .args(["passwd", "-6", password])
            .output()
        {
            if output.status.success() {
                let hashed = String::from_utf8_lossy(&output.stdout).trim().to_string();
                if !hashed.is_empty() {
                    return hashed;
                }
            }
        }

        match bcrypt_hash(password, DEFAULT_COST) {
            Ok(value) => value,
            Err(_) => format!("{{PLAIN}}{}", password),
        }
    }

    fn maildir_base_path(domain: &str, username: &str) -> PathBuf {
        Self::vmail_base_dir().join(domain).join(username)
    }

    fn username_from_address(address: &str) -> Option<String> {
        let mut parts = address.split('@');
        let user = parts.next()?.trim();
        if user.is_empty() {
            return None;
        }
        Some(Self::sanitize_local_part(user))
    }

    fn read_lines(path: &Path) -> Result<Vec<String>, String> {
        if !path.exists() {
            return Ok(Vec::new());
        }
        let raw = fs::read_to_string(path).map_err(|e| e.to_string())?;
        Ok(raw.lines().map(|x| x.to_string()).collect())
    }

    fn write_lines(path: &Path, lines: &[String]) -> Result<(), String> {
        Self::ensure_file(path)?;
        let content = if lines.is_empty() {
            String::new()
        } else {
            format!("{}\n", lines.join("\n"))
        };
        fs::write(path, content).map_err(|e| e.to_string())
    }

    fn postmap(path: &Path) -> Result<(), String> {
        if cfg!(windows) {
            return Err("postmap is supported only on Linux hosts.".to_string());
        }
        let output = Command::new("postmap")
            .arg(path.as_os_str())
            .output()
            .map_err(|e| format!("postmap calistirilamadi: {}", e))?;
        if output.status.success() {
            Ok(())
        } else {
            Err(String::from_utf8_lossy(&output.stderr).trim().to_string())
        }
    }

    fn reload_service(unit: &str) {
        if cfg!(windows) {
            return;
        }
        let _ = Command::new("systemctl").args(["reload", unit]).output();
        let _ = Command::new("systemctl").args(["restart", unit]).output();
    }

    fn apply_vmail_mailbox_create(
        address: &str,
        domain: &str,
        username: &str,
        password: &str,
        quota_mb: u32,
    ) -> Result<(), String> {
        let dovecot_users = Self::dovecot_users_file();
        let postfix_mailbox = Self::postfix_mailbox_map_file();

        Self::ensure_file(&dovecot_users)?;
        Self::ensure_file(&postfix_mailbox)?;

        let base = Self::maildir_base_path(domain, username);
        Self::ensure_maildir_permissions(&base)?;

        let mut dovecot_lines = Self::read_lines(&dovecot_users)?;
        dovecot_lines.retain(|line| !line.starts_with(&format!("{}:", address)));
        let pass_hash = Self::hash_password(password);
        let mail_root = Self::maildir_base_path(domain, username);
        dovecot_lines.push(Self::dovecot_user_record(
            address,
            &pass_hash,
            &mail_root,
            Self::vmail_quota_mb(quota_mb),
        ));
        Self::write_lines(&dovecot_users, &dovecot_lines)?;

        let mut mailbox_lines = Self::read_lines(&postfix_mailbox)?;
        mailbox_lines.retain(|line| !line.starts_with(&format!("{} ", address)));
        mailbox_lines.push(format!("{} {}/{}/Maildir/", address, domain, username));
        Self::write_lines(&postfix_mailbox, &mailbox_lines)?;
        Self::postmap(&postfix_mailbox)?;

        Self::reload_service("dovecot");
        Self::reload_service("postfix");
        Ok(())
    }

    fn apply_vmail_mailbox_delete(address: &str) -> Result<(), String> {
        let dovecot_users = Self::dovecot_users_file();
        let postfix_mailbox = Self::postfix_mailbox_map_file();

        if dovecot_users.exists() {
            let mut dovecot_lines = Self::read_lines(&dovecot_users)?;
            dovecot_lines.retain(|line| !line.starts_with(&format!("{}:", address)));
            Self::write_lines(&dovecot_users, &dovecot_lines)?;
        }

        if postfix_mailbox.exists() {
            let mut mailbox_lines = Self::read_lines(&postfix_mailbox)?;
            mailbox_lines.retain(|line| !line.starts_with(&format!("{} ", address)));
            Self::write_lines(&postfix_mailbox, &mailbox_lines)?;
            Self::postmap(&postfix_mailbox)?;
        }

        if !cfg!(windows) {
            let domain = address.split('@').nth(1).unwrap_or_default();
            let user = Self::username_from_address(address).unwrap_or_default();
            if !domain.is_empty() && !user.is_empty() {
                let mail_root = Self::maildir_base_path(domain, &user);
                let _ = fs::remove_dir_all(mail_root);
            }
        }

        Self::reload_service("dovecot");
        Self::reload_service("postfix");
        Ok(())
    }

    fn apply_vmail_mailbox_password_reset(
        address: &str,
        new_password: &str,
        quota_mb: u32,
    ) -> Result<(), String> {
        let dovecot_users = Self::dovecot_users_file();
        Self::ensure_file(&dovecot_users)?;

        let mut lines = Self::read_lines(&dovecot_users)?;
        let mut updated = false;
        let new_hash = Self::hash_password(new_password);

        for line in &mut lines {
            if line.starts_with(&format!("{}:", address)) {
                let parts: Vec<&str> = line.splitn(3, ':').collect();
                if parts.len() == 3 {
                    *line = format!("{}:{}:{}", parts[0], new_hash, parts[2]);
                } else {
                    *line = format!("{}:{}", address, new_hash);
                }
                updated = true;
                break;
            }
        }

        if !updated {
            let domain = address.split('@').nth(1).unwrap_or_default();
            let user = Self::username_from_address(address).unwrap_or_default();
            if domain.is_empty() || user.is_empty() {
                return Err("Mailbox backend kaydi bulunamadi.".to_string());
            }
            let mail_root = Self::maildir_base_path(domain, &user);
            Self::ensure_maildir_permissions(&mail_root)?;
            lines.push(Self::dovecot_user_record(
                address,
                &new_hash,
                &mail_root,
                Self::vmail_quota_mb(quota_mb),
            ));
        }

        Self::write_lines(&dovecot_users, &lines)?;
        Self::reload_service("dovecot");
        Ok(())
    }

    fn sync_vmail_alias_maps(state: &MailState) -> Result<(), String> {
        let alias_map = Self::postfix_alias_map_file();
        Self::ensure_file(&alias_map)?;

        let mut lines: Vec<String> = Vec::new();
        lines.push("# AuraPanel managed virtual alias map".to_string());

        for rule in &state.forwards {
            lines.push(format!("{} {}", rule.source, rule.target));
        }

        for rule in &state.catch_all {
            if rule.enabled && !rule.target.trim().is_empty() {
                lines.push(format!("@{} {}", rule.domain, rule.target));
            }
        }

        Self::write_lines(&alias_map, &lines)?;
        Self::postmap(&alias_map)?;
        Self::reload_service("postfix");
        Ok(())
    }

    fn escape_regex_literal(value: &str) -> String {
        let mut out = String::new();
        for ch in value.chars() {
            match ch {
                '\\' | '.' | '+' | '?' | '(' | ')' | '[' | ']' | '{' | '}' | '^' | '$' | '|' => {
                    out.push('\\');
                    out.push(ch);
                }
                _ => out.push(ch),
            }
        }
        out
    }

    fn routing_pattern_to_regex(pattern: &str) -> String {
        let value = pattern.trim();
        if value.starts_with('/') && value.ends_with('/') && value.len() > 2 {
            return value[1..value.len() - 1].to_string();
        }
        if value.contains('*') {
            let escaped = Self::escape_regex_literal(value);
            let wildcard = escaped.replace('*', ".*");
            return format!("^{}$", wildcard);
        }
        format!("^{}$", Self::escape_regex_literal(value))
    }

    fn sync_vmail_routing_map(state: &MailState) -> Result<(), String> {
        let routing_map = Self::postfix_routing_map_file();
        Self::ensure_file(&routing_map)?;

        let mut items = state.routing.clone();
        items.sort_by(|a, b| a.priority.cmp(&b.priority).then(a.pattern.cmp(&b.pattern)));

        let mut lines: Vec<String> = Vec::new();
        lines.push("# AuraPanel managed routing regexp map".to_string());
        for rule in items {
            let regex = Self::routing_pattern_to_regex(&rule.pattern);
            lines.push(format!("{} {}", regex, rule.target));
        }

        Self::write_lines(&routing_map, &lines)?;
        Self::reload_service("postfix");
        Ok(())
    }

    fn validate_backend_for_write() -> Result<(), String> {
        let backend = Self::backend();
        if backend == "local" || Self::backend_is_vmail() {
            return Ok(());
        }
        Err(format!(
            "Mail backend '{}' is not implemented yet. Supported backends: local, vmail.",
            backend
        ))
    }

    pub fn list_mailboxes() -> Vec<LocalMailbox> {
        Self::load_state().map(|s| s.mailboxes).unwrap_or_default()
    }

    pub async fn create_mailbox(config: &MailboxConfig) -> Result<(), String> {
        Self::validate_backend_for_write()?;

        let domain = Self::normalize_domain(&config.domain);
        let username = Self::sanitize_local_part(&config.username);
        if domain.is_empty() || username.is_empty() {
            return Err("domain and username are required.".to_string());
        }

        let email_address = format!("{}@{}", username, domain);

        let mut state = Self::load_state()?;
        if state.mailboxes.iter().any(|m| m.address == email_address) {
            return Ok(());
        }

        let next_id = state.mailboxes.iter().map(|m| m.id).max().unwrap_or(0) + 1;
        state.mailboxes.push(LocalMailbox {
            id: next_id,
            address: email_address.clone(),
            domain: domain.clone(),
            quota_mb: config.quota_mb.max(128),
            used_mb: 0,
        });
        Self::upsert_mailbox_secret(&mut state, &email_address, &config.password);
        Self::save_state(&state)?;

        if Self::backend_is_vmail() {
            Self::apply_vmail_mailbox_create(
                &email_address,
                &domain,
                &username,
                &config.password,
                config.quota_mb,
            )?;
        }
        Ok(())
    }

    pub async fn delete_mailbox(email: &str) -> Result<(), String> {
        Self::validate_backend_for_write()?;
        let address = email.trim().to_ascii_lowercase();
        if address.is_empty() {
            return Err("address is required.".to_string());
        }

        let mut state = Self::load_state()?;
        state.mailboxes.retain(|m| m.address != address);
        state
            .mailbox_secrets
            .retain(|entry| !entry.address.eq_ignore_ascii_case(&address));
        state
            .sso_tokens
            .retain(|entry| !entry.address.eq_ignore_ascii_case(&address));
        state.forwards.retain(|f| f.source != address);
        Self::save_state(&state)?;

        if Self::backend_is_vmail() {
            Self::apply_vmail_mailbox_delete(&address)?;
            Self::sync_vmail_alias_maps(&state)?;
        }
        Ok(())
    }

    pub fn reset_mailbox_password(req: &MailboxPasswordResetRequest) -> Result<(), String> {
        Self::validate_backend_for_write()?;
        let address = req.address.trim().to_ascii_lowercase();
        let new_password = req.new_password.trim().to_string();

        if address.is_empty() || !address.contains('@') {
            return Err("valid address is required.".to_string());
        }
        if new_password.is_empty() {
            return Err("new_password is required.".to_string());
        }

        let mut state = Self::load_state()?;
        let Some(index) = state.mailboxes.iter().position(|x| x.address == address) else {
            return Err("Mailbox not found.".to_string());
        };

        if Self::backend_is_vmail() {
            let quota_mb = state.mailboxes[index].quota_mb;
            Self::apply_vmail_mailbox_password_reset(&address, &new_password, quota_mb)?;
        }
        Self::upsert_mailbox_secret(&mut state, &address, &new_password);
        Self::save_state(&state)?;
        Ok(())
    }

    pub fn list_forwards(domain: Option<&str>) -> Result<Vec<MailForwardRule>, String> {
        let mut rules = Self::load_state()?.forwards;
        if let Some(d) = domain {
            let d = Self::normalize_domain(d);
            rules.retain(|x| x.domain == d);
        }
        rules.sort_by(|a, b| a.domain.cmp(&b.domain).then(a.source.cmp(&b.source)));
        Ok(rules)
    }

    pub fn add_forward(config: &MailForwardConfig) -> Result<MailForwardRule, String> {
        Self::validate_backend_for_write()?;

        let domain = Self::normalize_domain(&config.domain);
        let source = Self::normalize_address(&domain, &config.source);
        let target = config.target.trim().to_ascii_lowercase();

        if domain.is_empty() || source.is_empty() || target.is_empty() || !target.contains('@') {
            return Err("domain, source and valid target address are required.".to_string());
        }

        let mut state = Self::load_state()?;
        let per_domain_count = state.forwards.iter().filter(|x| x.domain == domain).count();
        if per_domain_count >= 200 {
            return Err("Forward rule limit reached for domain (200).".to_string());
        }

        if let Some(existing) = state
            .forwards
            .iter_mut()
            .find(|x| x.domain == domain && x.source == source)
        {
            existing.target = target.clone();
            let updated = existing.clone();
            Self::save_state(&state)?;
            if Self::backend_is_vmail() {
                Self::sync_vmail_alias_maps(&state)?;
            }
            return Ok(updated);
        }

        let entry = MailForwardRule {
            domain,
            source,
            target,
            created_at: Self::now_ts(),
        };
        state.forwards.push(entry.clone());
        Self::save_state(&state)?;
        if Self::backend_is_vmail() {
            Self::sync_vmail_alias_maps(&state)?;
        }
        Ok(entry)
    }

    pub fn delete_forward(config: &MailForwardDeleteRequest) -> Result<(), String> {
        Self::validate_backend_for_write()?;

        let domain = Self::normalize_domain(&config.domain);
        let source = Self::normalize_address(&domain, &config.source);
        let mut state = Self::load_state()?;
        let before = state.forwards.len();
        state
            .forwards
            .retain(|x| !(x.domain == domain && x.source == source));
        if before == state.forwards.len() {
            return Err("Forward rule not found.".to_string());
        }
        Self::save_state(&state)?;
        if Self::backend_is_vmail() {
            Self::sync_vmail_alias_maps(&state)?;
        }
        Ok(())
    }

    pub fn get_catch_all(domain: &str) -> Result<Option<MailCatchAllRule>, String> {
        let domain = Self::normalize_domain(domain);
        if domain.is_empty() {
            return Err("domain is required.".to_string());
        }
        let state = Self::load_state()?;
        Ok(state.catch_all.into_iter().find(|x| x.domain == domain))
    }

    pub fn set_catch_all(config: &MailCatchAllConfig) -> Result<MailCatchAllRule, String> {
        Self::validate_backend_for_write()?;

        let domain = Self::normalize_domain(&config.domain);
        let target = config.target.trim().to_ascii_lowercase();
        if domain.is_empty() {
            return Err("domain is required.".to_string());
        }
        if config.enabled && (target.is_empty() || !target.contains('@')) {
            return Err("target address is required when catch-all is enabled.".to_string());
        }

        let mut state = Self::load_state()?;
        let now = Self::now_ts();

        if let Some(existing) = state.catch_all.iter_mut().find(|x| x.domain == domain) {
            existing.enabled = config.enabled;
            existing.target = target.clone();
            existing.updated_at = now;
            let out = existing.clone();
            Self::save_state(&state)?;
            if Self::backend_is_vmail() {
                Self::sync_vmail_alias_maps(&state)?;
            }
            return Ok(out);
        }

        let entry = MailCatchAllRule {
            domain,
            enabled: config.enabled,
            target,
            updated_at: now,
        };
        state.catch_all.push(entry.clone());
        Self::save_state(&state)?;
        if Self::backend_is_vmail() {
            Self::sync_vmail_alias_maps(&state)?;
        }
        Ok(entry)
    }

    pub fn list_routing_rules(domain: Option<&str>) -> Result<Vec<MailRoutingRule>, String> {
        let mut items = Self::load_state()?.routing;
        if let Some(d) = domain {
            let d = Self::normalize_domain(d);
            items.retain(|x| x.domain == d);
        }
        items.sort_by(|a, b| a.priority.cmp(&b.priority).then(a.pattern.cmp(&b.pattern)));
        Ok(items)
    }

    pub fn add_routing_rule(config: &MailRoutingConfig) -> Result<MailRoutingRule, String> {
        Self::validate_backend_for_write()?;

        let domain = Self::normalize_domain(&config.domain);
        let pattern = config.pattern.trim().to_string();
        let target = config.target.trim().to_ascii_lowercase();

        if domain.is_empty() || pattern.is_empty() || target.is_empty() {
            return Err("domain, pattern and target are required.".to_string());
        }

        let mut state = Self::load_state()?;
        let per_domain_count = state.routing.iter().filter(|x| x.domain == domain).count();
        if per_domain_count >= 128 {
            return Err("Routing rule limit reached for domain (128).".to_string());
        }

        let id_seed = format!("{}:{}:{}", domain, pattern, Self::now_ts());
        let mut hasher = DefaultHasher::new();
        id_seed.hash(&mut hasher);
        let id = format!("rt-{:x}", hasher.finish());

        let entry = MailRoutingRule {
            id,
            domain,
            pattern,
            target,
            priority: config.priority,
            created_at: Self::now_ts(),
        };
        state.routing.push(entry.clone());
        Self::save_state(&state)?;
        if Self::backend_is_vmail() {
            Self::sync_vmail_routing_map(&state)?;
        }
        Ok(entry)
    }

    pub fn delete_routing_rule(config: &MailRoutingDeleteRequest) -> Result<(), String> {
        Self::validate_backend_for_write()?;

        let domain = Self::normalize_domain(&config.domain);
        let id = config.id.trim().to_string();
        let mut state = Self::load_state()?;
        let before = state.routing.len();
        state
            .routing
            .retain(|x| !(x.domain == domain && x.id == id));
        if before == state.routing.len() {
            return Err("Routing rule not found.".to_string());
        }
        Self::save_state(&state)?;
        if Self::backend_is_vmail() {
            Self::sync_vmail_routing_map(&state)?;
        }
        Ok(())
    }

    pub fn get_dkim(domain: &str) -> Result<Option<MailDkimRecord>, String> {
        let domain = Self::normalize_domain(domain);
        if domain.is_empty() {
            return Err("domain is required.".to_string());
        }
        let state = Self::load_state()?;
        Ok(state.dkim.into_iter().find(|x| x.domain == domain))
    }

    pub fn rotate_dkim(domain: &str) -> Result<MailDkimRecord, String> {
        Self::validate_backend_for_write()?;

        let domain = Self::normalize_domain(domain);
        if domain.is_empty() {
            return Err("domain is required.".to_string());
        }

        let now = Self::now_ts();
        let selector = format!("s{}", now);
        let public_key = format!("v=DKIM1; k=rsa; p={:x}", now.saturating_mul(17));
        let private_key = format!(
            "-----BEGIN PRIVATE KEY-----\n{:x}\n-----END PRIVATE KEY-----",
            now.saturating_mul(31)
        );

        let mut state = Self::load_state()?;
        if let Some(existing) = state.dkim.iter_mut().find(|x| x.domain == domain) {
            existing.selector = selector;
            existing.public_key = public_key;
            existing.private_key = private_key;
            existing.updated_at = now;
            let out = existing.clone();
            Self::save_state(&state)?;
            return Ok(out);
        }

        let entry = MailDkimRecord {
            domain,
            selector,
            public_key,
            private_key,
            updated_at: now,
        };
        state.dkim.push(entry.clone());
        Self::save_state(&state)?;
        Ok(entry)
    }

    pub fn generate_webmail_sso_link(
        req: &MailWebmailSsoRequest,
    ) -> Result<MailWebmailSsoLink, String> {
        let address = req.address.trim().to_ascii_lowercase();
        if address.is_empty() || !address.contains('@') {
            return Err("valid address is required.".to_string());
        }

        let ttl = req.ttl_seconds.unwrap_or(300).clamp(60, 1800);
        let now = Self::now_ts();
        let expires_at = now.saturating_add(ttl);
        let token = Self::generate_sso_token()?;

        let mut state = Self::load_state()?;
        let mailbox_exists = state
            .mailboxes
            .iter()
            .any(|m| m.address.eq_ignore_ascii_case(&address));
        if !mailbox_exists {
            return Err("Mailbox not found.".to_string());
        }
        let _ = Self::mailbox_password_for_sso(&state, &address)?;

        Self::cleanup_expired_sso_tokens(&mut state, now);
        state.sso_tokens.push(MailWebmailSsoTokenRecord {
            token: token.clone(),
            address: address.clone(),
            expires_at,
            created_at: now,
        });
        Self::save_state(&state)?;
        Self::ensure_roundcube_sso_bridge()?;

        let base = std::env::var("AURAPANEL_WEBMAIL_BASE_URL")
            .unwrap_or_else(|_| "http://127.0.0.1/webmail".to_string())
            .trim()
            .trim_end_matches('/')
            .to_string();

        Ok(MailWebmailSsoLink {
            url: format!("{}/sso?token={}", base, token),
            expires_at,
        })
    }

    pub fn consume_webmail_sso_token(
        req: &MailWebmailSsoConsumeRequest,
    ) -> Result<MailWebmailSsoSession, String> {
        let token = req.token.trim().to_string();
        if token.is_empty() {
            return Err("token is required.".to_string());
        }

        let mut state = Self::load_state()?;
        let now = Self::now_ts();
        Self::cleanup_expired_sso_tokens(&mut state, now);

        let Some(index) = state
            .sso_tokens
            .iter()
            .position(|x| x.token == token && x.expires_at > now)
        else {
            return Err("SSO token invalid or expired.".to_string());
        };
        let entry = state.sso_tokens.remove(index);
        let password = Self::mailbox_password_for_sso(&state, &entry.address)?;
        Self::save_state(&state)?;

        Ok(MailWebmailSsoSession {
            address: entry.address,
            password,
            expires_at: entry.expires_at,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::test_support::env_lock;
    use std::time::{SystemTime, UNIX_EPOCH};

    fn setup_env(test_name: &str) -> std::path::PathBuf {
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|d| d.as_nanos())
            .unwrap_or(0);
        let path = std::env::temp_dir().join(format!("aurapanel-mail-test-{}-{}", test_name, now));
        std::fs::create_dir_all(&path).expect("temp state dir");
        std::env::set_var("AURAPANEL_STATE_DIR", &path);
        std::env::set_var("AURAPANEL_MAIL_BACKEND", "local");
        path
    }

    fn teardown_env(path: &std::path::Path) {
        std::env::remove_var("AURAPANEL_STATE_DIR");
        std::env::remove_var("AURAPANEL_MAIL_BACKEND");
        let _ = std::fs::remove_dir_all(path);
    }

    #[test]
    fn forward_rule_lifecycle_works() {
        let _guard = env_lock().lock().expect("test lock");
        let state_dir = setup_env("forward");

        let created = MailManager::add_forward(&MailForwardConfig {
            domain: "Example.COM".to_string(),
            source: "info".to_string(),
            target: "target@example.net".to_string(),
        })
        .expect("forward create");
        assert_eq!(created.domain, "example.com");
        assert_eq!(created.source, "info@example.com");

        let listed = MailManager::list_forwards(Some("example.com")).expect("forward list");
        assert_eq!(listed.len(), 1);

        MailManager::delete_forward(&MailForwardDeleteRequest {
            domain: "example.com".to_string(),
            source: "info".to_string(),
        })
        .expect("forward delete");
        let listed_after = MailManager::list_forwards(Some("example.com")).expect("forward list");
        assert!(listed_after.is_empty());

        teardown_env(&state_dir);
    }

    #[test]
    fn catch_all_routing_and_dkim_work() {
        let _guard = env_lock().lock().expect("test lock");
        let state_dir = setup_env("mailops");

        let catch_all = MailManager::set_catch_all(&MailCatchAllConfig {
            domain: "example.com".to_string(),
            enabled: true,
            target: "ops@example.com".to_string(),
        })
        .expect("set catch all");
        assert!(catch_all.enabled);
        assert_eq!(catch_all.target, "ops@example.com");

        let route = MailManager::add_routing_rule(&MailRoutingConfig {
            domain: "example.com".to_string(),
            pattern: "invoice*".to_string(),
            target: "billing@example.com".to_string(),
            priority: 10,
        })
        .expect("routing create");
        assert!(route.id.starts_with("rt-"));

        let routing = MailManager::list_routing_rules(Some("example.com")).expect("routing list");
        assert_eq!(routing.len(), 1);

        let dkim = MailManager::rotate_dkim("example.com").expect("dkim rotate");
        assert_eq!(dkim.domain, "example.com");
        assert!(dkim.selector.starts_with('s'));

        teardown_env(&state_dir);
    }

    #[test]
    fn webmail_sso_requires_valid_address() {
        let _guard = env_lock().lock().expect("test lock");
        let state_dir = setup_env("sso");

        let invalid = MailManager::generate_webmail_sso_link(&MailWebmailSsoRequest {
            address: "invalid".to_string(),
            ttl_seconds: Some(300),
        });
        assert!(invalid.is_err());

        let rt = tokio::runtime::Builder::new_current_thread()
            .enable_all()
            .build()
            .expect("runtime");
        rt.block_on(MailManager::create_mailbox(&MailboxConfig {
            domain: "example.com".to_string(),
            username: "user".to_string(),
            password: "sso-pass-123".to_string(),
            quota_mb: 512,
            owner: None,
        }))
        .expect("mailbox create");

        let link = MailManager::generate_webmail_sso_link(&MailWebmailSsoRequest {
            address: "user@example.com".to_string(),
            ttl_seconds: Some(120),
        })
        .expect("sso link");
        assert!(link.url.contains("token="));
        assert!(!link.url.contains("address="));

        let token = link
            .url
            .split("token=")
            .nth(1)
            .unwrap_or_default()
            .to_string();
        assert!(!token.is_empty());

        let session = MailManager::consume_webmail_sso_token(&MailWebmailSsoConsumeRequest {
            token: token.clone(),
        })
        .expect("consume sso token");
        assert_eq!(session.address, "user@example.com");
        assert_eq!(session.password, "sso-pass-123");

        let consumed_again =
            MailManager::consume_webmail_sso_token(&MailWebmailSsoConsumeRequest { token });
        assert!(consumed_again.is_err());

        teardown_env(&state_dir);
    }
}
