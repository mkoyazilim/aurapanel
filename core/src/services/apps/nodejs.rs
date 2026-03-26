use anyhow::Result;
use std::process::Command;

pub struct NodeManager;

impl NodeManager {
    pub fn new() -> Self {
        Self {}
    }

    pub fn start_app(&self, app_name: &str, start_script: &str, dir: &str) -> Result<bool> {
        let output = Command::new("pm2")
            .arg("start")
            .arg(start_script)
            .arg("--name")
            .arg(app_name)
            .current_dir(dir)
            .output()?;
        
        Ok(output.status.success())
    }

    pub fn stop_app(&self, app_name: &str) -> Result<bool> {
        let output = Command::new("pm2")
            .arg("stop")
            .arg(app_name)
            .output()?;
        
        Ok(output.status.success())
    }

    pub fn install_dependencies(&self, dir: &str) -> Result<bool> {
        let output = Command::new("npm")
            .arg("install")
            .current_dir(dir)
            .output()?;
            
        Ok(output.status.success())
    }
}
