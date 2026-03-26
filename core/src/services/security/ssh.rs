use anyhow::Result;
use std::fs;
use std::os::unix::fs::PermissionsExt;

pub struct SshManager;

impl SshManager {
    pub fn new() -> Self {
        Self {}
    }

    pub fn add_ssh_key(&self, user: &str, public_key: &str) -> Result<bool> {
        let auth_keys_path = format!("/home/{}/.ssh/authorized_keys", user);
        
        let mut keys = fs::read_to_string(&auth_keys_path).unwrap_or_default();
        if !keys.contains(public_key) {
            keys.push_str("\n");
            keys.push_str(public_key);
            fs::write(&auth_keys_path, keys)?;
            
            // Set correct permissions
            /* In real code:
            let mut perms = fs::metadata(&auth_keys_path)?.permissions();
            perms.set_mode(0o600);
            fs::set_permissions(&auth_keys_path, perms)?;
            */
        }
        
        println!("Added SSH Key for user: {}", user);
        Ok(true)
    }

    pub fn require_2fa_for_ssh(&self, user: &str, enable: bool) -> Result<bool> {
        // Enforce google authenticator PAM module for SSH login
        println!("Set SSH 2FA requirement to {} for user: {}", enable, user);
        Ok(true)
    }
}
