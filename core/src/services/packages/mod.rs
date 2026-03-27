use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct HostingPackage {
    pub id: u64,
    pub name: String,
    pub plan_type: String,
    pub disk_gb: u32,
    pub bandwidth_gb: u32,
    pub domains: u32,
    pub databases: u32,
    pub emails: u32,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct CreatePackageRequest {
    pub name: String,
    pub plan_type: String,
    pub disk_gb: u32,
    pub bandwidth_gb: u32,
    pub domains: u32,
    pub databases: u32,
    pub emails: u32,
}

pub struct PackageManager;

impl PackageManager {
    pub fn list_packages() -> Result<Vec<HostingPackage>, String> {
        let path = packages_db_path();
        if !path.exists() {
            return Ok(Vec::new());
        }

        let json_str = fs::read_to_string(path)
            .map_err(|e| format!("Paket listesi okunamadi: {}", e))?;
        serde_json::from_str(&json_str)
            .map_err(|e| format!("Paket listesi parse edilemedi: {}", e))
    }

    pub fn create_package(req: &CreatePackageRequest) -> Result<String, String> {
        if req.name.trim().is_empty() {
            return Err("Paket adi zorunludur".to_string());
        }

        let plan_type = normalize_plan_type(&req.plan_type);
        let mut packages = Self::list_packages()?;
        if packages.iter().any(|p| p.name.eq_ignore_ascii_case(req.name.trim())) {
            return Err(format!("Paket '{}' zaten mevcut.", req.name.trim()));
        }

        let new_id = packages.iter().map(|p| p.id).max().unwrap_or(0) + 1;
        packages.push(HostingPackage {
            id: new_id,
            name: req.name.trim().to_string(),
            plan_type,
            disk_gb: req.disk_gb,
            bandwidth_gb: req.bandwidth_gb,
            domains: req.domains,
            databases: req.databases,
            emails: req.emails,
        });

        save_packages(&packages)?;
        Ok(format!("Paket '{}' basariyla olusturuldu.", req.name.trim()))
    }

    pub fn delete_package(id: u64) -> Result<String, String> {
        let mut packages = Self::list_packages()?;
        let before = packages.len();
        packages.retain(|p| p.id != id);
        if packages.len() == before {
            return Err(format!("Paket #{} bulunamadi.", id));
        }

        save_packages(&packages)?;
        Ok(format!("Paket #{} basariyla silindi.", id))
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

fn packages_db_path() -> PathBuf {
    state_root().join("packages.json")
}

fn save_packages(packages: &[HostingPackage]) -> Result<(), String> {
    let path = packages_db_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .map_err(|e| format!("Dizin olusturulamadi: {}", e))?;
    }

    let json = serde_json::to_string_pretty(packages)
        .map_err(|e| format!("JSON hatasi: {}", e))?;
    fs::write(path, json)
        .map_err(|e| format!("Dosya yazilamadi: {}", e))
}

fn normalize_plan_type(value: &str) -> String {
    let cleaned = value.trim().to_ascii_lowercase();
    match cleaned.as_str() {
        "reseller" => "reseller".to_string(),
        _ => "hosting".to_string(),
    }
}
