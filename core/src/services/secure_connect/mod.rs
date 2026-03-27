use serde::{Deserialize, Serialize};
use std::fs;
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::time::{SystemTime, UNIX_EPOCH};

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct SftpUserConfig {
    pub username: String,
    pub password: String,
    pub home_dir: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct SftpUserRecord {
    pub username: String,
    pub home_dir: String,
    pub created_at: u64,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct SftpPasswordResetRequest {
    pub username: String,
    pub new_password: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct FtpUserConfig {
    pub username: String,
    pub password: String,
    pub home_dir: String,
    pub domain: Option<String>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct FtpUserRecord {
    pub username: String,
    pub domain: Option<String>,
    pub home_dir: String,
    pub created_at: u64,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct FtpPasswordResetRequest {
    pub username: String,
    pub new_password: String,
}

fn state_root() -> PathBuf {
    if let Ok(path) = std::env::var("AURAPANEL_STATE_DIR") {
        let p = PathBuf::from(path.trim());
        if !p.as_os_str().is_empty() {
            return p;
        }
    }
    let prod = Path::new("/var/lib/aurapanel");
    if prod.exists() {
        prod.to_path_buf()
    } else {
        std::env::temp_dir().join("aurapanel")
    }
}

fn sftp_state_path() -> PathBuf {
    state_root().join("sftp_users.json")
}

fn ftp_state_path() -> PathBuf {
    state_root().join("ftp_users.json")
}

fn now_ts() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0)
}

fn normalize_username(input: &str) -> Option<String> {
    let value = input
        .trim()
        .to_ascii_lowercase()
        .chars()
        .filter(|c| c.is_ascii_alphanumeric() || *c == '_' || *c == '-')
        .collect::<String>();

    if value.is_empty() || value.len() > 64 {
        return None;
    }
    Some(value)
}

fn normalize_domain(input: Option<&str>) -> Option<String> {
    let raw = input?.trim().trim_end_matches('.').to_ascii_lowercase();
    if raw.is_empty() { None } else { Some(raw) }
}

fn load_sftp_users() -> Result<Vec<SftpUserRecord>, String> {
    let path = sftp_state_path();
    if !path.exists() {
        return Ok(Vec::new());
    }
    let raw = fs::read_to_string(path).map_err(|e| e.to_string())?;
    serde_json::from_str(&raw).map_err(|e| e.to_string())
}

fn save_sftp_users(users: &[SftpUserRecord]) -> Result<(), String> {
    let path = sftp_state_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| e.to_string())?;
    }
    let payload = serde_json::to_string_pretty(users).map_err(|e| e.to_string())?;
    fs::write(path, payload).map_err(|e| e.to_string())
}

fn load_ftp_users() -> Result<Vec<FtpUserRecord>, String> {
    let path = ftp_state_path();
    if !path.exists() {
        return Ok(Vec::new());
    }
    let raw = fs::read_to_string(path).map_err(|e| e.to_string())?;
    serde_json::from_str(&raw).map_err(|e| e.to_string())
}

fn save_ftp_users(users: &[FtpUserRecord]) -> Result<(), String> {
    let path = ftp_state_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| e.to_string())?;
    }
    let payload = serde_json::to_string_pretty(users).map_err(|e| e.to_string())?;
    fs::write(path, payload).map_err(|e| e.to_string())
}

fn pure_pw_exists() -> bool {
    Path::new("/usr/bin/pure-pw").exists()
        || Path::new("/usr/sbin/pure-pw").exists()
        || Path::new("/usr/local/bin/pure-pw").exists()
}

fn purepw_path() -> &'static str {
    if Path::new("/usr/local/bin/pure-pw").exists() {
        "/usr/local/bin/pure-pw"
    } else if Path::new("/usr/sbin/pure-pw").exists() {
        "/usr/sbin/pure-pw"
    } else {
        "/usr/bin/pure-pw"
    }
}

fn purepw_file() -> String {
    std::env::var("AURAPANEL_PUREPW_FILE").unwrap_or_else(|_| "/etc/pure-ftpd/pureftpd.passwd".to_string())
}

fn purepdb_file() -> String {
    std::env::var("AURAPANEL_PUREPDB_FILE").unwrap_or_else(|_| "/etc/pure-ftpd/pureftpd.pdb".to_string())
}

