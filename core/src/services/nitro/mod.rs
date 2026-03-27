use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, HashSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

fn state_dir() -> PathBuf {
    if let Ok(path) = std::env::var("AURAPANEL_STATE_DIR") {
        return PathBuf::from(path);
    }
    std::env::temp_dir().join("aurapanel")
}

fn suspended_state_file() -> PathBuf {
    state_dir().join("suspended_vhosts.json")
}

fn metadata_state_file() -> PathBuf {
    state_dir().join("vhost_metadata.json")
}

fn load_suspended_vhosts() -> HashSet<String> {
    let path = suspended_state_file();
    if !path.exists() {
        return HashSet::new();
    }
    fs::read_to_string(path)
        .ok()
        .and_then(|raw| serde_json::from_str::<Vec<String>>(&raw).ok())
        .map(|items| items.into_iter().collect())
        .unwrap_or_default()
}

fn save_suspended_vhosts(items: &HashSet<String>) -> Result<(), String> {
    let path = suspended_state_file();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| format!("State directory could not be created: {}", e))?;
    }
    let mut ordered: Vec<String> = items.iter().cloned().collect();
    ordered.sort();
    let payload = serde_json::to_string_pretty(&ordered).map_err(|e| e.to_string())?;
    fs::write(path, payload).map_err(|e| format!("Suspended state could not be written: {}", e))
}

#[derive(Debug, Serialize, Deserialize)]
pub struct VHostConfig {
    pub domain: String,
    pub user: String,
    pub php_version: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct VHostUpdateConfig {
    pub domain: String,
    pub owner: Option<String>,
    pub php_version: Option<String>,
    pub package: Option<String>,
    pub email: Option<String>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct VHostUpdateResult {
    pub domain: String,
    pub owner: String,
    pub php_version: String,
    pub package: String,
    pub email: String,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
struct VHostMetadata {
    domain: String,
    owner: String,
    php_version: String,
    package: String,
    email: String,
    updated_at: u64,
}

fn now_ts() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0)
}

fn sanitize_domain(domain: &str) -> String {
    domain.trim().to_lowercase().trim_matches('.').to_string()
}

fn normalize_owner(owner: &str) -> Option<String> {
    let cleaned = owner.trim();
    if cleaned.is_empty() {
        return None;
    }
    if cleaned.contains('/') || cleaned.contains('\\') || cleaned.contains(' ') {
        return None;
    }
    Some(cleaned.to_string())
}

fn normalize_php_version(raw: &str) -> Option<String> {
    let mut cleaned = raw.trim().to_lowercase();
    if cleaned.starts_with("php") {
        cleaned = cleaned.trim_start_matches("php").trim().to_string();
    }
    if cleaned.is_empty() {
        return None;
    }

    if cleaned.contains('.') {
        let mut parts = cleaned.split('.');
        let major = parts.next()?;
        let minor = parts.next()?;
        if parts.next().is_some() {
            return None;
        }
        if major.chars().all(|c| c.is_ascii_digit())
            && minor.chars().all(|c| c.is_ascii_digit())
            && !major.is_empty()
            && !minor.is_empty()
        {
            return Some(format!("{}.{}", major, minor));
        }
        return None;
    }

    if cleaned.chars().all(|c| c.is_ascii_digit()) {
        if cleaned.len() == 2 {
            let mut chars = cleaned.chars();
            let major = chars.next()?;
            let minor = chars.next()?;
            return Some(format!("{}.{}", major, minor));
        }
        if cleaned.len() == 3 {
            let mut chars = cleaned.chars();
            let major = chars.next()?;
            let minor1 = chars.next()?;
            let minor2 = chars.next()?;
            return Some(format!("{}.{}{}", major, minor1, minor2));
        }
    }

    None
}

fn php_version_to_ols_segment(version: &str) -> Option<String> {
    let normalized = normalize_php_version(version)?;
    Some(normalized.replace('.', ""))
}

fn normalize_package(package: &str) -> String {
    let cleaned = package.trim();
    if cleaned.is_empty() {
        "default".to_string()
    } else {
        cleaned.to_string()
    }
}

fn is_valid_email(email: &str) -> bool {
    if email.is_empty() || email.contains(' ') {
        return false;
    }
    let mut split = email.split('@');
    let local = split.next().unwrap_or_default();
    let domain = split.next().unwrap_or_default();
    if split.next().is_some() {
        return false;
    }
    !local.is_empty() && domain.contains('.')
}

fn normalize_email(email: &str) -> Option<String> {
    let cleaned = email.trim().to_lowercase();
    if is_valid_email(&cleaned) {
        Some(cleaned)
    } else {
        None
    }
}

fn load_vhost_metadata() -> BTreeMap<String, VHostMetadata> {
    let path = metadata_state_file();
    if !path.exists() {
        return BTreeMap::new();
    }

    let raw = match fs::read_to_string(&path) {
        Ok(value) => value,
        Err(_) => return BTreeMap::new(),
    };

    if let Ok(map) = serde_json::from_str::<BTreeMap<String, VHostMetadata>>(&raw) {
        return map;
    }

    if let Ok(list) = serde_json::from_str::<Vec<VHostMetadata>>(&raw) {
        let mut converted = BTreeMap::new();
        for item in list {
            converted.insert(item.domain.clone(), item);
        }
        return converted;
    }

    BTreeMap::new()
}

fn save_vhost_metadata(items: &BTreeMap<String, VHostMetadata>) -> Result<(), String> {
    let path = metadata_state_file();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| format!("State directory could not be created: {}", e))?;
    }
    let payload = serde_json::to_string_pretty(items).map_err(|e| e.to_string())?;
    fs::write(path, payload).map_err(|e| format!("VHost metadata could not be written: {}", e))
}

