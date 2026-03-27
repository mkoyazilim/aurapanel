use serde::{Deserialize, Serialize};
use std::fs;
use std::net::IpAddr;
use std::path::{Path, PathBuf};
use std::process::Command;

use super::{MariaDbManager, PostgresManager};

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct DbPasswordChangeRequest {
    pub db_user: String,
    pub new_password: String,
    pub host: Option<String>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct RemoteAccessGrantRequest {
    pub db_user: String,
    pub db_name: String,
    pub remote_ip: String,
    pub db_pass: Option<String>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct RemoteAccessRule {
    pub engine: String,
    pub db_user: String,
    pub db_name: String,
    pub remote: String,
    pub auth_method: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct DbConnectionCheckResult {
    pub engine: String,
    pub db_name: String,
    pub db_user: String,
    pub database_exists: bool,
    pub user_exists: bool,
    pub remote_access_rules: Vec<RemoteAccessRule>,
    pub ready: bool,
}

fn mysql_escape(value: &str) -> String {
    value.replace('\\', "\\\\").replace('\'', "\\'")
}

fn pg_escape(value: &str) -> String {
    value.replace('\'', "''")
}

fn pg_ident(value: &str) -> String {
    value.replace('"', "\"\"")
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

fn rules_path(engine: &str) -> PathBuf {
    state_root().join(format!("{}_remote_access.json", engine))
}

fn load_rules(engine: &str) -> Vec<RemoteAccessRule> {
    let path = rules_path(engine);
    if !path.exists() {
        return Vec::new();
    }
    fs::read_to_string(path)
        .ok()
        .and_then(|raw| serde_json::from_str::<Vec<RemoteAccessRule>>(&raw).ok())
        .unwrap_or_default()
}

fn save_rules(engine: &str, rules: &[RemoteAccessRule]) -> Result<(), String> {
    let path = rules_path(engine);
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| format!("State directory could not be created: {}", e))?;
    }
    let payload = serde_json::to_string_pretty(rules).map_err(|e| e.to_string())?;
    fs::write(path, payload).map_err(|e| format!("Remote access state could not be written: {}", e))
}

fn run_mysql(sql: &str) -> Result<String, String> {
    let output = Command::new("mysql")
        .args(["-u", "root", "-e", sql])
        .output();
    match output {
        Ok(o) if o.status.success() => Ok(String::from_utf8_lossy(&o.stdout).to_string()),
        Ok(o) => Err(String::from_utf8_lossy(&o.stderr).to_string()),
        Err(_) => Err("mysql client is not available.".to_string()),
    }
}

fn run_psql(sql: &str) -> Result<String, String> {
    let output = Command::new("sudo")
        .args(["-u", "postgres", "psql", "-c", sql])
        .output();
    match output {
        Ok(o) if o.status.success() => Ok(String::from_utf8_lossy(&o.stdout).to_string()),
        Ok(o) => Err(String::from_utf8_lossy(&o.stderr).to_string()),
        Err(_) => Err("postgresql command path is not available.".to_string()),
    }
}

pub fn change_password_mariadb(req: &DbPasswordChangeRequest) -> Result<String, String> {
    let user = req.db_user.trim();
    let password = req.new_password.trim();
    if user.is_empty() || password.is_empty() {
        return Err("db_user and new_password are required.".to_string());
    }

    let host = req.host.as_deref().unwrap_or("%").trim();
    run_mysql(&format!(
        "ALTER USER '{}'@'{}' IDENTIFIED BY '{}'; FLUSH PRIVILEGES;",
        mysql_escape(user),
        mysql_escape(host),
        mysql_escape(password)
    ))?;
    Ok(format!("MariaDB password rotated for '{}'.", user))
}

pub fn change_password_postgres(req: &DbPasswordChangeRequest) -> Result<String, String> {
    let user = req.db_user.trim();
    let password = req.new_password.trim();
    if user.is_empty() || password.is_empty() {
        return Err("db_user and new_password are required.".to_string());
    }

    run_psql(&format!(
        "ALTER ROLE \"{}\" WITH PASSWORD '{}';",
        pg_ident(user),
        pg_escape(password)
    ))?;
    Ok(format!("PostgreSQL password rotated for '{}'.", user))
}

pub fn list_remote_access_mariadb(db_user: Option<&str>) -> Result<Vec<RemoteAccessRule>, String> {
    let mut rules = load_rules("mariadb");
    let output = run_mysql("SELECT User, Host FROM mysql.user WHERE Host NOT IN ('localhost','127.0.0.1','::1');")?;
    for line in output.lines().skip(1).filter(|x| !x.trim().is_empty()) {
        let parts: Vec<&str> = line.split_whitespace().collect();
        if let (Some(user), Some(host)) = (parts.first(), parts.get(1)) {
            let exists = rules.iter().any(|r| r.db_user == *user && r.remote == *host && r.engine == "mariadb");
            if !exists {
                rules.push(RemoteAccessRule {
                    engine: "mariadb".to_string(),
                    db_user: (*user).to_string(),
                    db_name: "*".to_string(),
                    remote: (*host).to_string(),
                    auth_method: "mysql-native-password".to_string(),
                });
            }
        }
    }
    if let Some(user) = db_user {
        let user = user.trim();
        if !user.is_empty() {
            rules.retain(|x| x.db_user == user);
        }
    }
    Ok(rules)
}

pub fn allow_remote_access_mariadb(req: &RemoteAccessGrantRequest) -> Result<String, String> {
    let user = req.db_user.trim();
    let db_name = req.db_name.trim();
    let remote = req.remote_ip.trim().replace('*', "%");
    let pass = req.db_pass.as_deref().unwrap_or("").trim();
    if user.is_empty() || db_name.is_empty() || remote.is_empty() {
        return Err("db_user, db_name and remote_ip are required.".to_string());
    }
    if pass.is_empty() {
        return Err("db_pass is required.".to_string());
    }

    if !pass.is_empty() {
        run_mysql(&format!(
            "CREATE USER IF NOT EXISTS '{}'@'{}' IDENTIFIED BY '{}';",
            mysql_escape(user),
            mysql_escape(&remote),
            mysql_escape(pass)
        ))?;
    }
    run_mysql(&format!(
        "GRANT ALL PRIVILEGES ON `{}`.* TO '{}'@'{}'; FLUSH PRIVILEGES;",
        db_name,
        mysql_escape(user),
        mysql_escape(&remote)
    ))?;

    let mut rules = load_rules("mariadb");
    rules.push(RemoteAccessRule {
        engine: "mariadb".to_string(),
        db_user: user.to_string(),
        db_name: db_name.to_string(),
        remote: remote.to_string(),
        auth_method: "mysql-native-password".to_string(),
    });
    let _ = save_rules("mariadb", &rules);
    Ok(format!("MariaDB remote access granted for '{}'.", user))
}

pub fn list_remote_access_postgres(db_user: Option<&str>) -> Result<Vec<RemoteAccessRule>, String> {
    let mut rules = load_rules("postgresql");
    if let Some(user) = db_user {
        let user = user.trim();
        if !user.is_empty() {
            rules.retain(|x| x.db_user == user);
        }
    }
    Ok(rules)
}

pub fn allow_remote_access_postgres(req: &RemoteAccessGrantRequest) -> Result<String, String> {
    let user = req.db_user.trim();
    let db_name = req.db_name.trim();
    let remote = req.remote_ip.trim();
    if user.is_empty() || db_name.is_empty() || remote.is_empty() {
        return Err("db_user, db_name and remote_ip are required.".to_string());
    }
    let cidr = if remote.contains('/') {
        remote.to_string()
    } else {
        match remote.parse::<IpAddr>() {
            Ok(IpAddr::V4(_)) => format!("{}/32", remote),
            Ok(IpAddr::V6(_)) => format!("{}/128", remote),
            Err(_) => return Err("remote_ip must be a valid IP/CIDR for PostgreSQL.".to_string()),
        }
    };

    run_psql(&format!(
        "GRANT CONNECT ON DATABASE \"{}\" TO \"{}\";",
        pg_ident(db_name),
        pg_ident(user)
    ))?;

    let mut rules = load_rules("postgresql");
    rules.push(RemoteAccessRule {
        engine: "postgresql".to_string(),
        db_user: user.to_string(),
        db_name: db_name.to_string(),
        remote: cidr.clone(),
        auth_method: "scram-sha-256".to_string(),
    });
    let _ = save_rules("postgresql", &rules);
    Ok(format!("PostgreSQL remote access granted for '{}'.", user))
}

pub fn check_connection_readiness(engine: &str, db_name: &str, db_user: &str) -> Result<DbConnectionCheckResult, String> {
    let engine = engine.trim().to_lowercase();
    let database_exists;
    let user_exists;
    let remote_access_rules;

    if engine == "mariadb" || engine == "mysql" {
        database_exists = MariaDbManager::list_databases()?.iter().any(|x| x.name == db_name);
        user_exists = MariaDbManager::list_users()?.iter().any(|x| x.username == db_user);
        remote_access_rules = list_remote_access_mariadb(Some(db_user)).unwrap_or_default();
    } else if engine == "postgresql" || engine == "postgres" {
        database_exists = PostgresManager::list_databases()?.iter().any(|x| x.name == db_name);
        user_exists = PostgresManager::list_users()?.iter().any(|x| x.username == db_user);
        remote_access_rules = list_remote_access_postgres(Some(db_user)).unwrap_or_default();
    } else {
        return Err("Unsupported database engine.".to_string());
    }

    Ok(DbConnectionCheckResult {
        engine,
        db_name: db_name.to_string(),
        db_user: db_user.to_string(),
        database_exists,
        user_exists,
        ready: database_exists && user_exists,
        remote_access_rules,
    })
}