fn ensure_purepw_backend() -> Result<(), String> {
    if !pure_pw_exists() {
        return Err("pure-pw binary not found. Install pure-ftpd package first.".to_string());
    }
    let passwd_file = purepw_file();
    if let Some(parent) = Path::new(&passwd_file).parent() {
        fs::create_dir_all(parent).map_err(|e| e.to_string())?;
    }
    if !Path::new(&passwd_file).exists() {
        fs::write(&passwd_file, "").map_err(|e| e.to_string())?;
    }
    Ok(())
}

fn purepw_mkdb() -> Result<(), String> {
    let status = Command::new(purepw_path())
        .args(["mkdb", &purepdb_file(), "-f", &purepw_file()])
        .status()
        .map_err(|e| format!("pure-pw mkdb failed: {}", e))?;

    if status.success() {
        Ok(())
    } else {
        Err("pure-pw mkdb command failed.".to_string())
    }
}

fn run_purepw_with_password(args: &[String], password: &str) -> Result<(), String> {
    let mut child = Command::new(purepw_path())
        .args(args)
        .stdin(Stdio::piped())
        .stdout(Stdio::null())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|e| format!("pure-pw spawn failed: {}", e))?;

    if let Some(stdin) = child.stdin.as_mut() {
        let payload = format!("{0}\n{0}\n", password.trim());
        stdin
            .write_all(payload.as_bytes())
            .map_err(|e| format!("pure-pw stdin failed: {}", e))?;
    }

    let output = child
        .wait_with_output()
        .map_err(|e| format!("pure-pw wait failed: {}", e))?;

    if output.status.success() {
        Ok(())
    } else {
        Err(String::from_utf8_lossy(&output.stderr).trim().to_string())
    }
}

fn run_purepw(args: &[String]) -> Result<(), String> {
    let output = Command::new(purepw_path())
        .args(args)
        .output()
        .map_err(|e| format!("pure-pw failed: {}", e))?;
    if output.status.success() {
        Ok(())
    } else {
        Err(String::from_utf8_lossy(&output.stderr).trim().to_string())
    }
}

fn default_ftp_home(domain: Option<&str>, username: &str) -> String {
    match normalize_domain(domain) {
        Some(d) => format!("/home/{}/public_html", d),
        None => format!("/home/{}", username),
    }
}

fn default_ftp_uid() -> String {
    std::env::var("AURAPANEL_FTP_UID")
        .ok()
        .filter(|x| !x.trim().is_empty())
        .unwrap_or_else(|| "33".to_string())
}

fn default_ftp_gid() -> String {
    std::env::var("AURAPANEL_FTP_GID")
        .ok()
        .filter(|x| !x.trim().is_empty())
        .unwrap_or_else(|| "33".to_string())
}

pub struct SecureConnectManager;

impl SecureConnectManager {
    pub async fn create_sftp_user(config: &SftpUserConfig) -> Result<(), String> {
        let username = config.username.trim().to_string();
        let password = config.password.trim().to_string();
        let home_dir = config.home_dir.trim().to_string();

        if username.is_empty() || password.is_empty() || home_dir.is_empty() {
            return Err("username, password and home_dir are required.".to_string());
        }

        let _ = Command::new("useradd")
            .args([
                "-m",
                "-d",
                &home_dir,
                "-s",
                "/usr/sbin/nologin",
                "-G",
                "sftponly",
                &username,
            ])
            .output()
            .map_err(|e| format!("useradd failed: {}", e))?;

        let _ = Command::new("sh")
            .args(["-c", &format!("echo '{}:{}' | chpasswd", username, password)])
            .output()
            .map_err(|e| format!("chpasswd failed: {}", e))?;

        let _ = Command::new("chown").args(["root:root", &home_dir]).output();
        let _ = Command::new("chmod").args(["755", &home_dir]).output();

        let mut users = load_sftp_users()?;
        if !users.iter().any(|u| u.username == username) {
            users.push(SftpUserRecord {
                username,
                home_dir,
                created_at: now_ts(),
            });
            save_sftp_users(&users)?;
        }
        Ok(())
    }

    pub fn list_sftp_users() -> Result<Vec<SftpUserRecord>, String> {
        load_sftp_users()
    }

