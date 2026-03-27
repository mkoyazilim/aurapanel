use serde::{Deserialize, Serialize};
use std::process::Command;

pub mod auradb;
pub mod ops;

// ¦¦¦ Ortak Yapılar ¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct DbConfig {
    pub db_name: String,
    pub db_user: String,
    pub db_pass: String,
    pub host: Option<String>,
    pub site_domain: Option<String>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct DatabaseInfo {
    pub name: String,
    pub engine: String, // "mariadb" || "postgresql"
    pub size: String,
    pub tables: u32,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct DbUserInfo {
    pub username: String,
    pub host: String,
    pub engine: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct DbCreateResult {
    pub message: String,
    pub db_name: String,
    pub db_user: String,
    pub engine: String,
    pub site_domain: Option<String>,
}

// ¦¦¦ MariaDB Manager ¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦

pub struct MariaDbManager;

impl MariaDbManager {
    fn run_sql(sql: &str) -> Result<String, String> {
        let output = Command::new("mysql")
            .args(["-u", "root", "-e", sql])
            .output();

        match output {
            Ok(o) if o.status.success() => Ok(String::from_utf8_lossy(&o.stdout).to_string()),
            Ok(o) => Err(String::from_utf8_lossy(&o.stderr).to_string()),
            Err(e) => Err(format!("mysql command failed: {}", e)),
        }
    }

    pub fn create_database(config: &DbConfig) -> Result<DbCreateResult, String> {
        let host = config.host.as_deref().unwrap_or("localhost");
        let sqls = vec![
            format!("CREATE DATABASE IF NOT EXISTS `{}` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", config.db_name),
            format!("CREATE USER IF NOT EXISTS '{}'@'{}' IDENTIFIED BY '{}';", config.db_user, host, config.db_pass),
            format!("GRANT ALL PRIVILEGES ON `{}`.* TO '{}'@'{}';", config.db_name, config.db_user, host),
            "FLUSH PRIVILEGES;".to_string(),
        ];
        for sql in &sqls {
            Self::run_sql(sql)?;
        }
        Ok(DbCreateResult {
            message: format!("MariaDB veritabanı '{}' ve kullanıcı '{}' oluşturuldu", config.db_name, config.db_user),
            db_name: config.db_name.clone(),
            db_user: config.db_user.clone(),
            engine: "mariadb".to_string(),
            site_domain: config.site_domain.clone(),
        })
    }

    pub fn drop_database(db_name: &str) -> Result<String, String> {
        Self::run_sql(&format!("DROP DATABASE IF EXISTS `{}`;", db_name))?;
        Ok(format!("MariaDB veritabanı '{}' silindi", db_name))
    }

    pub fn drop_user(username: &str, host: &str) -> Result<String, String> {
        Self::run_sql(&format!("DROP USER IF EXISTS '{}'@'{}';", username, host))?;
        Self::run_sql("FLUSH PRIVILEGES;")?;
        Ok(format!("MariaDB kullanıcı '{}' silindi", username))
    }

    pub fn list_databases() -> Result<Vec<DatabaseInfo>, String> {
        let output = Self::run_sql("SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME NOT IN ('information_schema','mysql','performance_schema','sys');")?;
        let dbs: Vec<DatabaseInfo> = output.lines()
            .skip(1)
            .filter(|l| !l.is_empty())
            .map(|l| DatabaseInfo {
                name: l.trim().to_string(),
                engine: "mariadb".into(),
                size: "—".into(),
                tables: 0,
            })
            .collect();
        Ok(dbs)
    }

    pub fn list_users() -> Result<Vec<DbUserInfo>, String> {
        let output = Self::run_sql("SELECT User, Host FROM mysql.user WHERE User NOT IN ('root','mysql.sys','mysql.session','mariadb.sys','debian-sys-maint');")?;
        let users: Vec<DbUserInfo> = output.lines()
            .skip(1)
            .filter(|l| !l.is_empty())
            .map(|l| {
                let parts: Vec<&str> = l.split_whitespace().collect();
                DbUserInfo {
                    username: parts.first().unwrap_or(&"").to_string(),
                    host: parts.get(1).unwrap_or(&"localhost").to_string(),
                    engine: "mariadb".into(),
                }
            })
            .collect();
        Ok(users)
    }
}

// ¦¦¦ PostgreSQL Manager ¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦¦

pub struct PostgresManager;

impl PostgresManager {
    fn run_psql(sql: &str) -> Result<String, String> {
        let output = Command::new("sudo")
            .args(["-u", "postgres", "psql", "-c", sql])
            .output();

        match output {
            Ok(o) if o.status.success() => Ok(String::from_utf8_lossy(&o.stdout).to_string()),
            Ok(o) => Err(String::from_utf8_lossy(&o.stderr).to_string()),
            Err(e) => Err(format!("psql command failed: {}", e)),
        }
    }

    pub fn create_database(config: &DbConfig) -> Result<DbCreateResult, String> {
        // Kullanıcı oluştur
        Self::run_psql(&format!(
            "DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '{}') THEN CREATE ROLE \"{}\" LOGIN PASSWORD '{}'; END IF; END $$;",
            config.db_user, config.db_user, config.db_pass
        ))?;

        // Veritabanı oluştur
        let check = Self::run_psql(&format!(
            "SELECT 1 FROM pg_database WHERE datname = '{}';", config.db_name
        ))?;
        if !check.contains("1") {
            // createdb komutu daha güvenli (psql CREATE DATABASE transaction'da çalışmaz)
            let output = Command::new("sudo")
                .args(["-u", "postgres", "createdb", &config.db_name, "-O", &config.db_user])
                .output();
            match output {
                Ok(o) if o.status.success() => {},
                Ok(o) => {
                    let err = String::from_utf8_lossy(&o.stderr).to_string();
                    if !err.contains("already exists") {
                        return Err(err);
                    }
                },
                Err(e) => return Err(format!("createdb command failed: {}", e)),
            }
        }

        // Yetki ver
        Self::run_psql(&format!(
            "GRANT ALL PRIVILEGES ON DATABASE \"{}\" TO \"{}\";",
            config.db_name, config.db_user
        ))?;

        Ok(DbCreateResult {
            message: format!("PostgreSQL veritabanı '{}' ve kullanıcı '{}' oluşturuldu", config.db_name, config.db_user),
            db_name: config.db_name.clone(),
            db_user: config.db_user.clone(),
            engine: "postgresql".to_string(),
            site_domain: config.site_domain.clone(),
        })
    }

    pub fn drop_database(db_name: &str) -> Result<String, String> {
        // Önce aktif bağlantıları kes
        let _ = Self::run_psql(&format!(
            "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '{}' AND pid <> pg_backend_pid();",
            db_name
        ));
        let output = Command::new("sudo")
            .args(["-u", "postgres", "dropdb", "--if-exists", db_name])
            .output();
        match output {
            Ok(o) if o.status.success() => Ok(format!("PostgreSQL veritabanı '{}' silindi", db_name)),
            Ok(o) => Err(String::from_utf8_lossy(&o.stderr).to_string()),
            Err(e) => Err(format!("dropdb command failed: {}", e)),
        }
    }

    pub fn drop_user(username: &str) -> Result<String, String> {
        Self::run_psql(&format!("DROP ROLE IF EXISTS \"{}\";", username))?;
        Ok(format!("PostgreSQL kullanıcı '{}' silindi", username))
    }

    pub fn list_databases() -> Result<Vec<DatabaseInfo>, String> {
        let output = Self::run_psql("SELECT datname FROM pg_database WHERE datistemplate = false AND datname NOT IN ('postgres');")?;
        let dbs: Vec<DatabaseInfo> = output.lines()
            .skip(2) // psql header
            .filter(|l| !l.trim().is_empty() && !l.contains("rows)") && !l.starts_with("---"))
            .map(|l| DatabaseInfo {
                name: l.trim().to_string(),
                engine: "postgresql".into(),
                size: "—".into(),
                tables: 0,
            })
            .collect();
        Ok(dbs)
    }

    pub fn list_users() -> Result<Vec<DbUserInfo>, String> {
        let output = Self::run_psql("SELECT rolname FROM pg_roles WHERE rolcanlogin = true AND rolname NOT IN ('postgres');")?;
        let users: Vec<DbUserInfo> = output.lines()
            .skip(2)
            .filter(|l| !l.trim().is_empty() && !l.contains("rows)") && !l.starts_with("---"))
            .map(|l| DbUserInfo {
                username: l.trim().to_string(),
                host: "local".into(),
                engine: "postgresql".into(),
            })
            .collect();
        Ok(users)
    }
}

