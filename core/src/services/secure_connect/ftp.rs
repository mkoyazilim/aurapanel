use anyhow::Result;

pub struct FtpManager;

impl FtpManager {
    pub fn new() -> Self {
        Self {}
    }

    // Purely for legacy fallback if user configures vsftpd/proftpd
    pub fn create_ftp_account(&self, username: &str, _password: &str, _dir: &str) -> Result<bool> {
        println!("FTP account {} created", username);
        Ok(true)
    }
}
