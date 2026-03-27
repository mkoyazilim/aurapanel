use serde::{Deserialize, Serialize};
use std::io::Write;
use std::process::{Command, Stdio};

#[derive(Serialize, Deserialize, Debug)]
pub struct GitOpsConfig {
    pub domain: String,
    pub repo_url: String,
    pub branch: String,
    pub deploy_path: String,
    pub webhook_secret: String,
}

pub struct GitOpsManager;

impl GitOpsManager {
    pub async fn deploy(config: &GitOpsConfig) -> Result<(), String> {
        if cfg!(target_os = "windows") {
            return Err("GitOps deployment is supported only on Linux hosts.".to_string());
        }

        if std::path::Path::new(&format!("{}/.git", config.deploy_path)).exists() {
            let output = Command::new("git")
                .current_dir(&config.deploy_path)
                .args(["pull", "origin", &config.branch])
                .output()
                .map_err(|e| format!("git pull failed: {}", e))?;

            if !output.status.success() {
                return Err(format!(
                    "Pull failed: {}",
                    String::from_utf8_lossy(&output.stderr)
                ));
            }
        } else {
            let output = Command::new("git")
                .args(["clone", "-b", &config.branch, &config.repo_url, &config.deploy_path])
                .output()
                .map_err(|e| format!("git clone failed: {}", e))?;

            if !output.status.success() {
                return Err(format!(
                    "Clone failed: {}",
                    String::from_utf8_lossy(&output.stderr)
                ));
            }
        }

        let composer_json = format!("{}/composer.json", config.deploy_path);
        let package_json = format!("{}/package.json", config.deploy_path);

        if std::path::Path::new(&composer_json).exists() {
            let output = Command::new("composer")
                .current_dir(&config.deploy_path)
                .args(["install", "--no-dev", "--optimize-autoloader"])
                .output()
                .map_err(|e| format!("composer install failed: {}", e))?;
            if !output.status.success() {
                return Err(String::from_utf8_lossy(&output.stderr).trim().to_string());
            }
        }

        if std::path::Path::new(&package_json).exists() {
            let output = Command::new("npm")
                .current_dir(&config.deploy_path)
                .args(["ci", "--production"])
                .output()
                .map_err(|e| format!("npm ci failed: {}", e))?;
            if !output.status.success() {
                return Err(String::from_utf8_lossy(&output.stderr).trim().to_string());
            }
        }

        Ok(())
    }

    pub fn verify_signature(payload: &[u8], signature: &str, secret: &str) -> bool {
        let sig = signature.trim();
        let sec = secret.trim();
        if sig.is_empty() || sec.is_empty() {
            return false;
        }

        let provided = sig.strip_prefix("sha256=").unwrap_or(sig).to_ascii_lowercase();
        if provided.is_empty() {
            return false;
        }

        let mut child = match Command::new("openssl")
            .args(["dgst", "-sha256", "-hmac", sec, "-binary"])
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .spawn()
        {
            Ok(c) => c,
            Err(_) => return false,
        };

        if let Some(stdin) = child.stdin.as_mut() {
            if stdin.write_all(payload).is_err() {
                return false;
            }
        }

        let output = match child.wait_with_output() {
            Ok(o) if o.status.success() => o,
            _ => return false,
        };

        let computed = bytes_to_hex(&output.stdout);
        secure_eq(computed.as_bytes(), provided.as_bytes())
    }
}

fn bytes_to_hex(bytes: &[u8]) -> String {
    let mut out = String::with_capacity(bytes.len() * 2);
    for b in bytes {
        out.push_str(&format!("{:02x}", b));
    }
    out
}

fn secure_eq(a: &[u8], b: &[u8]) -> bool {
    if a.len() != b.len() {
        return false;
    }
    let mut diff = 0u8;
    for (x, y) in a.iter().zip(b.iter()) {
        diff |= x ^ y;
    }
    diff == 0
}
