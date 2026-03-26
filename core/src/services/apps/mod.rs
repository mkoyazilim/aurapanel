use serde::{Deserialize, Serialize};
use std::process::Command;

#[derive(Serialize, Deserialize, Debug)]
pub struct CmsInstallConfig {
    pub domain: String,
    pub app_type: String, // "wordpress", "laravel"
    pub db_name: String,
    pub db_user: String,
    pub db_pass: String,
}

pub struct AppManager;

impl AppManager {
    /// Tek tıklamayla WordPress, Laravel veya benzeri CMS'leri indirip kurar
    pub async fn install_cms(config: &CmsInstallConfig) -> Result<(), String> {
        println!("[DEV MODE] Installing {} on {}", config.app_type, config.domain);

        let public_html = format!("/home/aurapanel/public_html/{}", config.domain);

        match config.app_type.as_str() {
            "wordpress" => {
                // Şimdilik wp-cli komutlarını simüle ediyoruz.
                // Command::new("wp").arg("core").arg("download").arg("--path=".to_owned() + &public_html).output();
                println!("WordPress indirme ve wp-config kurulumu simüle edildi.");
            },
            "laravel" => {
                // Command::new("composer").arg("create-project").arg("--prefer-dist").arg("laravel/laravel").arg(&public_html).output();
                println!("Laravel skeleton indirmesi simüle edildi.");
            },
            _ => return Err(format!("Desteklenmeyen uygulama tipi: {}", config.app_type)),
        }

        Ok(())
    }
}
