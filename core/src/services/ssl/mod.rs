use serde::{Deserialize, Serialize};
use std::fs;
use std::net::{ToSocketAddrs, UdpSocket};
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

    fn certbot_binary() -> Option<&'static str> {
        if Path::new("/usr/bin/certbot").exists() {
            Some("/usr/bin/certbot")
        } else if Path::new("/snap/bin/certbot").exists() {
            Some("/snap/bin/certbot")
        } else {
            None
        }
    }

    fn certbot_missing_message() -> String {
        "certbot is not installed. Re-run AuraPanel installer, or install manually (Ubuntu/Debian: apt-get install -y certbot; Alma/Rocky: dnf install -y certbot).".to_string()
    }

    fn detect_server_ip() -> Option<String> {
        if let Ok(ip) = std::env::var("AURAPANEL_SERVER_IP") {
            let ip = ip.trim().to_string();
            if !ip.is_empty() {
                return Some(ip);
            }
        }

        let socket = UdpSocket::bind("0.0.0.0:0").ok()?;
        socket.connect("1.1.1.1:80").ok()?;
        let ip = socket.local_addr().ok()?.ip();
        if ip.is_loopback() || ip.is_unspecified() {
            None
        } else {
            Some(ip.to_string())
        }
    }

    fn dns_preflight_disabled() -> bool {
        std::env::var("AURAPANEL_SSL_SKIP_DNS_PREFLIGHT")
            .map(|v| {
                matches!(
                    v.trim().to_ascii_lowercase().as_str(),
                    "1" | "true" | "yes" | "on"
                )
            })
            .unwrap_or(false)
    }

    fn resolve_host_ips(host: &str) -> Result<Vec<String>, String> {
        let addr = format!("{}:80", host);
        let mut ips: Vec<String> = addr
            .to_socket_addrs()
            .map_err(|e| format!("DNS cozumleme basarisiz ({}): {}", host, e))?
            .map(|sock| sock.ip().to_string())
            .collect();
        ips.sort();
        ips.dedup();
        Ok(ips)
    }

    fn host_points_to_server(host: &str, server_ip: &str) -> Result<bool, String> {
        let ips = Self::resolve_host_ips(host)?;
        if ips.is_empty() {
            return Ok(false);
        }
        Ok(ips.iter().any(|ip| ip == server_ip))
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

        let webroot = config
            .webroot
            .as_deref()
            .unwrap_or("/usr/local/lsws/Example/html");
        println!(
            "[ACME] Issuing SSL for {} via Let's Encrypt (email: {})",
            domain, email
        );

        let certbot_bin = Self::certbot_binary().ok_or_else(Self::certbot_missing_message)?;

        let mut domains_for_cert = vec![domain.clone()];
        if !Self::dns_preflight_disabled() {
            let server_ip = Self::detect_server_ip().ok_or_else(|| {
                "Sunucu IP'si tespit edilemedi. AURAPANEL_SERVER_IP tanimlayin veya DNS preflight'i kapatin (AURAPANEL_SSL_SKIP_DNS_PREFLIGHT=1).".to_string()
            })?;

            let apex_ips = Self::resolve_host_ips(&domain)?;
            if apex_ips.is_empty() || !apex_ips.iter().any(|ip| ip == &server_ip) {
                return Err(format!(
                    "DNS preflight failed for {}. Beklenen sunucu IP: {}. Bulunan A/AAAA: [{}]. Domain kaydini bu sunucuya yonlendirin ve varsa proxy/CDN'i gecici DNS only yapin.",
                    domain,
                    server_ip,
                    apex_ips.join(", ")
                ));
            }

            let www_domain = format!("www.{}", domain);
            match Self::host_points_to_server(&www_domain, &server_ip) {
                Ok(true) => domains_for_cert.push(www_domain),
                Ok(false) => {
                    println!(
                        "[ACME] www domain DNS preflight skip edildi, sertifika yalnizca apex domain icin alinacak: {}",
                        www_domain
                    );
                }
                Err(err) => {
                    println!(
                        "[ACME] www domain DNS preflight okunamadi, sertifika yalnizca apex domain icin alinacak ({}): {}",
                        www_domain, err
                    );
                }
            }
        } else {
            domains_for_cert.push(format!("www.{}", domain));
        }

        let mut certbot_args = vec![
            "certonly".to_string(),
            "--webroot".to_string(),
            "-w".to_string(),
            webroot.to_string(),
        ];
        for host in domains_for_cert {
            certbot_args.push("-d".to_string());
            certbot_args.push(host);
        }
        certbot_args.push("--email".to_string());
        certbot_args.push(email.to_string());
        certbot_args.push("--agree-tos".to_string());
        certbot_args.push("--non-interactive".to_string());

        let output = Command::new(certbot_bin)
            .args(&certbot_args)
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
        Self::reload_ols()?;
        // Best-effort: ensure auto-renewal cron is in place after first successful issuance
        let _ = Self::ensure_renewal_cron();
        Ok(())
    }

    /// Ensures a system cron job exists for automatic certificate renewal.
    /// Runs certbot renew daily at 03:00, then reloads OLS.
    pub fn ensure_renewal_cron() -> Result<(), String> {
        let cron_path = "/etc/cron.d/aurapanel-ssl-renew";
        // Idempotent: skip if a managed cron already exists
        if let Ok(existing) = fs::read_to_string(cron_path) {
            if existing.contains("certbot renew") {
                return Ok(());
            }
        }
        let content = concat!(
            "# Managed by AuraPanel — do not edit manually\n",
            "SHELL=/bin/bash\n",
            "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n",
            "\n",
            "0 3 * * * root certbot renew --quiet",
            " --post-hook \"/usr/local/lsws/bin/lswsctrl restart\"",
            " >> /var/log/aurapanel-ssl-renew.log 2>&1\n",
        );
        fs::write(cron_path, content).map_err(|e| format!("SSL renew cron yazilamadi: {}", e))
    }

    /// Issues a wildcard certificate (`*.domain.tld`) using DNS-01 challenge.
    ///
    /// Requires either:
    ///   - `certbot-dns-cloudflare` (if `CLOUDFLARE_API_TOKEN` env is set), or
    ///   - `certbot-dns-powerdns` plugin, or
    ///   - Manual mode (returns instructions instead of auto-completing).
    ///
    /// On success the wildcard cert is stored in
    /// `/etc/letsencrypt/live/<domain>/fullchain.pem` and bound to OLS.
    pub async fn issue_wildcard_certificate(config: &SslConfig) -> Result<String, String> {
        let domain = Self::normalize_domain(&config.domain);
        let email = config.email.trim();
        if domain.is_empty() || email.is_empty() {
            return Err("domain ve email zorunludur.".to_string());
        }
        let certbot_bin = Self::certbot_binary().ok_or_else(Self::certbot_missing_message)?;

        // Prefer automatic DNS plugins when available
        let cf_credentials = format!("/etc/aurapanel/cloudflare-{}.ini", domain);
        let pdns_credentials = "/etc/aurapanel/pdns-credentials.ini";

        if Path::new(&cf_credentials).exists() {
            // certbot-dns-cloudflare
            let output = Command::new(certbot_bin)
                .args([
                    "certonly",
                    "--dns-cloudflare",
                    "--dns-cloudflare-credentials",
                    &cf_credentials,
                    "-d",
                    &domain,
                    "-d",
                    &format!("*.{}", domain),
                    "--email",
                    email,
                    "--agree-tos",
                    "--non-interactive",
                ])
                .output()
                .map_err(|e| format!("certbot calistirilamadi: {}", e))?;

            if !output.status.success() {
                return Err(String::from_utf8_lossy(&output.stderr).to_string());
            }
            Self::bind_ssl_to_ols(&domain)?;
            Self::reload_ols()?;
            let _ = Self::ensure_renewal_cron();
            return Ok(format!(
                "*.{} icin wildcard sertifika basariyla alindi.",
                domain
            ));
        }

        if Path::new(pdns_credentials).exists() {
            // certbot-dns-rfc2136 (compatible with PowerDNS)
            let output = Command::new(certbot_bin)
                .args([
                    "certonly",
                    "--dns-rfc2136",
                    "--dns-rfc2136-credentials",
                    pdns_credentials,
                    "-d",
                    &domain,
                    "-d",
                    &format!("*.{}", domain),
                    "--email",
                    email,
                    "--agree-tos",
                    "--non-interactive",
                ])
                .output()
                .map_err(|e| format!("certbot calistirilamadi: {}", e))?;

            if !output.status.success() {
                return Err(String::from_utf8_lossy(&output.stderr).to_string());
            }
            Self::bind_ssl_to_ols(&domain)?;
            Self::reload_ols()?;
            let _ = Self::ensure_renewal_cron();
            return Ok(format!(
                "*.{} icin wildcard sertifika basariyla alindi.",
                domain
            ));
        }

        // No DNS plugin: return manual instructions
        Err(format!(
            "Wildcard sertifika icin DNS-01 dogrulama gereklidir. \
            Lutfen DNS saglayiciniz icin bir credentials dosyasi olusturun:\n\
            - Cloudflare: /etc/aurapanel/cloudflare-{domain}.ini\n\
            - PowerDNS (RFC2136): /etc/aurapanel/pdns-credentials.ini\n\
            Daha fazla bilgi: https://certbot.eff.org/docs/using.html#dns-plugins"
        ))
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
        let certbot_bin = Self::certbot_binary().ok_or_else(Self::certbot_missing_message)?;
        Command::new(certbot_bin)
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
            if days < 0 {
                "expired"
            } else {
                "active"
            }
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
            let content = fs::read_to_string(&vhconf_path)
                .map_err(|e| format!("vhconf read failed: {}", e))?;
            let mut content = Self::strip_vhssl_blocks(&content);
            content.push_str(&ssl_block);
            fs::write(&vhconf_path, content).map_err(|e| format!("vhconf write failed: {}", e))?;
        } else {
            return Err(format!("VHost config not found: {}", vhconf_path));
        }

        Ok(())
    }

    fn strip_vhssl_blocks(content: &str) -> String {
        let mut result = Vec::new();
        let mut skipping = false;
        let mut depth = 0i32;

        for line in content.lines() {
            let trimmed = line.trim();
            if !skipping && trimmed.starts_with("vhssl") && trimmed.contains('{') {
                skipping = true;
                depth = line.matches('{').count() as i32 - line.matches('}').count() as i32;
                continue;
            }

            if skipping {
                depth += line.matches('{').count() as i32;
                depth -= line.matches('}').count() as i32;
                if depth <= 0 {
                    skipping = false;
                }
                continue;
            }

            result.push(line.to_string());
        }

        let mut output = result.join("\n");
        if !output.ends_with('\n') {
            output.push('\n');
        }
        output
    }

    fn reload_ols() -> Result<(), String> {
        let output = Command::new("/usr/local/lsws/bin/lswsctrl")
            .arg("restart")
            .output()
            .map_err(|e| format!("OLS reload failed: {}", e))?;

        if output.status.success() {
            Ok(())
        } else {
            Err(String::from_utf8_lossy(&output.stderr).trim().to_string())
        }
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
            content = Self::upsert_kv_line(
                &content,
                "smtpd_tls_cert_file",
                &cert_file.to_string_lossy(),
            );
            content =
                Self::upsert_kv_line(&content, "smtpd_tls_key_file", &key_file.to_string_lossy());
            content =
                Self::upsert_kv_line(&content, "smtp_tls_cert_file", &cert_file.to_string_lossy());
            content =
                Self::upsert_kv_line(&content, "smtp_tls_key_file", &key_file.to_string_lossy());
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

        let _ = Command::new("systemctl")
            .args(["reload", "postfix"])
            .output();
        let _ = Command::new("systemctl")
            .args(["restart", "dovecot"])
            .output();
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