    pub fn delete_sftp_user(username: &str) -> Result<(), String> {
        let username = username.trim();
        if username.is_empty() {
            return Err("username is required.".to_string());
        }

        let _ = Command::new("userdel").args(["-r", username]).output();

        let mut users = load_sftp_users()?;
        let before = users.len();
        users.retain(|u| u.username != username);
        if before == users.len() {
            return Err("SFTP user not found.".to_string());
        }
        save_sftp_users(&users)
    }

    pub fn reset_sftp_password(req: &SftpPasswordResetRequest) -> Result<(), String> {
        let username = req.username.trim();
        let password = req.new_password.trim();
        if username.is_empty() || password.is_empty() {
            return Err("username and new_password are required.".to_string());
        }

        let _ = Command::new("sh")
            .args(["-c", &format!("echo '{}:{}' | chpasswd", username, password)])
            .output()
            .map_err(|e| format!("chpasswd failed: {}", e))?;
        Ok(())
    }

    pub fn create_ftp_user(config: &FtpUserConfig) -> Result<(), String> {
        let username = normalize_username(&config.username)
            .ok_or_else(|| "valid username is required.".to_string())?;
        let password = config.password.trim().to_string();
        if password.is_empty() {
            return Err("password is required.".to_string());
        }

        let domain = normalize_domain(config.domain.as_deref());
        let home_dir = if config.home_dir.trim().is_empty() {
            default_ftp_home(domain.as_deref(), &username)
        } else {
            config.home_dir.trim().to_string()
        };

        ensure_purepw_backend()?;

        fs::create_dir_all(&home_dir).map_err(|e| format!("home_dir create failed: {}", e))?;

        let mut args = vec![
            "useradd".to_string(),
            username.clone(),
            "-u".to_string(),
            default_ftp_uid(),
            "-g".to_string(),
            default_ftp_gid(),
            "-d".to_string(),
            home_dir.clone(),
            "-f".to_string(),
            purepw_file(),
            "-m".to_string(),
        ];
        run_purepw_with_password(&args, &password)?;
        purepw_mkdb()?;
        args.clear();

        let mut users = load_ftp_users()?;
        if users.iter().all(|u| u.username != username) {
            users.push(FtpUserRecord {
                username,
                domain,
                home_dir,
                created_at: now_ts(),
            });
            save_ftp_users(&users)?;
        }
        Ok(())
    }

    pub fn list_ftp_users(domain: Option<&str>) -> Result<Vec<FtpUserRecord>, String> {
        let mut users = load_ftp_users()?;
        if let Some(d) = normalize_domain(domain) {
            users.retain(|u| u.domain.as_deref() == Some(d.as_str()));
        }
        users.sort_by(|a, b| a.username.cmp(&b.username));
        Ok(users)
    }

    pub fn delete_ftp_user(username: &str) -> Result<(), String> {
        let username = normalize_username(username).ok_or_else(|| "valid username is required.".to_string())?;
        ensure_purepw_backend()?;

        let args = vec![
            "userdel".to_string(),
            username.clone(),
            "-f".to_string(),
            purepw_file(),
            "-m".to_string(),
        ];
        run_purepw(&args)?;
        purepw_mkdb()?;

        let mut users = load_ftp_users()?;
        let before = users.len();
        users.retain(|u| u.username != username);
        if before == users.len() {
            return Err("FTP user not found.".to_string());
        }
        save_ftp_users(&users)
    }

    pub fn reset_ftp_password(req: &FtpPasswordResetRequest) -> Result<(), String> {
        let username = normalize_username(&req.username).ok_or_else(|| "valid username is required.".to_string())?;
        let password = req.new_password.trim().to_string();
        if password.is_empty() {
            return Err("new_password is required.".to_string());
        }

        ensure_purepw_backend()?;
        let args = vec![
            "passwd".to_string(),
            username,
            "-f".to_string(),
            purepw_file(),
            "-m".to_string(),
        ];
        run_purepw_with_password(&args, &password)?;
        purepw_mkdb()?;
        Ok(())
    }

    pub async fn start_web_terminal(port: u16) -> Result<String, String> {
        if port == 0 {
            return Err("port is required.".to_string());
        }
        let host = std::env::var("AURAPANEL_PANEL_HOST")
            .ok()
            .filter(|x| !x.trim().is_empty())
            .unwrap_or_else(|| "127.0.0.1".to_string());
        Ok(format!("https://{}:{}", host.trim(), port))
    }
}
