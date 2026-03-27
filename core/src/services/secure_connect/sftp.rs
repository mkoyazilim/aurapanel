use anyhow::Result;
use std::fs;
use std::path::PathBuf;
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

pub struct SftpManager;

impl SftpManager {
    pub fn new() -> Self {
        Self {}
    }

    pub fn setup_chroot_user(&self, username: &str, home_dir: &str) -> Result<bool> {
        let useradd = Command::new("useradd")
            .arg("-d")
            .arg(home_dir)
            .arg("-s")
            .arg("/bin/false")
            .arg(username)
            .output()?;
        
        Ok(useradd.status.success())
    }

    pub fn generate_ssh_key(&self, username: &str) -> Result<String> {
        let safe_user = username.trim().to_ascii_lowercase();
        if safe_user.is_empty() {
            anyhow::bail!("username is required");
        }

        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_nanos();
        let base_dir = PathBuf::from("/tmp/aurapanel-sshkeys");
        fs::create_dir_all(&base_dir)?;

        let key_path = base_dir.join(format!("{}_{}", safe_user, nanos));
        let key_path_str = key_path.to_string_lossy().to_string();

        let output = Command::new("ssh-keygen")
            .args(["-t", "ed25519", "-N", "", "-f", &key_path_str, "-C", &safe_user])
            .output()?;
        if !output.status.success() {
            anyhow::bail!(
                "ssh-keygen failed: {}",
                String::from_utf8_lossy(&output.stderr).trim()
            );
        }

        let pub_key_path = key_path.with_extension("pub");
        let pub_key = fs::read_to_string(pub_key_path)?;
        Ok(pub_key.trim().to_string())
    }
}
