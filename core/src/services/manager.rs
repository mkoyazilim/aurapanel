use anyhow::Result;
use std::process::Command;

pub struct ServiceManager;

impl ServiceManager {
    pub fn new() -> Self {
        Self {}
    }

    pub fn start_service(&self, name: &str) -> Result<bool> {
        let output = Command::new("systemctl")
            .arg("start")
            .arg(name)
            .output()?;
        
        Ok(output.status.success())
    }

    pub fn stop_service(&self, name: &str) -> Result<bool> {
        let output = Command::new("systemctl")
            .arg("stop")
            .arg(name)
            .output()?;
        
        Ok(output.status.success())
    }

    pub fn restart_service(&self, name: &str) -> Result<bool> {
        let output = Command::new("systemctl")
            .arg("restart")
            .arg(name)
            .output()?;
        
        Ok(output.status.success())
    }

    pub fn check_status(&self, name: &str) -> Result<String> {
        let output = Command::new("systemctl")
            .arg("is-active")
            .arg(name)
            .output()?;
        
        let status = String::from_utf8_lossy(&output.stdout).trim().to_string();
        Ok(status)
    }
}
