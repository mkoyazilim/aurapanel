use serde::{Deserialize, Serialize};
use std::fs;
use std::path::Path;
use std::process::Command;

#[derive(Serialize, Deserialize, Debug)]
pub struct RedisConfig {
    pub domain: String,
    pub max_memory_mb: u32,
}

pub struct PerfManager;

impl PerfManager {
    pub async fn create_redis_instance(config: &RedisConfig) -> Result<(), String> {
        let domain = config.domain.trim().to_ascii_lowercase();
        if domain.is_empty() {
            return Err("domain is required.".to_string());
        }

        let max_memory = config.max_memory_mb.clamp(64, 65536);
        let conf_dir = "/etc/aurapanel/redis";
        let conf_path = format!("{}/{}.conf", conf_dir, domain);
        let socket_dir = "/run/aurapanel/redis";
        let sock_path = format!("{}/{}.sock", socket_dir, domain);
        let pid_file = format!("/run/aurapanel/redis-{}.pid", domain);
        let db_file = format!("dump-{}.rdb", domain);

        fs::create_dir_all(conf_dir).map_err(|e| format!("redis conf dir create failed: {}", e))?;
        fs::create_dir_all(socket_dir).map_err(|e| format!("redis socket dir create failed: {}", e))?;

        let conf = format!(
            "port 0\nunixsocket {}\nunixsocketperm 770\nmaxmemory {}mb\nmaxmemory-policy allkeys-lru\ndaemonize yes\npidfile {}\ndir /var/lib/redis\ndbfilename {}\n",
            sock_path, max_memory, pid_file, db_file
        );
        fs::write(&conf_path, conf).map_err(|e| format!("redis conf write failed: {}", e))?;

        let output = Command::new("redis-server")
            .arg(&conf_path)
            .output()
            .map_err(|e| format!("redis-server failed: {}", e))?;
        if !output.status.success() {
            return Err(String::from_utf8_lossy(&output.stderr).trim().to_string());
        }

        Ok(())
    }

    pub fn purge_lscache(domain: &str) -> Result<(), String> {
        let domain = domain.trim().to_ascii_lowercase();
        if domain.is_empty() {
            return Err("domain is required.".to_string());
        }

        let candidates = [
            format!("/home/{}/public_html/.lscache", domain),
            format!("/home/{}/.lscache", domain),
            format!("/usr/local/lsws/cachedata/{}", domain),
            format!("/var/cache/lsws/{}", domain),
        ];

        for path in candidates {
            if Path::new(&path).exists() {
                let output = Command::new("rm")
                    .args(["-rf", &path])
                    .output()
                    .map_err(|e| format!("cache purge command failed: {}", e))?;
                if !output.status.success() {
                    return Err(String::from_utf8_lossy(&output.stderr).trim().to_string());
                }
            }
        }

        Ok(())
    }
}
