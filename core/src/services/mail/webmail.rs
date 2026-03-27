use std::collections::hash_map::DefaultHasher;
use std::hash::{Hash, Hasher};
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

pub struct WebmailManager;

impl WebmailManager {
    pub fn install_roundcube(
        domain: &str,
        db_host: &str,
        db_name: &str,
        db_user: &str,
        db_pass: &str,
    ) -> Result<(), String> {
        if cfg!(target_os = "windows") {
            return Err("Roundcube installation is supported only on Linux hosts.".to_string());
        }

        if domain.trim().is_empty()
            || db_host.trim().is_empty()
            || db_name.trim().is_empty()
            || db_user.trim().is_empty()
            || db_pass.trim().is_empty()
        {
            return Err("domain, db_host, db_name, db_user and db_pass are required.".to_string());
        }

        let webmail_dir = "/usr/local/lsws/Example/html/webmail";
        let clone = Command::new("git")
            .args([
                "clone",
                "--depth",
                "1",
                "https://github.com/roundcube/roundcubemail.git",
                webmail_dir,
            ])
            .output()
            .map_err(|e| format!("Roundcube clone failed: {}", e))?;
        if !clone.status.success() {
            return Err(String::from_utf8_lossy(&clone.stderr).trim().to_string());
        }

        let config_content = format!(
            r#"<?php
$config['db_dsnw'] = 'mysql://{}:{}@{}/{}';
$config['default_host'] = 'ssl://127.0.0.1';
$config['default_port'] = 993;
$config['smtp_server'] = 'tls://127.0.0.1';
$config['smtp_port'] = 587;
$config['product_name'] = 'AuraPanel Webmail';
$config['des_key'] = '{}';
$config['plugins'] = ['archive', 'zipdownload', 'markasjunk'];
?>"#,
            db_user,
            db_pass,
            db_host,
            db_name,
            random_des_key(),
        );

        let config_path = format!("{}/config/config.inc.php", webmail_dir);
        std::fs::write(&config_path, config_content)
            .map_err(|e| format!("Roundcube config write failed: {}", e))?;

        Ok(())
    }
}

fn random_des_key() -> String {
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos();
    let mut hasher = DefaultHasher::new();
    now.hash(&mut hasher);
    std::process::id().hash(&mut hasher);
    let seed = hasher.finish();

    let mut out = String::new();
    let mut counter = 0_u64;
    while out.len() < 32 {
        let mut chunk_hasher = DefaultHasher::new();
        seed.hash(&mut chunk_hasher);
        counter.hash(&mut chunk_hasher);
        out.push_str(&format!("{:016x}", chunk_hasher.finish()));
        counter += 1;
    }

    out.chars().take(32).collect()
}