fn upsert_vhost_metadata(entry: VHostMetadata) -> Result<(), String> {
    let mut map = load_vhost_metadata();
    map.insert(entry.domain.clone(), entry);
    save_vhost_metadata(&map)
}

fn remove_vhost_metadata(domain: &str) -> Result<(), String> {
    let mut map = load_vhost_metadata();
    map.remove(domain);
    save_vhost_metadata(&map)
}

fn vhost_conf_file(domain: &str) -> PathBuf {
    PathBuf::from("/usr/local/lsws/conf/vhosts")
        .join(domain)
        .join("vhconf.conf")
}

fn extract_php_from_vhconf(content: &str) -> Option<String> {
    for line in content.lines() {
        let trimmed = line.trim();
        if !trimmed.starts_with("path") {
            continue;
        }
        let marker = "/lsphp";
        if let Some(idx) = trimmed.find(marker) {
            let suffix = &trimmed[(idx + marker.len())..];
            let digits: String = suffix.chars().take_while(|c| c.is_ascii_digit()).collect();
            if !digits.is_empty() {
                return normalize_php_version(&digits);
            }
        }
    }
    None
}

fn extract_admin_email_from_vhconf(content: &str) -> Option<String> {
    for line in content.lines() {
        let trimmed = line.trim();
        if trimmed.starts_with("adminEmails") {
            let value = trimmed.trim_start_matches("adminEmails").trim();
            if let Some(normalized) = normalize_email(value) {
                return Some(normalized);
            }
        }
    }
    None
}

