use anyhow::Result;
use std::fs;
use std::process::Command;

pub struct CmsInstaller;

impl CmsInstaller {
    pub fn new() -> Self {
        Self {}
    }

    pub fn install_wordpress(&self, domain: &str, _admin_email: &str) -> Result<bool> {
        let target = cms_target_dir(domain)?;
        fs::create_dir_all(&target)?;
        let output = Command::new("wp")
            .args(["core", "download", "--allow-root", "--path"])
            .arg(&target)
            .output()?;
        Ok(output.status.success())
    }

    pub fn install_laravel(&self, domain: &str) -> Result<bool> {
        let target = cms_target_dir(domain)?;
        fs::create_dir_all(&target)?;
        let output = Command::new("composer")
            .args(["create-project", "--prefer-dist", "--no-interaction", "laravel/laravel"])
            .arg(&target)
            .output()?;
        Ok(output.status.success())
    }
}

fn cms_target_dir(domain: &str) -> Result<String> {
    let d = domain.trim().to_ascii_lowercase();
    if d.is_empty() {
        anyhow::bail!("domain is required");
    }
    Ok(format!("/home/{}/public_html", d))
}
