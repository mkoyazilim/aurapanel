use serde::{Deserialize, Serialize};
use std::fs;
use anyhow::{Result, Context};

#[derive(Debug, Serialize, Deserialize)]
pub struct ServerConfig {
    pub host: String,
    pub port: u16,
    pub mode: String, // debug, release
}

#[derive(Debug, Serialize, Deserialize)]
pub struct DbConfig {
    pub primary_uri: String,
    pub clickhouse_uri: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Settings {
    pub server: ServerConfig,
    pub database: DbConfig,
}

pub fn load_config(path: &str) -> Result<Settings> {
    let config_content = fs::read_to_string(path)
        .with_context(|| format!("Failed to read config file at {}", path))?;
    
    let settings: Settings = toml::from_str(&config_content)
        .with_context(|| "Failed to parse TOML configuration")?;
    
    Ok(settings)
}