fn update_vhconf_content(content: &str, php_version: &str, email: &str) -> Result<String, String> {
    let php_segment = php_version_to_ols_segment(php_version)
        .ok_or_else(|| "Gecersiz PHP versiyonu".to_string())?;

    let mut updated_lines = Vec::new();
    let mut php_line_updated = false;
    let mut email_line_updated = false;

    for line in content.lines() {
        let trimmed = line.trim();

        if trimmed.starts_with("adminEmails") {
            updated_lines.push(format!("adminEmails               {}", email));
            email_line_updated = true;
            continue;
        }

        if trimmed.starts_with("path") && trimmed.contains("/lsphp") {
            updated_lines.push(format!("  path                    /usr/local/lsws/lsphp{}/bin/lsphp", php_segment));
            php_line_updated = true;
            continue;
        }

        updated_lines.push(line.to_string());
    }

    if !php_line_updated {
        return Err("vhconf icinde PHP path satiri bulunamadi".to_string());
    }
    if !email_line_updated {
        return Err("vhconf icinde adminEmails satiri bulunamadi".to_string());
    }

    let mut output = updated_lines.join("\n");
    if content.ends_with('\n') {
        output.push('\n');
    }
    Ok(output)
}

pub struct NitroEngine;

impl NitroEngine {
    /// Creates a new OpenLiteSpeed Virtual Host and its directories
    pub fn create_vhost(config: &VHostConfig) -> Result<(), String> {
        let domain = sanitize_domain(&config.domain);
        let owner = normalize_owner(&config.user)
            .ok_or_else(|| "Gecersiz kullanici".to_string())?;
        let php_version = normalize_php_version(&config.php_version)
            .ok_or_else(|| "Gecersiz PHP versiyonu".to_string())?;

        // 1. Create required directories
        let home_dir = format!("/home/{}", owner);
        let public_html = format!("{}/public_html/{}", home_dir, domain);
        let vhost_conf_dir = format!("/usr/local/lsws/conf/vhosts/{}", domain);

        if !Path::new("/usr/local/lsws").exists() {
            return Err("OpenLiteSpeed is not installed on this system.".to_string());
        }

        fs::create_dir_all(&public_html)
            .map_err(|e| format!("Ana dizin olusturulamadi: {}", e))?;
        fs::create_dir_all(&vhost_conf_dir)
            .map_err(|e| format!("Vhost config dizini olusturulamadi: {}", e))?;

        // 2. Write vhost config template
        let vhconf_content = format!(
            r#"
docRoot                   $VH_ROOT/public_html/{domain}
vhDomain                  {domain}
vhAliases                 www.{domain}
adminEmails               webmaster@{domain}
enableGzip                1

index  {{
  useServer               0
  indexFiles              index.php, index.html
}}

context / {{
  allowBrowse             1
  rewrite  {{
    enable                1
    autoLoadHtaccess      1
  }}
}}

extprocessor {domain}_php {{
  type                    lsapi
  address                 UDS://tmp/lshttpd/{domain}.sock
  maxConns                35
  env                     PHP_LSAPI_CHILDREN=35
  initTimeout             60
  retryTimeout            0
  persistConn             1
  respBuffer              0
  autoStart               1
  path                    /usr/local/lsws/lsphp{php_version}/bin/lsphp
  backlog                 100
  instances               1
  runOnStartUp            3
}}
            "#,
            domain = domain,
            php_version = php_version.replace('.', "")
        );

        let conf_file = format!("{}/vhconf.conf", vhost_conf_dir);
        fs::write(&conf_file, vhconf_content)
            .map_err(|e| format!("vhconf.conf yazilamadi: {}", e))?;

        let mut suspended = load_suspended_vhosts();
        if suspended.remove(&domain) {
            let _ = save_suspended_vhosts(&suspended);
        }

        if let Err(err) = upsert_vhost_metadata(VHostMetadata {
            domain: domain.clone(),
            owner,
            php_version,
            package: "default".to_string(),
            email: format!("webmaster@{}", domain),
            updated_at: now_ts(),
        }) {
            eprintln!("[WARN] VHost metadata update failed for {}: {}", domain, err);
        }

        // 3. Reload OpenLiteSpeed gracefully
        Self::reload_ols()?;

        Ok(())
    }

    /// Graceful restart for OpenLiteSpeed
    pub fn reload_ols() -> Result<(), String> {
        if !Path::new("/usr/local/lsws").exists() {
            return Err("OpenLiteSpeed is not installed on this system.".to_string());
        }

        let output = Command::new("/usr/local/lsws/bin/lswsctrl")
            .arg("restart")
            .output()
            .map_err(|e| format!("OLS command could not run: {}", e))?;

        if !output.status.success() {
            return Err(format!("OLS restart failed: {:?}", output.stderr));
        }

        Ok(())
    }

