pub mod backup;
pub mod minio;

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MinioBucketRequest {
    pub bucket_name: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MinioCredentialsRequest {
    pub user: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MinioCredentialsResponse {
    pub access_key: String,
    pub secret_key: String,
}

#[derive(Serialize, Deserialize, Debug, Clone, Default)]
struct StorageState {
    #[serde(default)]
    buckets: Vec<String>,
    #[serde(default)]
    credentials: HashMap<String, MinioCredentialsResponse>,
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

fn storage_state_path() -> PathBuf {
    state_root().join("storage_state.json")
}

fn load_storage_state() -> Result<StorageState, String> {
    let path = storage_state_path();
    if !path.exists() {
        return Ok(StorageState::default());
    }
    let raw = fs::read_to_string(path).map_err(|e| e.to_string())?;
    serde_json::from_str(&raw).map_err(|e| e.to_string())
}

fn save_storage_state(state: &StorageState) -> Result<(), String> {
    let path = storage_state_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| e.to_string())?;
    }
    let payload = serde_json::to_string_pretty(state).map_err(|e| e.to_string())?;
    fs::write(path, payload).map_err(|e| e.to_string())
}

fn secure_random_hex(byte_count: usize) -> String {
    if cfg!(unix) && Path::new("/dev/urandom").exists() {
        if let Ok(bytes) = fs::read("/dev/urandom") {
            if bytes.len() >= byte_count {
                return bytes[..byte_count]
                    .iter()
                    .map(|b| format!("{:02x}", b))
                    .collect::<String>();
            }
        }
    }

    let seed = format!(
        "{}:{}:{}",
        std::process::id(),
        std::thread::current().name().unwrap_or("main"),
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_nanos()
    );
    let mut out = String::new();
    while out.len() < byte_count * 2 {
        out.push_str(&format!("{:x}", fxhash(&format!("{}:{}", seed, out.len()))));
    }
    out.chars().take(byte_count * 2).collect()
}

fn fxhash(input: &str) -> u64 {
    use std::collections::hash_map::DefaultHasher;
    use std::hash::{Hash, Hasher};
    let mut h = DefaultHasher::new();
    input.hash(&mut h);
    h.finish()
}

fn bucket_name_valid(bucket_name: &str) -> bool {
    let b = bucket_name.trim();
    if b.len() < 3 || b.len() > 63 {
        return false;
    }
    b.chars()
        .all(|c| c.is_ascii_lowercase() || c.is_ascii_digit() || c == '-' || c == '.')
}

fn command_exists(command: &str) -> bool {
    Command::new("sh")
        .args(["-c", &format!("command -v {} >/dev/null 2>&1", command)])
        .output()
        .map(|o| o.status.success())
        .unwrap_or(false)
}

fn run_command(program: &str, args: &[&str]) -> Result<(), String> {
    let output = Command::new(program)
        .args(args)
        .output()
        .map_err(|e| format!("{} failed to execute: {}", program, e))?;
    if output.status.success() {
        Ok(())
    } else {
        Err(String::from_utf8_lossy(&output.stderr).to_string())
    }
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct BackupConfig {
    pub domain: String,
    pub backup_path: String,
    #[serde(default)]
    pub remote_repo: String,
    #[serde(default)]
    pub password: String,
    pub incremental: Option<bool>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct BackupDestination {
    pub id: String,
    pub name: String,
    pub remote_repo: String,
    #[serde(default)]
    pub password: String,
    #[serde(default)]
    pub enabled: bool,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct BackupSchedule {
    pub id: String,
    pub domain: String,
    pub destination_id: String,
    pub backup_path: String,
    pub cron: String,
    #[serde(default)]
    pub incremental: bool,
    #[serde(default)]
    pub enabled: bool,
}

#[derive(Serialize, Deserialize, Debug, Clone, Default)]
struct BackupCenterState {
    #[serde(default)]
    destinations: Vec<BackupDestination>,
    #[serde(default)]
    schedules: Vec<BackupSchedule>,
}

fn backup_center_state_path() -> PathBuf {
    state_root().join("backup_center.json")
}

fn load_backup_center_state() -> Result<BackupCenterState, String> {
    let path = backup_center_state_path();
    if !path.exists() {
        return Ok(BackupCenterState::default());
    }
    let raw = fs::read_to_string(path).map_err(|e| e.to_string())?;
    serde_json::from_str(&raw).map_err(|e| e.to_string())
}

fn save_backup_center_state(state: &BackupCenterState) -> Result<(), String> {
    let path = backup_center_state_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| e.to_string())?;
    }
    let payload = serde_json::to_string_pretty(state).map_err(|e| e.to_string())?;
    fs::write(path, payload).map_err(|e| e.to_string())
}

pub struct BackupManager;

impl BackupManager {
    fn default_target() -> String {
        std::env::var("AURAPANEL_BACKUP_TARGET")
            .unwrap_or_else(|_| "internal-minio".to_string())
            .trim()
            .to_ascii_lowercase()
    }

    fn resolve_backup_repo(config: &BackupConfig) -> Result<String, String> {
        let explicit = config.remote_repo.trim();
        if !explicit.is_empty() {
            return Ok(explicit.to_string());
        }

        let target = Self::default_target();
        if target != "internal-minio" {
            return Err("remote_repo is required for non-internal-minio backup target.".to_string());
        }

        let endpoint = std::env::var("AURAPANEL_BACKUP_MINIO_ENDPOINT")
            .unwrap_or_else(|_| "http://127.0.0.1:9000".to_string())
            .trim()
            .trim_end_matches('/')
            .to_string();
        let bucket = std::env::var("AURAPANEL_BACKUP_MINIO_BUCKET")
            .unwrap_or_else(|_| "aurapanel-backups".to_string())
            .trim()
            .to_string();

        if endpoint.is_empty() || bucket.is_empty() {
            return Err("internal-minio target requires endpoint and bucket configuration.".to_string());
        }

        Ok(format!("s3:{}/{}/{}", endpoint, bucket, config.domain.trim()))
    }

    fn resolve_backup_password(config: &BackupConfig) -> Result<String, String> {
        let explicit = config.password.trim();
        if !explicit.is_empty() {
            return Ok(explicit.to_string());
        }

        let from_env = std::env::var("AURAPANEL_BACKUP_RESTIC_PASSWORD").unwrap_or_default();
        if !from_env.trim().is_empty() {
            return Ok(from_env.trim().to_string());
        }

        Err("Backup password is required. Provide payload.password or AURAPANEL_BACKUP_RESTIC_PASSWORD.".to_string())
    }

    fn resolve_minio_env() -> Result<Option<(String, String)>, String> {
        if Self::default_target() != "internal-minio" {
            return Ok(None);
        }

        let access_key = std::env::var("AURAPANEL_BACKUP_MINIO_ACCESS_KEY").unwrap_or_default();
        let secret_key = std::env::var("AURAPANEL_BACKUP_MINIO_SECRET_KEY").unwrap_or_default();

        if access_key.trim().is_empty() || secret_key.trim().is_empty() {
            return Err(
                "Internal MinIO backup target requires AURAPANEL_BACKUP_MINIO_ACCESS_KEY and AURAPANEL_BACKUP_MINIO_SECRET_KEY."
                    .to_string(),
            );
        }

        Ok(Some((access_key.trim().to_string(), secret_key.trim().to_string())))
    }

    fn validate_backup_input(config: &BackupConfig) -> Result<(), String> {
        if config.domain.trim().is_empty() {
            return Err("domain is required.".to_string());
        }
        if config.backup_path.trim().is_empty() {
            return Err("backup_path is required.".to_string());
        }
        Ok(())
    }

    fn ensure_backup_path(path: &str) -> Result<(), String> {
        let p = Path::new(path);
        if p.exists() {
            return Ok(());
        }
        fs::create_dir_all(p).map_err(|e| format!("backup_path does not exist and cannot be created: {}", e))
    }

    fn restic_available() -> bool {
        Path::new("/usr/bin/restic").exists() || command_exists("restic")
    }

    fn shell_quote(value: &str) -> String {
        format!("'{}'", value.replace('\'', "'\"'\"'"))
    }

    fn validate_cron_expr(expr: &str) -> Result<(), String> {
        let parts: Vec<&str> = expr.split_whitespace().collect();
        if parts.len() != 5 {
            return Err("cron must have 5 fields: minute hour day month weekday".to_string());
        }
        Ok(())
    }

    fn cron_file_path() -> PathBuf {
        PathBuf::from(
            std::env::var("AURAPANEL_BACKUP_CRON_FILE")
                .unwrap_or_else(|_| "/etc/cron.d/aurapanel-backup".to_string()),
        )
    }

    fn sync_backup_cron(state: &BackupCenterState) -> Result<(), String> {
        let mut lines = vec![
            "# Managed by AuraPanel Backup Center".to_string(),
            "SHELL=/bin/bash".to_string(),
            "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin".to_string(),
            String::new(),
        ];

        for schedule in state.schedules.iter().filter(|x| x.enabled) {
            let Some(destination) = state
                .destinations
                .iter()
                .find(|d| d.id == schedule.destination_id && d.enabled) else {
                continue;
            };

            Self::validate_cron_expr(&schedule.cron)?;

            let mode_tag = if schedule.incremental {
                "incremental"
            } else {
                "full"
            };

            let command = format!(
                "RESTIC_REPOSITORY={repo} RESTIC_PASSWORD={password} restic backup {path} --tag {domain} --tag {mode} >> /var/log/aurapanel-backup.log 2>&1",
                repo = Self::shell_quote(&destination.remote_repo),
                password = Self::shell_quote(&destination.password),
                path = Self::shell_quote(&schedule.backup_path),
                domain = Self::shell_quote(&schedule.domain),
                mode = Self::shell_quote(mode_tag),
            );
            lines.push(format!("{} root {}", schedule.cron.trim(), command));
        }

        let cron_file = Self::cron_file_path();
        if let Some(parent) = cron_file.parent() {
            fs::create_dir_all(parent).map_err(|e| e.to_string())?;
        }

        fs::write(cron_file, format!("{}\n", lines.join("\n"))).map_err(|e| e.to_string())
    }

    pub fn list_destinations() -> Result<Vec<BackupDestination>, String> {
        Ok(load_backup_center_state()?.destinations)
    }

    pub fn upsert_destination(mut payload: BackupDestination) -> Result<BackupDestination, String> {
        if payload.name.trim().is_empty() {
            return Err("destination name is required".to_string());
        }
        if payload.remote_repo.trim().is_empty() {
            return Err("remote_repo is required".to_string());
        }
        if payload.password.trim().is_empty() {
            return Err("password is required".to_string());
        }

        if payload.id.trim().is_empty() {
            payload.id = format!("dest_{}", secure_random_hex(6));
        }
        payload.name = payload.name.trim().to_string();
        payload.remote_repo = payload.remote_repo.trim().to_string();
        payload.password = payload.password.trim().to_string();
        payload.enabled = payload.enabled;

        let mut state = load_backup_center_state()?;
        if let Some(existing) = state.destinations.iter_mut().find(|x| x.id == payload.id) {
            *existing = payload.clone();
        } else {
            state.destinations.push(payload.clone());
        }
        save_backup_center_state(&state)?;
        Self::sync_backup_cron(&state)?;
        Ok(payload)
    }

    pub fn delete_destination(id: &str) -> Result<(), String> {
        let id = id.trim();
        if id.is_empty() {
            return Err("id is required".to_string());
        }

        let mut state = load_backup_center_state()?;
        let before = state.destinations.len();
        state.destinations.retain(|x| x.id != id);
        state.schedules.retain(|x| x.destination_id != id);
        if before == state.destinations.len() {
            return Err("destination not found".to_string());
        }
        save_backup_center_state(&state)?;
        Self::sync_backup_cron(&state)?;
        Ok(())
    }

    pub fn list_schedules() -> Result<Vec<BackupSchedule>, String> {
        Ok(load_backup_center_state()?.schedules)
    }

    pub fn upsert_schedule(mut payload: BackupSchedule) -> Result<BackupSchedule, String> {
        if payload.domain.trim().is_empty() {
            return Err("domain is required".to_string());
        }
        if payload.destination_id.trim().is_empty() {
            return Err("destination_id is required".to_string());
        }
        if payload.backup_path.trim().is_empty() {
            return Err("backup_path is required".to_string());
        }
        Self::validate_cron_expr(&payload.cron)?;

        if payload.id.trim().is_empty() {
            payload.id = format!("sch_{}", secure_random_hex(6));
        }

        payload.domain = payload.domain.trim().to_ascii_lowercase();
        payload.destination_id = payload.destination_id.trim().to_string();
        payload.backup_path = payload.backup_path.trim().to_string();
        payload.cron = payload.cron.trim().to_string();

        let mut state = load_backup_center_state()?;
        if !state.destinations.iter().any(|d| d.id == payload.destination_id) {
            return Err("destination_id not found".to_string());
        }

        if let Some(existing) = state.schedules.iter_mut().find(|x| x.id == payload.id) {
            *existing = payload.clone();
        } else {
            state.schedules.push(payload.clone());
        }
        save_backup_center_state(&state)?;
        Self::sync_backup_cron(&state)?;
        Ok(payload)
    }

    pub fn delete_schedule(id: &str) -> Result<(), String> {
        let id = id.trim();
        if id.is_empty() {
            return Err("id is required".to_string());
        }
        let mut state = load_backup_center_state()?;
        let before = state.schedules.len();
        state.schedules.retain(|x| x.id != id);
        if before == state.schedules.len() {
            return Err("schedule not found".to_string());
        }
        save_backup_center_state(&state)?;
        Self::sync_backup_cron(&state)?;
        Ok(())
    }

    pub fn list_snapshots(config: &BackupConfig) -> Result<serde_json::Value, String> {
        Self::validate_backup_input(config)?;
        let remote_repo = Self::resolve_backup_repo(config)?;
        let password = Self::resolve_backup_password(config)?;
        let minio_env = Self::resolve_minio_env()?;

        if !Self::restic_available() {
            return Err("restic is not installed.".to_string());
        }

        let mut cmd = Command::new("restic");
        cmd.env("RESTIC_REPOSITORY", &remote_repo)
            .env("RESTIC_PASSWORD", &password)
            .args(["snapshots", "--json", "--tag", config.domain.trim()]);

        if let Some((access_key, secret_key)) = minio_env {
            cmd.env("AWS_ACCESS_KEY_ID", access_key)
                .env("AWS_SECRET_ACCESS_KEY", secret_key);
        }

        let output = cmd
            .output()
            .map_err(|e| format!("restic snapshots failed: {}", e))?;
        if !output.status.success() {
            return Err(String::from_utf8_lossy(&output.stderr).trim().to_string());
        }

        serde_json::from_slice::<serde_json::Value>(&output.stdout)
            .map_err(|e| format!("snapshot json parse failed: {}", e))
    }

    pub async fn create_backup(config: &BackupConfig) -> Result<String, String> {
        Self::validate_backup_input(config)?;
        Self::ensure_backup_path(&config.backup_path)?;
        let remote_repo = Self::resolve_backup_repo(config)?;
        let password = Self::resolve_backup_password(config)?;
        let minio_env = Self::resolve_minio_env()?;

        let incremental = config.incremental.unwrap_or(false);

        if !Self::restic_available() {
            return Err("restic is not installed.".to_string());
        }

        let db_dump = format!("/tmp/{}_db.sql", config.domain);
        if command_exists("mysqldump") {
            let _ = run_command("mysqldump", &["--all-databases", "--result-file", &db_dump]);
        }

        let mode_tag = if incremental { "incremental" } else { "full" };
        let mut args: Vec<String> = vec!["backup".to_string(), config.backup_path.clone()];
        if Path::new(&db_dump).exists() {
            args.push(db_dump.clone());
        }
        args.push("--tag".to_string());
        args.push(config.domain.clone());
        args.push("--tag".to_string());
        args.push(mode_tag.to_string());

        let mut cmd = Command::new("restic");
        cmd.env("RESTIC_REPOSITORY", &remote_repo)
            .env("RESTIC_PASSWORD", &password)
            .args(args.iter().map(|s| s.as_str()));

        if let Some((access_key, secret_key)) = minio_env {
            cmd.env("AWS_ACCESS_KEY_ID", access_key)
                .env("AWS_SECRET_ACCESS_KEY", secret_key);
        }

        let output = cmd
            .output()
            .map_err(|e| format!("restic backup failed: {}", e))?;

        if !output.status.success() {
            return Err(format!("Backup failed: {}", String::from_utf8_lossy(&output.stderr)));
        }

        Ok(String::from_utf8_lossy(&output.stdout).trim().to_string())
    }

    pub async fn restore_backup(config: &BackupConfig, snapshot_id: &str) -> Result<(), String> {
        Self::validate_backup_input(config)?;
        let remote_repo = Self::resolve_backup_repo(config)?;
        let password = Self::resolve_backup_password(config)?;
        let minio_env = Self::resolve_minio_env()?;
        if snapshot_id.trim().is_empty() {
            return Err("snapshot_id is required.".to_string());
        }

        if !Self::restic_available() {
            return Err("restic is not installed.".to_string());
        }

        let mut cmd = Command::new("restic");
        cmd.env("RESTIC_REPOSITORY", &remote_repo)
            .env("RESTIC_PASSWORD", &password)
            .args(["restore", snapshot_id, "--target", &config.backup_path]);

        if let Some((access_key, secret_key)) = minio_env {
            cmd.env("AWS_ACCESS_KEY_ID", access_key)
                .env("AWS_SECRET_ACCESS_KEY", secret_key);
        }

        let output = cmd.output().map_err(|e| format!("restore failed: {}", e))?;

        if !output.status.success() {
            return Err(String::from_utf8_lossy(&output.stderr).to_string());
        }
        Ok(())
    }
}

pub struct StorageManager;

impl StorageManager {
    pub fn create_bucket(bucket_name: &str) -> Result<(), String> {
        let bucket_name = bucket_name.trim().to_ascii_lowercase();
        if !bucket_name_valid(&bucket_name) {
            return Err("bucket_name is invalid. Use 3-63 chars [a-z0-9.-].".to_string());
        }

        let alias = std::env::var("AURAPANEL_MINIO_ALIAS").unwrap_or_default();
        if !alias.trim().is_empty() && command_exists("mc") {
            let target = format!("{}/{}", alias.trim(), bucket_name);
            let _ = run_command("mc", &["mb", "--ignore-existing", &target]);
        }

        let mut state = load_storage_state()?;
        if !state.buckets.iter().any(|x| x == &bucket_name) {
            state.buckets.push(bucket_name);
        }
        save_storage_state(&state)
    }

    pub fn list_buckets() -> Result<Vec<String>, String> {
        Ok(load_storage_state()?.buckets)
    }

    pub fn generate_credentials(user: &str) -> Result<MinioCredentialsResponse, String> {
        let user = user.trim().to_ascii_lowercase();
        if user.is_empty() {
            return Err("user is required.".to_string());
        }

        let mut state = load_storage_state()?;
        if let Some(existing) = state.credentials.get(&user) {
            return Ok(existing.clone());
        }

        let access_key = format!("ak_{}", secure_random_hex(8));
        let secret_key = secure_random_hex(24);
        let creds = MinioCredentialsResponse {
            access_key,
            secret_key,
        };
        state.credentials.insert(user, creds.clone());
        save_storage_state(&state)?;
        Ok(creds)
    }
}
