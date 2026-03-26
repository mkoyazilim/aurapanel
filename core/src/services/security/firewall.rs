use anyhow::Result;
use std::process::Command;

pub struct FirewallManager;

impl FirewallManager {
    pub fn new() -> Self {
        Self {}
    }

    pub fn block_ip(&self, ip: &str) -> Result<bool> {
        let output = Command::new("nft")
            .arg("add")
            .arg("element")
            .arg("inet")
            .arg("filter")
            .arg("blacklist")
            .arg(format!("{{ {} }}", ip))
            .output()?;
            
        Ok(output.status.success())
    }

    pub fn unblock_ip(&self, ip: &str) -> Result<bool> {
        let output = Command::new("nft")
            .arg("delete")
            .arg("element")
            .arg("inet")
            .arg("filter")
            .arg("blacklist")
            .arg(format!("{{ {} }}", ip))
            .output()?;
            
        Ok(output.status.success())
    }
}
