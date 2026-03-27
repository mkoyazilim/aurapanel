use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

const OLS_TUNING_BEGIN: &str = "# AURAPANEL_OLS_TUNING_BEGIN";
const OLS_TUNING_END: &str = "# AURAPANEL_OLS_TUNING_END";

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct OlsTuningConfig {
    pub max_connections: u32,
    pub max_ssl_connections: u32,
    pub conn_timeout_secs: u32,
    pub keep_alive_timeout_secs: u32,
    pub max_keep_alive_requests: u32,
    pub gzip_compression: bool,
    pub static_cache_enabled: bool,
    pub static_cache_max_age_secs: u32,
}

impl Default for OlsTuningConfig {
    fn default() -> Self {
        Self {
            max_connections: 10_000,
            max_ssl_connections: 10_000,
            conn_timeout_secs: 300,
            keep_alive_timeout_secs: 5,
            max_keep_alive_requests: 10_000,
            gzip_compression: true,
            static_cache_enabled: true,
            static_cache_max_age_secs: 3600,
        }
    }
}

pub struct OlsTuningManager;

impl OlsTuningManager {
    pub fn get_config() -> Result<OlsTuningConfig, String> {
        load_state().or_else(|_| Ok(OlsTuningConfig::default()))
    }

    pub fn save_config(input: &OlsTuningConfig) -> Result<OlsTuningConfig, String> {
        let normalized = normalize(input)?;
        save_state(&normalized)?;
        Ok(normalized)
    }

    pub fn apply_config(input: &OlsTuningConfig) -> Result<String, String> {
        let normalized = normalize(input)?;
        save_state(&normalized)?;

        if crate::runtime::simulation_enabled() {
            return Ok("Simulation mode: OLS tuning state saved, config write skipped.".to_string());
        }

        let path = ols_httpd_conf_path();
        if !path.exists() {
            return Err(format!(
                "OpenLiteSpeed config file not found: {}",
                path.display()
            ));
        }

        let original = fs::read_to_string(&path)
            .map_err(|e| format!("OLS config read failed: {}", e))?;

        let managed_block = render_managed_block(&normalized);
        let updated = inject_managed_block(&original, &managed_block);

        let backup_path = PathBuf::from(format!("{}.aurapanel.bak", path.display()));
        fs::write(&backup_path, &original)
            .map_err(|e| format!("OLS config backup write failed: {}", e))?;

        if let Err(write_err) = fs::write(&path, updated) {
            let _ = fs::write(&path, original);
            return Err(format!("OLS config write failed: {}", write_err));
        }

        if let Err(restart_err) = restart_ols() {
            let _ = fs::write(&path, original);
            return Err(format!(
                "OLS restart failed, config reverted: {}",
                restart_err
            ));
        }

        Ok(format!(
            "OLS tuning applied successfully (config: {}).",
            path.display()
        ))
    }
}

fn normalize(input: &OlsTuningConfig) -> Result<OlsTuningConfig, String> {
    if input.max_connections < 100 || input.max_connections > 500_000 {
        return Err("max_connections must be between 100 and 500000".to_string());
    }
    if input.max_ssl_connections < 100 || input.max_ssl_connections > 500_000 {
        return Err("max_ssl_connections must be between 100 and 500000".to_string());
    }
    if input.conn_timeout_secs < 30 || input.conn_timeout_secs > 3600 {
        return Err("conn_timeout_secs must be between 30 and 3600".to_string());
    }
    if input.keep_alive_timeout_secs < 1 || input.keep_alive_timeout_secs > 120 {
        return Err("keep_alive_timeout_secs must be between 1 and 120".to_string());
    }
    if input.max_keep_alive_requests < 10 || input.max_keep_alive_requests > 1_000_000 {
        return Err("max_keep_alive_requests must be between 10 and 1000000".to_string());
    }
    if input.static_cache_max_age_secs > 31_536_000 {
        return Err("static_cache_max_age_secs must be <= 31536000".to_string());
    }

    Ok(input.clone())
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

fn state_path() -> PathBuf {
    state_root().join("ols_tuning.json")
}

fn ols_httpd_conf_path() -> PathBuf {
    PathBuf::from(
        std::env::var("AURAPANEL_OLS_HTTPD_CONF")
            .unwrap_or_else(|_| "/usr/local/lsws/conf/httpd_config.conf".to_string()),
    )
}

fn load_state() -> Result<OlsTuningConfig, String> {
    let path = state_path();
    if !path.exists() {
        return Ok(OlsTuningConfig::default());
    }

    let raw = fs::read_to_string(path).map_err(|e| e.to_string())?;
    serde_json::from_str(&raw).map_err(|e| e.to_string())
}

fn save_state(cfg: &OlsTuningConfig) -> Result<(), String> {
    let path = state_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| e.to_string())?;
    }

    let payload = serde_json::to_string_pretty(cfg).map_err(|e| e.to_string())?;
    fs::write(path, payload).map_err(|e| e.to_string())
}

