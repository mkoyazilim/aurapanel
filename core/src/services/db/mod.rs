use serde::{Deserialize, Serialize};
use std::process::Command;

pub mod auradb;

// ─── Ortak Yapılar ──────────────────────────────────────────

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct DbConfig {
    pub db_name: String,
    pub db_user: String,
    pub db_pass: String,
    pub host: Option<String>,
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

// ─── MariaDB Manager ────────────────────────────────────────

pub struct MariaDbManager;

impl MariaDbManager {
    fn run_sql(sql: &str) -> Result<String, String> {
        let output = Command::new("mysql")
            .args(["-u", "root", "-e", sql])
            .output();

        match output {
            Ok(o) if o.status.success() => Ok(String::from_utf8_lossy(&o.stdout).to_string()),
            Ok(o) => Err(String::from_utf8_lossy(&o.stderr).to_string()),
            Err(_) => {
                println!("[DEV MODE] MariaDB simülasyon: {}", sql);
                Ok("[simulated]".to_string())
            }
        }
    }

    pub fn create_database(config: &DbConfig) -> Result<String, String> {
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
        Ok(format!("MariaDB veritabanı '{}' ve kullanıcı '{}' oluşturuldu", config.db_name, config.db_user))
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
        if output.contains("[simulated]") {
            return Ok(vec![
                DatabaseInfo { name: "wp_blog".into(), engine: "mariadb".into(), size: "24.5 MB".into(), tables: 12 },
                DatabaseInfo { name: "app_prod".into(), engine: "mariadb".into(), size: "156.2 MB".into(), tables: 45 },
                DatabaseInfo { name: "ecommerce".into(), engine: "mariadb".into(), size: "512.8 MB".into(), tables: 78 },
            ]);
        }
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
        if output.contains("[simulated]") {
            return Ok(vec![
                DbUserInfo { username: "wp_user".into(), host: "localhost".into(), engine: "mariadb".into() },
                DbUserInfo { username: "app_user".into(), host: "localhost".into(), engine: "mariadb".into() },
            ]);
        }
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

// ─── PostgreSQL Manager ──────────────────────────────────────

pub struct PostgresManager;

impl PostgresManager {
    fn run_psql(sql: &str) -> Result<String, String> {
        let output = Command::new("sudo")
            .args(["-u", "postgres", "psql", "-c", sql])
            .output();

        match output {
            Ok(o) if o.status.success() => Ok(String::from_utf8_lossy(&o.stdout).to_string()),
            Ok(o) => Err(String::from_utf8_lossy(&o.stderr).to_string()),
            Err(_) => {
                println!("[DEV MODE] PostgreSQL simülasyon: {}", sql);
                Ok("[simulated]".to_string())
            }
        }
    }

    pub fn create_database(config: &DbConfig) -> Result<String, String> {
        // Kullanıcı oluştur
        Self::run_psql(&format!(
            "DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '{}') THEN CREATE ROLE \"{}\" LOGIN PASSWORD '{}'; END IF; END $$;",
            config.db_user, config.db_user, config.db_pass
        ))?;

        // Veritabanı oluştur
        let check = Self::run_psql(&format!(
            "SELECT 1 FROM pg_database WHERE datname = '{}';", config.db_name
        ))?;
        if !check.contains("1") || check.contains("[simulated]") {
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
                Err(_) => println!("[DEV MODE] createdb simülasyon: {}", config.db_name),
            }
        }

        // Yetki ver
        Self::run_psql(&format!(
            "GRANT ALL PRIVILEGES ON DATABASE \"{}\" TO \"{}\";",
            config.db_name, config.db_user
        ))?;

        Ok(format!("PostgreSQL veritabanı '{}' ve kullanıcı '{}' oluşturuldu", config.db_name, config.db_user))
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
            Err(_) => {
                println!("[DEV MODE] dropdb simülasyon: {}", db_name);
                Ok(format!("[DEV] PostgreSQL veritabanı '{}' silindi", db_name))
            }
        }
    }

    pub fn drop_user(username: &str) -> Result<String, String> {
        Self::run_psql(&format!("DROP ROLE IF EXISTS \"{}\";", username))?;
        Ok(format!("PostgreSQL kullanıcı '{}' silindi", username))
    }

    pub fn list_databases() -> Result<Vec<DatabaseInfo>, String> {
        let output = Self::run_psql("SELECT datname FROM pg_database WHERE datistemplate = false AND datname NOT IN ('postgres');")?;
        if output.contains("[simulated]") {
            return Ok(vec![
                DatabaseInfo { name: "analytics_db".into(), engine: "postgresql".into(), size: "89.3 MB".into(), tables: 23 },
                DatabaseInfo { name: "saas_app".into(), engine: "postgresql".into(), size: "1.2 GB".into(), tables: 134 },
            ]);
        }
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
        if output.contains("[simulated]") {
            return Ok(vec![
                DbUserInfo { username: "analytics_user".into(), host: "local".into(), engine: "postgresql".into() },
                DbUserInfo { username: "saas_user".into(), host: "local".into(), engine: "postgresql".into() },
            ]);
        }
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
