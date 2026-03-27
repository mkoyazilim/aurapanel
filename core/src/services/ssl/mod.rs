use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

#[derive(Serialize, Deserialize, Debug)]
pub struct SslConfig {
    pub domain: String,
    pub email: String,
    pub webroot: Option<String>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct SslCertificateDetails {
    pub domain: String,
    pub has_ssl: bool,
    pub status: String,
    pub cert_path: String,
    pub issuer: Option<String>,
    pub expiry_date: Option<String>,
    pub days_remaining: Option<i64>,
    pub error_message: Option<String>,
}

#[derive(Serialize, Deserialize, Debug, Clone, Default)]
struct SslBindingsState {
    hostname_ssl_domain: Option<String>,
    mail_ssl_domain: Option<String>,
    updated_at: u64,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct SslBindingsView {
    pub hostname_ssl_domain: Option<String>,
    pub mail_ssl_domain: Option<String>,
    pub updated_at: u64,
}

pub struct SslManager;

impl SslManager {
    fn now_ts() -> u64 {
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|d| d.as_secs())
            .unwrap_or(0)
    }

    fn normalize_domain(value: &str) -> String {
        value.trim().trim_end_matches('.').to_ascii_lowercase()
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

    fn bindings_path() -> PathBuf {
        Self::state_root().join("ssl_bindings.json")
    }

    fn load_bindings() -> Result<SslBindingsState, String> {
        let path = Self::bindings_path();
        if !path.exists() {
            return Ok(SslBindingsState::default());
        }
        let raw = fs::read_to_string(path).map_err(|e| e.to_string())?;
        serde_json::from_str(&raw).map_err(|e| e.to_string())
    }

    fn save_bindings(state: &SslBindingsState) -> Result<(), String> {
        let path = Self::bindings_path();
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).map_err(|e| e.to_string())?;
        }
        let payload = serde_json::to_string_pretty(state).map_err(|e| e.to_string())?;
        fs::write(path, payload).map_err(|e| e.to_string())
    }

    async fn issue_certificate_only(config: &SslConfig) -> Result<(), String> {
        let domain = Self::normalize_domain(&config.domain);
        let email = config.email.trim();
        if domain.is_empty() || email.is_empty() {
            return Err("domain and email are required.".to_string());
        }

        let webroot = config.webroot.as_deref().unwrap_or("/usr/local/lsws/Example/html");
        println!(
            "[ACME] Issuing SSL for {} via Let's Encrypt (email: {})",
            domain, email
        );

        if !Path::new("/usr/bin/certbot").exists() {
            return Err("certbot is not installed.".to_string());
        }

        let output = Command::new("certbot")
            .args([
                "certonly",
                "--webroot",
                "-w",
                webroot,
                "-d",
                &domain,
                "-d",
                &format!("www.{}", domain),
                "--email",
                email,
                "--agree-tos",
                "--non-interactive",
            ])
            .output()
            .map_err(|e| format!("certbot calistirilamadi: {}", e))?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(format!("SSL alinamadi: {}", stderr));
        }

