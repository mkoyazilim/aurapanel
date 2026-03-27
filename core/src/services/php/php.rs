use serde::{Deserialize, Serialize};
use std::fs;
use std::path::Path;
use std::process::Command;

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct PhpVersion {
    pub version: String,
    pub installed: bool,
    pub eol: bool,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct PhpExtension {
    pub name: String,
    pub enabled: bool,
}

pub struct PhpManager;

impl PhpManager {
    pub fn list_versions() -> Result<Vec<PhpVersion>, String> {
        let mut versions = vec![
            PhpVersion { version: "7.4".to_string(), installed: false, eol: true },
            PhpVersion { version: "8.0".to_string(), installed: false, eol: true },
            PhpVersion { version: "8.1".to_string(), installed: false, eol: false },
            PhpVersion { version: "8.2".to_string(), installed: false, eol: false },
            PhpVersion { version: "8.3".to_string(), installed: false, eol: false },
            PhpVersion { version: "8.4".to_string(), installed: false, eol: false },
        ];

        for v in &mut versions {
            if Self::is_php_installed(&v.version) {
                v.installed = true;
            }
        }

        Ok(versions)
    }

    pub fn install_version(version: &str) -> Result<String, String> {
        let version = normalize_version(version)?;
        ensure_linux("PHP install is supported only on Linux hosts.")?;

        let pkg = format!("php{}-fpm", version);
        let output = Command::new("apt")
            .args(["install", "-y", &pkg])
            .output()
            .map_err(|e| format!("apt install failed: {}", e))?;

        if output.status.success() {
            Ok(format!("PHP {} installed successfully.", version))
        } else {
            Err(String::from_utf8_lossy(&output.stderr).trim().to_string())
        }
    }

    pub fn remove_version(version: &str) -> Result<String, String> {
        let version = normalize_version(version)?;
        ensure_linux("PHP remove is supported only on Linux hosts.")?;

        let pkg = format!("php{}-fpm", version);
        let output = Command::new("apt")
            .args(["purge", "-y", &pkg])
            .output()
            .map_err(|e| format!("apt purge failed: {}", e))?;

        if output.status.success() {
            Ok(format!("PHP {} removed successfully.", version))
        } else {
            Err(String::from_utf8_lossy(&output.stderr).trim().to_string())
        }
    }

    pub fn restart_fpm(version: &str) -> Result<String, String> {
        let version = normalize_version(version)?;
        ensure_linux("PHP restart is supported only on Linux hosts.")?;

        let srv = format!("php{}-fpm", version);
        let output = Command::new("systemctl")
            .args(["restart", &srv])
            .output()
            .map_err(|e| format!("systemctl restart failed: {}", e))?;

        if output.status.success() {
            Ok(format!("PHP {} FPM restarted.", version))
        } else {
            Err(String::from_utf8_lossy(&output.stderr).trim().to_string())
        }
    }

    pub fn get_ini(version: &str) -> Result<String, String> {
        let version = normalize_version(version)?;
        let path = Self::resolve_ini_path(&version)
            .ok_or_else(|| format!("php.ini not found for PHP {}", version))?;

        fs::read_to_string(&path)
            .map_err(|e| format!("php.ini read failed ({}): {}", path.display(), e))
    }

    pub fn save_ini(version: &str, content: &str) -> Result<String, String> {
        let version = normalize_version(version)?;
        let path = Self::resolve_ini_path(&version)
            .ok_or_else(|| format!("php.ini not found for PHP {}", version))?;

        fs::write(&path, content)
            .map_err(|e| format!("php.ini write failed ({}): {}", path.display(), e))?;

        let _ = Self::restart_fpm(&version);
        Ok(format!("PHP {} php.ini saved.", version))
    }

    fn is_php_installed(version: &str) -> bool {
        let cli_path = format!("/usr/bin/php{}", version);
        if Path::new(&cli_path).exists() {
            return true;
        }

        let fpm_service = format!("php{}-fpm", version);
        let status = Command::new("systemctl")
            .args(["status", &fpm_service])
            .output();
        if let Ok(output) = status {
            if output.status.success() {
                return true;
            }
        }

        let lsws_version = version.replace('.', "");
        let lsphp_path = format!("/usr/local/lsws/lsphp{}/bin/lsphp", lsws_version);
        Path::new(&lsphp_path).exists()
    }

    fn resolve_ini_path(version: &str) -> Option<std::path::PathBuf> {
        let candidates = [
            format!("/etc/php/{}/fpm/php.ini", version),
            format!("/usr/local/lsws/lsphp{}/etc/php/{}/litespeed/php.ini", version.replace('.', ""), version),
            format!("/usr/local/lsws/lsphp{}/etc/php.ini", version.replace('.', "")),
        ];

        for candidate in candidates {
            let path = std::path::PathBuf::from(candidate);
            if path.exists() {
                return Some(path);
            }
        }
        None
    }
}

fn normalize_version(version: &str) -> Result<String, String> {
    let cleaned = version.trim();
    let parts: Vec<&str> = cleaned.split('.').collect();
    if parts.len() != 2 {
        return Err("version must be in major.minor format (e.g. 8.3)".to_string());
    }
    if parts.iter().any(|p| p.is_empty() || p.chars().any(|c| !c.is_ascii_digit())) {
        return Err("version must be numeric in major.minor format".to_string());
    }
    Ok(format!("{}.{}", parts[0], parts[1]))
}

fn ensure_linux(message: &str) -> Result<(), String> {
    if cfg!(windows) {
        return Err(message.to_string());
    }
    Ok(())
}
