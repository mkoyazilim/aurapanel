use anyhow::Result;
use std::process::Command;

pub struct OlsManager;

impl OlsManager {
    pub fn new() -> Self {
        Self {}
    }

    pub fn restart_graceful(&self) -> Result<bool> {
        let output = Command::new("systemctl")
            .arg("reload")
            .arg("lsws")
            .output()?;
        Ok(output.status.success())
    }

    pub fn hard_restart(&self) -> Result<bool> {
        let output = Command::new("systemctl")
            .arg("restart")
            .arg("lsws")
            .output()?;
        Ok(output.status.success())
    }

    pub fn get_version(&self) -> Result<String> {
        let output = Command::new("/usr/local/lsws/bin/lshttpd")
            .arg("-v")
            .output()?;
        Ok(String::from_utf8_lossy(&output.stdout).trim().to_string())
    }
}
