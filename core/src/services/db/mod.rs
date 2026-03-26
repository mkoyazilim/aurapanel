use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Debug)]
pub struct DbConfig {
    pub db_name: String,
    pub db_user: String,
    pub db_pass: String,
    pub host: Option<String>,
}

pub struct DbManager;

impl DbManager {
    /// Yeni bir MariaDB/MySQL Veritabanı ve Kullanıcısı oluşturur
    pub async fn create_database(config: &DbConfig) -> Result<(), String> {
        let host = config.host.as_deref().unwrap_or("localhost");
        
        println!("[DEV MODE] Creating Database: {} for user: {}@{}", config.db_name, config.db_user, host);

        // Gerçek implementasyonda MariaDB 'root' kullanıcısıyla (vey paneldeki sysadmin yetkili mariadb kullanıcısıyla) bağlanılır:
        // let pool = sqlx::MySqlPool::connect("mysql://root:PASS@localhost").await.unwrap();
        // 
        // 1. Veritabanı yarat:
        // sqlx::query(&format!("CREATE DATABASE IF NOT EXISTS `{}`", config.db_name)).execute(&pool).await?;
        //
        // 2. Kullanıcı yarat:
        // sqlx::query(&format!("CREATE USER IF NOT EXISTS '{}'@'{}' IDENTIFIED BY '{}'", config.db_user, host, config.db_pass)).execute(&pool).await?;
        //
        // 3. Yetkileri ver:
        // sqlx::query(&format!("GRANT ALL PRIVILEGES ON `{}`.* TO '{}'@'{}'", config.db_name, config.db_user, host)).execute(&pool).await?;
        //
        // 4. Yetkileri tazele:
        // sqlx::query("FLUSH PRIVILEGES").execute(&pool).await?;

        Ok(())
    }

    /// Veritabanı siler
    pub async fn drop_database(db_name: &str) -> Result<(), String> {
        println!("[DEV MODE] Dropping Database: {}", db_name);
        // let pool = sqlx::MySqlPool::connect("...").await?;
        // sqlx::query(&format!("DROP DATABASE IF EXISTS `{}`", db_name)).execute(&pool).await?;
        Ok(())
    }
}
