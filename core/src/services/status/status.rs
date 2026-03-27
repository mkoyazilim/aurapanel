use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

const DEFAULT_PANEL_PORT: u16 = 8090;
const GATEWAY_ENV_FILE: &str = "/etc/aurapanel/aurapanel.env";
const GATEWAY_ADDR_KEY: &str = "AURAPANEL_GATEWAY_ADDR";
const GATEWAY_ALLOWED_ORIGINS_KEY: &str = "AURAPANEL_ALLOWED_ORIGINS";
const GATEWAY_SERVICE_NAME: &str = "aurapanel-api";

#[derive(Serialize, Deserialize, Debug)]
pub struct SystemMetrics {
    pub cpu_usage: f64,
    pub cpu_cores: u32,
    pub cpu_model: String,
    pub ram_usage: f64,
    pub ram_used: String,
    pub ram_total: String,
    pub disk_usage: f64,
    pub disk_used: String,
    pub disk_total: String,
    pub uptime_seconds: u64,
    pub uptime_human: String,
    pub load_avg: String,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct ServiceStatus {
    pub name: String,
    pub status: String,
    pub desc: String,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct ProcessInfo {
    pub pid: u32,
    pub user: String,
    pub cpu: f64,
    pub mem: f64,
    pub command: String,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct NetworkStats {
    pub tx_rate: String,
    pub rx_rate: String,
    pub tx_total: String,
    pub rx_total: String,
    pub http_conns: u32,
    pub ssh_conns: u32,
    pub mysql_conns: u32,
    pub pg_conns: u32,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct PanelPortInfo {
    pub current_port: u16,
    pub default_port: u16,
    pub gateway_addr: String,
    pub env_file: String,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct PanelPortUpdateResult {
    pub previous_port: u16,
    pub new_port: u16,
    pub gateway_addr: String,
    pub env_file: String,
    pub firewall_actions: Vec<String>,
    pub restart_scheduled: bool,
    pub warnings: Vec<String>,
}

pub struct StatusManager;

impl StatusManager {
    pub fn get_metrics() -> Result<SystemMetrics, String> {
        if cfg!(windows) {
            return Err("System metrics are supported only on Linux hosts.".to_string());
        }
        if !Path::new("/proc/loadavg").exists() {
            return Err("/proc/loadavg not found. Real Linux metrics cannot be collected.".to_string());
        }

        let cpu_cores = fs::read_to_string("/proc/cpuinfo")
            .unwrap_or_default()
            .lines()
            .filter(|l| l.starts_with("processor"))
            .count() as u32;

        let cpu_model = fs::read_to_string("/proc/cpuinfo")
            .unwrap_or_default()
            .lines()
            .find(|l| l.starts_with("model name"))
            .map(|l| l.split(':').nth(1).unwrap_or("").trim().to_string())
            .unwrap_or_else(|| "Unknown".to_string());

        let load_avg = fs::read_to_string("/proc/loadavg")
            .unwrap_or_default()
            .split_whitespace()
            .take(3)
            .collect::<Vec<_>>()
            .join(", ");

        let cpu_usage = fs::read_to_string("/proc/loadavg")
            .unwrap_or_default()
            .split_whitespace()
            .next()
            .unwrap_or("0")
            .parse::<f64>()
            .unwrap_or(0.0)
            / cpu_cores.max(1) as f64
            * 100.0;

        let meminfo = fs::read_to_string("/proc/meminfo").unwrap_or_default();
        let mem_total = Self::parse_meminfo(&meminfo, "MemTotal");
        let mem_avail = Self::parse_meminfo(&meminfo, "MemAvailable");
        let mem_used = mem_total.saturating_sub(mem_avail);
        let ram_usage = if mem_total > 0 {
            (mem_used as f64 / mem_total as f64) * 100.0
        } else {
            0.0
        };

        let df_output = Command::new("df")
            .args(["-B1", "/"])
            .output()
            .map(|o| String::from_utf8_lossy(&o.stdout).to_string())
            .unwrap_or_default();
        let (disk_total, disk_used, disk_pct) = Self::parse_df(&df_output);

        let uptime_str = fs::read_to_string("/proc/uptime").unwrap_or_default();
        let uptime_secs = uptime_str
            .split_whitespace()
            .next()
            .unwrap_or("0")
            .parse::<f64>()
            .unwrap_or(0.0) as u64;
        let days = uptime_secs / 86_400;
        let hours = (uptime_secs % 86_400) / 3_600;
        let mins = (uptime_secs % 3_600) / 60;

        Ok(SystemMetrics {
            cpu_usage: (cpu_usage * 10.0).round() / 10.0,
            cpu_cores,
            cpu_model,
            ram_usage: (ram_usage * 10.0).round() / 10.0,
            ram_used: Self::format_bytes(mem_used * 1024),
            ram_total: Self::format_bytes(mem_total * 1024),
            disk_usage: disk_pct,
            disk_used,
            disk_total,
            uptime_seconds: uptime_secs,
            uptime_human: format!("{} gun, {} saat, {} dk", days, hours, mins),
            load_avg,
        })
    }

    pub fn get_services() -> Result<Vec<ServiceStatus>, String> {
        if cfg!(windows) {
            return Err("Service status is supported only on Linux hosts.".to_string());
        }

        let service_list = vec![
            ("lshttpd", "OpenLiteSpeed", "Web Sunucusu"),
            ("mariadb", "MariaDB", "Veritabani Sunucusu"),
            ("postgresql", "PostgreSQL", "PostgreSQL 16"),
            ("php8.3-fpm", "PHP 8.3-FPM", "PHP Isleme Motoru"),
            ("pure-ftpd", "PureFTPd", "FTP Sunucu"),
            ("postfix", "Postfix", "SMTP Sunucu"),
            ("dovecot", "Dovecot", "IMAP/POP3 Sunucu"),
            ("pdns", "PowerDNS", "DNS Sunucu"),
            ("redis-server", "Redis", "Cache Sunucu"),
            ("docker", "Docker", "Container Engine"),
            ("fail2ban", "Fail2Ban", "Brute-force Korumasi"),
        ];

        let mut results = Vec::new();
        for (unit, name, desc) in &service_list {
            let output = Command::new("systemctl").args(["is-active", unit]).output();
            let status = match output {
                Ok(o) => {
                    let s = String::from_utf8_lossy(&o.stdout).trim().to_string();
                    if s == "active" {
                        "running"
                    } else {
                        "stopped"
                    }
                }
                Err(_) => "stopped",
            };
            results.push(ServiceStatus {
                name: name.to_string(),
                status: status.to_string(),
                desc: desc.to_string(),
            });
        }
        Ok(results)
    }

    pub fn get_processes() -> Result<Vec<ProcessInfo>, String> {
        if cfg!(windows) {
            return Err("Process inspection is supported only on Linux hosts.".to_string());
        }

        let output = Command::new("ps")
            .args(["aux", "--sort=-%cpu"])
            .output()
            .map_err(|e| format!("ps calistirilamadi: {}", e))?;

        let stdout = String::from_utf8_lossy(&output.stdout);
        let mut procs = Vec::new();
        for (i, line) in stdout.lines().enumerate() {
            if i == 0 || i > 20 {
                continue;
            }
            let parts: Vec<&str> = line.split_whitespace().collect();
            if parts.len() >= 11 {
                procs.push(ProcessInfo {
                    pid: parts[1].parse().unwrap_or(0),
                    user: parts[0].to_string(),
                    cpu: parts[2].parse().unwrap_or(0.0),
                    mem: parts[3].parse().unwrap_or(0.0),
                    command: parts[10..].join(" "),
                });
            }
        }
        Ok(procs)
    }

    pub fn control_service(name: &str, action: &str) -> Result<String, String> {
        let normalized_action = action.trim().to_ascii_lowercase();
        let normalized_name = name.trim().to_string();
        let unit_name = Self::resolve_service_name(&normalized_name);

        if normalized_action == "kill" {
            if normalized_name.is_empty() {
                return Err("process id is required for kill action.".to_string());
            }

            let output = Command::new("kill")
                .args(["-9", &normalized_name])
                .output()
                .map_err(|e| format!("kill calistirilamadi: {}", e))?;

            if output.status.success() {
                return Ok(format!("Process {} killed.", normalized_name));
            }

            return Err(String::from_utf8_lossy(&output.stderr).to_string());
        }
        if cfg!(windows) {
            return Err("Service control is supported only on Linux hosts.".to_string());
        }

        let output = Command::new("systemctl")
            .args([&normalized_action, &unit_name])
            .output()
            .map_err(|e| format!("systemctl {} calistirilamadi: {}", normalized_action, e))?;

        if output.status.success() {
            Ok(format!("{} servisi {} edildi.", normalized_name, normalized_action))
        } else {
            Err(String::from_utf8_lossy(&output.stderr).to_string())
        }
    }

    pub fn get_panel_port() -> Result<PanelPortInfo, String> {
        let gateway_addr = Self::current_gateway_addr();
        let current_port = Self::extract_port_from_addr(&gateway_addr).unwrap_or(DEFAULT_PANEL_PORT);

        Ok(PanelPortInfo {
            current_port,
            default_port: DEFAULT_PANEL_PORT,
            gateway_addr,
            env_file: GATEWAY_ENV_FILE.to_string(),
        })
    }

    pub fn update_panel_port(new_port: u16, open_firewall: bool) -> Result<PanelPortUpdateResult, String> {
        if new_port == 0 {
            return Err("Port araligi 1-65535 olmalidir.".to_string());
        }

        let previous_port = Self::get_panel_port()?.current_port;
        let gateway_addr = format!(":{}", new_port);
        let mut warnings = Vec::new();

        Self::upsert_env_value(GATEWAY_ENV_FILE, GATEWAY_ADDR_KEY, &gateway_addr)?;
        Self::sync_allowed_origins(new_port)?;

        let firewall_actions = if open_firewall {
            match Self::open_firewall_port(new_port) {
                Ok(actions) => actions,
                Err(err) => {
                    warnings.push(err);
                    Vec::new()
                }
            }
        } else {
            vec!["Firewall update skipped by request.".to_string()]
        };

        let restart_scheduled = match Self::schedule_gateway_restart() {
            Ok(v) => v,
            Err(err) => {
                warnings.push(err);
                false
            }
        };

        Ok(PanelPortUpdateResult {
            previous_port,
            new_port,
            gateway_addr,
            env_file: GATEWAY_ENV_FILE.to_string(),
            firewall_actions,
            restart_scheduled,
            warnings,
        })
    }

    fn parse_meminfo(content: &str, key: &str) -> u64 {
        content
            .lines()
            .find(|l| l.starts_with(key))
            .and_then(|l| l.split_whitespace().nth(1))
            .and_then(|v| v.parse::<u64>().ok())
            .unwrap_or(0)
    }

    fn parse_df(output: &str) -> (String, String, f64) {
        if let Some(line) = output.lines().nth(1) {
            let parts: Vec<&str> = line.split_whitespace().collect();
            if parts.len() >= 5 {
                let total = parts[1].parse::<u64>().unwrap_or(0);
                let used = parts[2].parse::<u64>().unwrap_or(0);
                let pct = parts[4].trim_end_matches('%').parse::<f64>().unwrap_or(0.0);
                return (Self::format_bytes(total), Self::format_bytes(used), pct);
            }
        }
        ("0 B".to_string(), "0 B".to_string(), 0.0)
    }

    fn format_bytes(bytes: u64) -> String {
        const KB: u64 = 1024;
        const MB: u64 = KB * 1024;
        const GB: u64 = MB * 1024;
        const TB: u64 = GB * 1024;

        if bytes >= TB {
            format!("{:.1} TB", bytes as f64 / TB as f64)
        } else if bytes >= GB {
            format!("{:.1} GB", bytes as f64 / GB as f64)
        } else if bytes >= MB {
            format!("{:.1} MB", bytes as f64 / MB as f64)
        } else if bytes >= KB {
            format!("{:.1} KB", bytes as f64 / KB as f64)
        } else {
            format!("{} B", bytes)
        }
    }

    fn current_gateway_addr() -> String {
        if let Some(file_value) = Self::read_env_value(GATEWAY_ENV_FILE, GATEWAY_ADDR_KEY) {
            if !file_value.trim().is_empty() {
                return file_value.trim().to_string();
            }
        }

        if let Ok(env_value) = std::env::var(GATEWAY_ADDR_KEY) {
            if !env_value.trim().is_empty() {
                return env_value.trim().to_string();
            }
        }

        format!(":{}", DEFAULT_PANEL_PORT)
    }

    fn extract_port_from_addr(addr: &str) -> Option<u16> {
        let trimmed = addr.trim();
        if trimmed.is_empty() {
            return None;
        }

        if let Some(rest) = trimmed.strip_prefix(':') {
            return rest.trim().parse::<u16>().ok().filter(|p| *p > 0);
        }

        let candidate = trimmed.rsplit(':').next()?.trim();
        candidate.parse::<u16>().ok().filter(|p| *p > 0)
    }

    fn resolve_service_name(input: &str) -> String {
        let lowered = input.trim().to_ascii_lowercase();
        match lowered.as_str() {
            "openlitespeed" => "lshttpd".to_string(),
            "mariadb" => "mariadb".to_string(),
            "postgresql" => "postgresql".to_string(),
            "php 8.3-fpm" | "php8.3-fpm" => "php8.3-fpm".to_string(),
            "pureftpd" | "pure-ftpd" => "pure-ftpd".to_string(),
            "postfix" => "postfix".to_string(),
            "dovecot" => "dovecot".to_string(),
            "powerdns" | "pdns" => "pdns".to_string(),
            "redis" | "redis-server" => "redis-server".to_string(),
            "docker" => "docker".to_string(),
            "fail2ban" => "fail2ban".to_string(),
            _ => input.trim().to_string(),
        }
    }

    fn read_env_value(file: &str, key: &str) -> Option<String> {
        let content = fs::read_to_string(file).ok()?;

        content.lines().find_map(|line| {
            let trimmed = line.trim();
            if trimmed.is_empty() || trimmed.starts_with('#') {
                return None;
            }

            let (line_key, line_value) = trimmed.split_once('=')?;
            if line_key.trim() != key {
                return None;
            }
            Some(line_value.trim().to_string())
        })
    }

    fn upsert_env_value(file: &str, key: &str, value: &str) -> Result<(), String> {
        let path = Path::new(file);
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).map_err(|e| format!("env klasoru olusturulamadi: {}", e))?;
        }

        let existing = fs::read_to_string(path).unwrap_or_default();
        let mut lines: Vec<String> = existing.lines().map(|line| line.to_string()).collect();

        let mut updated = false;
        for line in &mut lines {
            let trimmed = line.trim_start();
            if trimmed.starts_with('#') {
                continue;
            }
            if let Some((line_key, _)) = trimmed.split_once('=') {
                if line_key.trim() == key {
                    *line = format!("{}={}", key, value);
                    updated = true;
                    break;
                }
            }
        }

        if !updated {
            lines.push(format!("{}={}", key, value));
        }

        let mut out = lines.join("\n");
        out.push('\n');
        fs::write(path, out).map_err(|e| format!("env guncellenemedi: {}", e))
    }

    fn sync_allowed_origins(new_port: u16) -> Result<(), String> {
        let mut origins = Vec::<String>::new();

        let raw = Self::read_env_value(GATEWAY_ENV_FILE, GATEWAY_ALLOWED_ORIGINS_KEY)
            .or_else(|| std::env::var(GATEWAY_ALLOWED_ORIGINS_KEY).ok())
            .unwrap_or_default();

        for item in raw.split(',') {
            let origin = item.trim();
            if origin.is_empty() {
                continue;
            }
            if !origins.iter().any(|x| x == origin) {
                origins.push(origin.to_string());
            }
        }

        let localhost_origin = format!("http://localhost:{}", new_port);
        let loopback_origin = format!("http://127.0.0.1:{}", new_port);

        if !origins.iter().any(|x| x == &localhost_origin) {
            origins.push(localhost_origin);
        }
        if !origins.iter().any(|x| x == &loopback_origin) {
            origins.push(loopback_origin);
        }

        Self::upsert_env_value(
            GATEWAY_ENV_FILE,
            GATEWAY_ALLOWED_ORIGINS_KEY,
            &origins.join(","),
        )
    }

    fn open_firewall_port(port: u16) -> Result<Vec<String>, String> {
        if cfg!(windows) {
            return Ok(vec!["Windows mode: firewall update skipped.".to_string()]);
        }

        let mut actions = Vec::new();
        let rule = format!("{}/tcp", port);

        if Self::binary_exists("ufw") {
            let status = Command::new("ufw")
                .arg("status")
                .output()
                .map_err(|e| format!("ufw status calistirilamadi: {}", e))?;

            let stdout = String::from_utf8_lossy(&status.stdout);
            if status.status.success() && stdout.contains("Status: active") {
                let allow = Command::new("ufw")
                    .args(["allow", &rule])
                    .output()
                    .map_err(|e| format!("ufw allow komutu calismadi: {}", e))?;
                if allow.status.success() {
                    actions.push(format!("ufw allow {}", rule));
                } else {
                    let stderr = String::from_utf8_lossy(&allow.stderr);
                    actions.push(format!("ufw warning: {}", stderr.trim()));
                }
            } else {
                actions.push("ufw yuklu ama aktif degil.".to_string());
            }
        }

        if Self::binary_exists("firewall-cmd") {
            let state = Command::new("firewall-cmd")
                .arg("--state")
                .output()
                .map_err(|e| format!("firewall-cmd --state calistirilamadi: {}", e))?;

            let running = state.status.success()
                && String::from_utf8_lossy(&state.stdout)
                    .trim()
                    .eq_ignore_ascii_case("running");

            if running {
                let add = Command::new("firewall-cmd")
                    .args(["--permanent", "--add-port", &rule])
                    .output()
                    .map_err(|e| format!("firewall-cmd add-port hatasi: {}", e))?;

                if add.status.success() {
                    actions.push(format!("firewalld add-port {}", rule));
                    let _ = Command::new("firewall-cmd").arg("--reload").output();
                } else {
                    let stderr = String::from_utf8_lossy(&add.stderr);
                    actions.push(format!("firewalld warning: {}", stderr.trim()));
                }
            } else {
                actions.push("firewalld yuklu ama aktif degil.".to_string());
            }
        }

        if actions.is_empty() {
            actions.push("Desteklenen firewall araci bulunamadi (ufw/firewalld).".to_string());
        }

        Ok(actions)
    }

    fn schedule_gateway_restart() -> Result<bool, String> {
        if cfg!(windows) {
            return Ok(false);
        }

        let command = format!(
            "nohup sh -c 'sleep 2; systemctl restart {}' >/dev/null 2>&1 &",
            GATEWAY_SERVICE_NAME
        );

        let status = Command::new("sh")
            .args(["-c", &command])
            .status()
            .map_err(|e| format!("gateway restart planlanamadi: {}", e))?;

        if status.success() {
            Ok(true)
        } else {
            Err("gateway restart planlama komutu basarisiz oldu.".to_string())
        }
    }

    fn binary_exists(name: &str) -> bool {
        let candidates = [
            PathBuf::from(format!("/usr/bin/{}", name)),
            PathBuf::from(format!("/usr/sbin/{}", name)),
            PathBuf::from(format!("/bin/{}", name)),
            PathBuf::from(format!("/sbin/{}", name)),
        ];

        candidates.iter().any(|path| path.exists())
    }
}