    /// List all virtual hosts
    pub fn list_vhosts() -> Result<Vec<serde_json::Value>, String> {
        use serde_json::json;
        let suspended = load_suspended_vhosts();
        let metadata = load_vhost_metadata();

        if !Path::new("/usr/local/lsws").exists() {
            return Err("OpenLiteSpeed is not installed on this system.".to_string());
        }

        // Linux scan: enumerate users and /home/<user>/public_html/*
        let mut sites = Vec::new();
        let home_dir = Path::new("/home");
        if let Ok(users) = fs::read_dir(home_dir) {
            for user_entry in users.flatten() {
                let public_html = user_entry.path().join("public_html");
                if let Ok(domains) = fs::read_dir(&public_html) {
                    for domain_entry in domains.flatten() {
                        let domain = sanitize_domain(&domain_entry.file_name().to_string_lossy());
                        let has_ssl = Path::new(&format!("/etc/letsencrypt/live/{}", domain)).exists();
                        let is_suspended = suspended.contains(&domain);
                        let owner_from_fs = user_entry.file_name().to_string_lossy().to_string();

                        let conf_file = vhost_conf_file(&domain);
                        let conf_content = fs::read_to_string(&conf_file).ok();
                        let conf_php = conf_content
                            .as_deref()
                            .and_then(extract_php_from_vhconf)
                            .unwrap_or_else(|| "8.3".to_string());
                        let conf_email = conf_content
                            .as_deref()
                            .and_then(extract_admin_email_from_vhconf)
                            .unwrap_or_else(|| format!("webmaster@{}", domain));

                        let meta = metadata.get(&domain).cloned().unwrap_or(VHostMetadata {
                            domain: domain.clone(),
                            owner: owner_from_fs,
                            php_version: conf_php,
                            package: "default".to_string(),
                            email: conf_email,
                            updated_at: 0,
                        });

                        let disk = Self::dir_size_human(&domain_entry.path());
                        sites.push(json!({
                            "domain": domain,
                            "ssl": has_ssl,
                            "disk_usage": disk,
                            "quota": "10 GB",
                            "php": meta.php_version,
                            "php_version": meta.php_version,
                            "user": meta.owner,
                            "owner": meta.owner,
                            "package": meta.package,
                            "email": meta.email,
                            "status": if is_suspended { "suspended" } else { "active" }
                        }));
                    }
                }
            }
        }
        Ok(sites)
    }

    /// Delete one virtual host
    pub fn delete_vhost(domain: &str) -> Result<String, String> {
        let domain = sanitize_domain(domain);

        if !Path::new("/usr/local/lsws").exists() {
            return Err("OpenLiteSpeed is not installed on this system.".to_string());
        }

        let conf_dir = format!("/usr/local/lsws/conf/vhosts/{}", domain);
        if Path::new(&conf_dir).exists() {
            fs::remove_dir_all(&conf_dir)
                .map_err(|e| format!("Vhost conf directory could not be deleted: {}", e))?;
        }

        Self::reload_ols()?;

        let mut suspended = load_suspended_vhosts();
        if suspended.remove(&domain) {
            let _ = save_suspended_vhosts(&suspended);
        }

        let _ = remove_vhost_metadata(&domain);

        Ok(format!("{} deleted successfully.", domain))
    }

    pub fn suspend_vhost(domain: &str) -> Result<String, String> {
        let domain = sanitize_domain(domain);
        if domain.is_empty() {
            return Err("domain is required.".to_string());
        }
        let mut suspended = load_suspended_vhosts();
        suspended.insert(domain.clone());
        save_suspended_vhosts(&suspended)?;
        Ok(format!("{} suspended.", domain))
    }

