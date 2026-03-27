use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct PanelUser {
    pub id: u64,
    pub username: String,
    pub email: String,
    pub role: String,
    pub package: String,
    pub sites: u32,
    pub active: bool,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct CreateUserRequest {
    pub username: String,
    pub email: String,
    pub password: String,
    pub role: String,
    pub package: String,
}

pub struct UserManager;

impl UserManager {
    pub fn list_users() -> Result<Vec<PanelUser>, String> {
        let path = users_db_path();
        if !path.exists() {
            return Ok(Vec::new());
        }

        let json_str = fs::read_to_string(path).map_err(|e| format!("Kullanici listesi okunamadi: {}", e))?;
        serde_json::from_str(&json_str).map_err(|e| format!("Kullanici listesi parse edilemedi: {}", e))
    }

    pub fn create_user(req: &CreateUserRequest) -> Result<String, String> {
        let username = sanitize_username(&req.username).ok_or_else(|| "Gecerli username zorunludur".to_string())?;
        let email = req.email.trim().to_ascii_lowercase();
        if email.is_empty() || !email.contains('@') {
            return Err("Gecerli email zorunludur".to_string());
        }

        let role = normalize_role(&req.role);
        let package = if req.package.trim().is_empty() {
            "default".to_string()
        } else {
            req.package.trim().to_string()
        };

        let mut users = Self::list_users()?;
        if users.iter().any(|u| u.username == username) {
            return Err(format!("Kullanici '{}' zaten mevcut.", username));
        }

        let new_id = users.iter().map(|u| u.id).max().unwrap_or(0) + 1;
        users.push(PanelUser {
            id: new_id,
            username: username.clone(),
            email,
            role,
            package,
            sites: 0,
            active: true,
        });
        save_users(&users)?;

        if !cfg!(windows) {
            let _ = std::process::Command::new("useradd")
                .args(["-m", "-s", "/bin/bash", &username])
                .output();
        }

        Ok(format!("Kullanici '{}' basariyla olusturuldu.", username))
    }

    pub fn delete_user(username: &str) -> Result<String, String> {
        let username = sanitize_username(username).ok_or_else(|| "Gecerli username zorunludur".to_string())?;

        let mut users = Self::list_users()?;
        let before = users.len();
        users.retain(|u| u.username != username);
        if users.len() == before {
            return Err(format!("Kullanici '{}' bulunamadi.", username));
        }
        save_users(&users)?;

        if !cfg!(windows) {
            let _ = std::process::Command::new("userdel")
                .args(["-r", &username])
                .output();
        }

        Ok(format!("Kullanici '{}' basariyla silindi.", username))
    }
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

fn users_db_path() -> PathBuf {
    state_root().join("users.json")
}

fn save_users(users: &[PanelUser]) -> Result<(), String> {
    let path = users_db_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| format!("Dizin olusturulamadi: {}", e))?;
    }

    let json = serde_json::to_string_pretty(users)
        .map_err(|e| format!("JSON hatasi: {}", e))?;
    fs::write(path, json).map_err(|e| format!("Dosya yazilamadi: {}", e))
}

fn sanitize_username(input: &str) -> Option<String> {
    let cleaned = input
        .trim()
        .to_ascii_lowercase()
        .chars()
        .filter(|c| c.is_ascii_alphanumeric() || *c == '_' || *c == '-')
        .collect::<String>();

    if cleaned.is_empty() || cleaned.len() > 64 {
        None
    } else {
        Some(cleaned)
    }
}

fn normalize_role(role: &str) -> String {
    let role = role.trim().to_ascii_lowercase();
    match role.as_str() {
        "admin" | "reseller" => role,
        _ => "user".to_string(),
    }
}