fn render_managed_block(cfg: &OlsTuningConfig) -> String {
    format!(
        "{begin}\n\ntuning {{\n  maxConnections         {max_conn}\n  maxSSLConnections      {max_ssl_conn}\n  connTimeout            {conn_timeout}\n  maxKeepAliveReq        {max_keepalive}\n  smartKeepAlive         1\n  keepAliveTimeout       {keepalive_timeout}\n}}\n\n# AuraPanel static file cache preference\ncache {{\n  enableCache            {cache_enabled}\n  enablePublicCache      {cache_enabled}\n  maxCacheObjSize        10485760\n  defaultExpires         {cache_ttl}\n}}\n\n# AuraPanel gzip preference\ngzip {{\n  enable                 {gzip_enabled}\n}}\n\n{end}\n",
        begin = OLS_TUNING_BEGIN,
        end = OLS_TUNING_END,
        max_conn = cfg.max_connections,
        max_ssl_conn = cfg.max_ssl_connections,
        conn_timeout = cfg.conn_timeout_secs,
        max_keepalive = cfg.max_keep_alive_requests,
        keepalive_timeout = cfg.keep_alive_timeout_secs,
        cache_enabled = if cfg.static_cache_enabled { 1 } else { 0 },
        cache_ttl = cfg.static_cache_max_age_secs,
        gzip_enabled = if cfg.gzip_compression { 1 } else { 0 },
    )
}

fn inject_managed_block(original: &str, block: &str) -> String {
    if let Some(begin_idx) = original.find(OLS_TUNING_BEGIN) {
        if let Some(end_rel) = original[begin_idx..].find(OLS_TUNING_END) {
            let end_idx = begin_idx + end_rel + OLS_TUNING_END.len();
            let mut output = String::new();
            output.push_str(&original[..begin_idx]);
            if !output.ends_with('\n') {
                output.push('\n');
            }
            output.push_str(block);
            if end_idx < original.len() {
                let tail = &original[end_idx..];
                if !tail.starts_with('\n') {
                    output.push('\n');
                }
                output.push_str(tail);
            }
            return output;
        }
    }

    let mut output = original.to_string();
    if !output.ends_with('\n') {
        output.push('\n');
    }
    output.push('\n');
    output.push_str(block);
    output
}

fn restart_ols() -> Result<(), String> {
    let lswsctrl = Path::new("/usr/local/lsws/bin/lswsctrl");

    let output = if lswsctrl.exists() {
        Command::new(lswsctrl)
            .arg("restart")
            .output()
            .map_err(|e| format!("lswsctrl restart failed: {}", e))?
    } else {
        Command::new("systemctl")
            .args(["restart", "lsws"])
            .output()
            .map_err(|e| format!("systemctl restart lsws failed: {}", e))?
    };

    if output.status.success() {
        Ok(())
    } else {
        Err(String::from_utf8_lossy(&output.stderr).trim().to_string())
    }
}