    pub fn unsuspend_vhost(domain: &str) -> Result<String, String> {
        let domain = sanitize_domain(domain);
        if domain.is_empty() {
            return Err("domain is required.".to_string());
        }
        let mut suspended = load_suspended_vhosts();
        suspended.remove(&domain);
        save_suspended_vhosts(&suspended)?;
        Ok(format!("{} unsuspended.", domain))
    }

    pub fn update_vhost(config: &VHostUpdateConfig) -> Result<VHostUpdateResult, String> {
        let domain = sanitize_domain(&config.domain);
        if domain.is_empty() {
            return Err("domain zorunludur".to_string());
        }

        let mut metadata = load_vhost_metadata();
        let existing = metadata.get(&domain).cloned();

        let conf_file = vhost_conf_file(&domain);
        let ols_exists = Path::new("/usr/local/lsws").exists();
        let conf_content = if ols_exists && conf_file.exists() {
            Some(
                fs::read_to_string(&conf_file)
                    .map_err(|e| format!("vhconf okunamadi: {}", e))?,
            )
        } else {
            None
        };

        if !ols_exists {
            return Err("OpenLiteSpeed is not installed on this system.".to_string());
        }

        if ols_exists && conf_content.is_none() {
            return Err(format!("VHost config bulunamadi: {}", conf_file.display()));
        }

        let fallback_owner = existing
            .as_ref()
            .map(|x| x.owner.clone())
            .unwrap_or_else(|| "aura".to_string());
        let fallback_php = existing
            .as_ref()
            .map(|x| x.php_version.clone())
            .or_else(|| conf_content.as_deref().and_then(extract_php_from_vhconf))
            .unwrap_or_else(|| "8.3".to_string());
        let fallback_package = existing
            .as_ref()
            .map(|x| x.package.clone())
            .unwrap_or_else(|| "default".to_string());
        let fallback_email = existing
            .as_ref()
            .map(|x| x.email.clone())
            .or_else(|| conf_content.as_deref().and_then(extract_admin_email_from_vhconf))
            .unwrap_or_else(|| format!("webmaster@{}", domain));

        let owner = normalize_owner(config.owner.as_deref().unwrap_or(&fallback_owner))
            .ok_or_else(|| "Gecersiz owner".to_string())?;
        let php_version = normalize_php_version(config.php_version.as_deref().unwrap_or(&fallback_php))
            .ok_or_else(|| "Gecersiz php_version".to_string())?;
        let package = normalize_package(config.package.as_deref().unwrap_or(&fallback_package));
        let email = normalize_email(config.email.as_deref().unwrap_or(&fallback_email))
            .ok_or_else(|| "Gecersiz email".to_string())?;

        if let Some(content) = conf_content.as_deref() {
            let updated_content = update_vhconf_content(content, &php_version, &email)?;
            fs::write(&conf_file, updated_content)
                .map_err(|e| format!("vhconf yazilamadi: {}", e))?;
            Self::reload_ols()?;
        }

        metadata.insert(domain.clone(), VHostMetadata {
            domain: domain.clone(),
            owner: owner.clone(),
            php_version: php_version.clone(),
            package: package.clone(),
            email: email.clone(),
            updated_at: now_ts(),
        });
        save_vhost_metadata(&metadata)?;

        Ok(VHostUpdateResult {
            domain,
            owner,
            php_version,
            package,
            email,
        })
    }

    fn dir_size_human(path: &Path) -> String {
        let mut total: u64 = 0;
        if let Ok(entries) = fs::read_dir(path) {
            for entry in entries.flatten() {
                if let Ok(meta) = entry.metadata() {
                    total += meta.len();
                }
            }
        }
        if total > 1_073_741_824 {
            format!("{:.1} GB", total as f64 / 1_073_741_824.0)
        } else if total > 1_048_576 {
            format!("{:.1} MB", total as f64 / 1_048_576.0)
        } else {
            format!("{} KB", total / 1024)
        }
    }
}
