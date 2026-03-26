use anyhow::Result;
use std::process::Command;

pub struct BackupManager;

impl BackupManager {
    pub fn new() -> Self {
        Self {}
    }

    pub fn backup_site(&self, domain: &str, repo_path: &str, password: &str) -> Result<bool> {
        let output = Command::new("restic")
            .arg("-r")
            .arg(repo_path)
            .arg("backup")
            .arg(format!("/var/www/vhosts/{}/html", domain))
            .env("RESTIC_PASSWORD", password)
            .output()?;
        
        Ok(output.status.success())
    }

    pub fn restore_site(&self, domain: &str, repo_path: &str, snapshot_id: &str, password: &str) -> Result<bool> {
        let output = Command::new("restic")
            .arg("-r")
            .arg(repo_path)
            .arg("restore")
            .arg(snapshot_id)
            .arg("--target")
            .arg(format!("/var/www/vhosts/{}/html", domain))
            .env("RESTIC_PASSWORD", password)
            .output()?;
        
        Ok(output.status.success())
    }
}