        Ok(())
    }

    pub async fn issue_certificate(config: &SslConfig) -> Result<(), String> {
        Self::issue_certificate_only(config).await?;
        Self::bind_ssl_to_ols(&Self::normalize_domain(&config.domain))?;
        Ok(())
    }

    pub async fn issue_hostname_certificate(config: &SslConfig) -> Result<(), String> {
        Self::issue_certificate_only(config).await?;
        let mut state = Self::load_bindings()?;
        state.hostname_ssl_domain = Some(Self::normalize_domain(&config.domain));
        state.updated_at = Self::now_ts();
        Self::save_bindings(&state)
    }

    pub async fn issue_mail_server_certificate(config: &SslConfig) -> Result<(), String> {
        Self::issue_certificate_only(config).await?;
        Self::bind_ssl_to_mailstack(&Self::normalize_domain(&config.domain))?;
        let mut state = Self::load_bindings()?;
        state.mail_ssl_domain = Some(Self::normalize_domain(&config.domain));
        state.updated_at = Self::now_ts();
        Self::save_bindings(&state)
    }

    pub fn renew_all() -> Result<(), String> {
        println!("[ACME] Running certbot renew for all domains...");
        if !Path::new("/usr/bin/certbot").exists() {
            return Err("certbot is not installed.".to_string());
        }

        Command::new("certbot")
            .args(["renew", "--quiet"])
            .output()
            .map_err(|e| format!("Renewal failed: {}", e))?;

        Ok(())
    }

    pub fn get_bindings() -> Result<SslBindingsView, String> {
        let state = Self::load_bindings()?;
        Ok(SslBindingsView {
            hostname_ssl_domain: state.hostname_ssl_domain,
            mail_ssl_domain: state.mail_ssl_domain,
            updated_at: state.updated_at,
        })
    }

    pub fn certificate_details(domain: &str) -> Result<SslCertificateDetails, String> {
        let domain = Self::normalize_domain(domain);
        if domain.is_empty() {
            return Err("domain is required.".to_string());
        }

        let cert_path = format!("/etc/letsencrypt/live/{}/fullchain.pem", domain);
        if !Path::new(&cert_path).exists() {
            return Ok(SslCertificateDetails {
                domain,
                has_ssl: false,
                status: "missing".to_string(),
                cert_path,
                issuer: None,
                expiry_date: None,
                days_remaining: None,
                error_message: None,
            });
        }

        let output = Command::new("openssl")
            .args(["x509", "-in", &cert_path, "-noout", "-issuer", "-enddate"])
            .output()
            .map_err(|e| format!("openssl calistirilamadi: {}", e))?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
            return Ok(SslCertificateDetails {
                domain,
                has_ssl: false,
                status: "error".to_string(),
                cert_path,
                issuer: None,
                expiry_date: None,
                days_remaining: None,
                error_message: Some(stderr),
            });
        }

        let stdout = String::from_utf8_lossy(&output.stdout);
        let mut issuer: Option<String> = None;
        let mut expiry_date: Option<String> = None;

        for line in stdout.lines() {
            let trimmed = line.trim();
            if let Some(v) = trimmed.strip_prefix("issuer=") {
                issuer = Some(v.trim().to_string());
            } else if let Some(v) = trimmed.strip_prefix("notAfter=") {
                expiry_date = Some(v.trim().to_string());
            }
        }

        let days_remaining = if let Some(exp) = expiry_date.as_deref() {
            Self::days_until(exp)
        } else {
            None
        };
        let status = if let Some(days) = days_remaining {
            if days < 0 { "expired" } else { "active" }
        } else {
            "active"
        };

        Ok(SslCertificateDetails {
            domain,
            has_ssl: true,
            status: status.to_string(),
            cert_path,
            issuer,
            expiry_date,
            days_remaining,
            error_message: None,
        })
    }

    fn days_until(expiry_raw: &str) -> Option<i64> {
        let parsed = Command::new("date")
            .args(["-d", expiry_raw, "+%s"])
            .output()
            .ok()?;
        if !parsed.status.success() {
            return None;
        }
        let epoch = String::from_utf8_lossy(&parsed.stdout)
            .trim()
            .parse::<i64>()
            .ok()?;
        let now = Self::now_ts() as i64;
        Some((epoch - now) / 86_400)
    }

    fn bind_ssl_to_ols(domain: &str) -> Result<(), String> {
        let ssl_block = format!(
            r#"
vhssl {{
  keyFile         /etc/letsencrypt/live/{domain}/privkey.pem
  certFile        /etc/letsencrypt/live/{domain}/fullchain.pem
  certChain       1
}}
"#
        );

        let vhconf_path = format!("/usr/local/lsws/conf/vhosts/{}/vhconf.conf", domain);
        if Path::new(&vhconf_path).exists() {
            let mut content = fs::read_to_string(&vhconf_path)
                .map_err(|e| format!("vhconf read failed: {}", e))?;
            content.push_str(&ssl_block);
            fs::write(&vhconf_path, content).map_err(|e| format!("vhconf write failed: {}", e))?;
        } else {
            return Err(format!("VHost config not found: {}", vhconf_path));
        }

        Ok(())
    }

    fn bind_ssl_to_mailstack(domain: &str) -> Result<(), String> {
        let cert_dir = PathBuf::from(format!("/etc/letsencrypt/live/{}", domain));
        let cert_file = cert_dir.join("fullchain.pem");
        let key_file = cert_dir.join("privkey.pem");

        if !cert_file.exists() || !key_file.exists() {
            return Err(format!(
                "Mail SSL cert files missing for {}: {} / {}",
                domain,
                cert_file.display(),
                key_file.display()
            ));
        }

        let postfix_main_cf = PathBuf::from(
            std::env::var("AURAPANEL_POSTFIX_MAIN_CF")
                .unwrap_or_else(|_| "/etc/postfix/main.cf".to_string()),
        );
        if postfix_main_cf.exists() {
            let mut content = fs::read_to_string(&postfix_main_cf)
                .map_err(|e| format!("postfix main.cf read failed: {}", e))?;
            content = Self::upsert_kv_line(&content, "smtpd_tls_cert_file", &cert_file.to_string_lossy());
            content = Self::upsert_kv_line(&content, "smtpd_tls_key_file", &key_file.to_string_lossy());
            content = Self::upsert_kv_line(&content, "smtp_tls_cert_file", &cert_file.to_string_lossy());
            content = Self::upsert_kv_line(&content, "smtp_tls_key_file", &key_file.to_string_lossy());
            fs::write(&postfix_main_cf, content)
                .map_err(|e| format!("postfix main.cf write failed: {}", e))?;
        }

        let dovecot_ssl_conf = PathBuf::from(
            std::env::var("AURAPANEL_DOVECOT_SSL_CONF")
                .unwrap_or_else(|_| "/etc/dovecot/conf.d/10-ssl.conf".to_string()),
        );
        if dovecot_ssl_conf.exists() {
            let mut content = fs::read_to_string(&dovecot_ssl_conf)
                .map_err(|e| format!("dovecot ssl conf read failed: {}", e))?;
            content = Self::upsert_kv_line(&content, "ssl", "required");
            content = Self::upsert_kv_line(
                &content,
                "ssl_cert",
                &format!("<{}", cert_file.to_string_lossy()),
            );
            content = Self::upsert_kv_line(
                &content,
                "ssl_key",
                &format!("<{}", key_file.to_string_lossy()),
            );
            fs::write(&dovecot_ssl_conf, content)
                .map_err(|e| format!("dovecot ssl conf write failed: {}", e))?;
        }

        let _ = Command::new("systemctl").args(["reload", "postfix"]).output();
        let _ = Command::new("systemctl").args(["restart", "dovecot"]).output();
        Ok(())
    }

    fn upsert_kv_line(content: &str, key: &str, value: &str) -> String {
        let mut replaced = false;
        let mut lines = Vec::new();
        for line in content.lines() {
            let trimmed = line.trim_start();
            let is_match = trimmed.starts_with(&format!("{} =", key))
                || trimmed.starts_with(&format!("{}=", key));
            if is_match {
                lines.push(format!("{} = {}", key, value));
                replaced = true;
            } else {
                lines.push(line.to_string());
            }
        }

        if !replaced {
            lines.push(format!("{} = {}", key, value));
        }
        let mut out = lines.join("\n");
        out.push('\n');
        out
    }
}
