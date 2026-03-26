use serde::{Deserialize, Serialize};
use std::fs;
use std::path::Path;
use std::process::Command;

#[derive(Debug, Serialize, Deserialize)]
pub struct VHostConfig {
    pub domain: String,
    pub user: String,
    pub php_version: String,
}

pub struct NitroEngine;

impl NitroEngine {
    /// Creates a new OpenLiteSpeed Virtual Host and its directories
    pub fn create_vhost(config: &VHostConfig) -> Result<(), String> {
        // 1. Dizinleri Oluştur
        let home_dir = format!("/home/{}", config.user);
        let public_html = format!("{}/public_html/{}", home_dir, config.domain);
        let vhost_conf_dir = format!("/usr/local/lsws/conf/vhosts/{}", config.domain);

        // Rust'ta dizinleri yarat (Linux yetkisi gerekir)
        // MVP için şimdilik hata döndürmeden (Eğer Windows'ta veya test ortamındaysak debug bas)
        if !Path::new("/usr/local/lsws").exists() {
            println!("[DEV MODE] OLS is not installed on this system. Simulating VHost Creation for {}", config.domain);
            return Ok(());
        }

        fs::create_dir_all(&public_html)
            .map_err(|e| format!("Ana dizin oluşturulamadı: {}", e))?;
        fs::create_dir_all(&vhost_conf_dir)
            .map_err(|e| format!("Vhost Config dizini oluşturulamadı: {}", e))?;

        // 2. Vhost Config Şablonunu Yaz
        let vhconf_content = format!(
            r#"
docRoot                   $VH_ROOT/public_html/{domain}
vhDomain                  {domain}
vhAliases                 www.{domain}
adminEmails               webmaster@{domain}
enableGzip                1

index  {{
  useServer               0
  indexFiles              index.php, index.html
}}

context / {{
  allowBrowse             1
  rewrite  {{
    enable                1
    autoLoadHtaccess      1
  }}
}}

extprocessor {domain}_php {{
  type                    lsapi
  address                 UDS://tmp/lshttpd/{domain}.sock
  maxConns                35
  env                     PHP_LSAPI_CHILDREN=35
  initTimeout             60
  retryTimeout            0
  persistConn             1
  respBuffer              0
  autoStart               1
  path                    /usr/local/lsws/lsphp{php_version}/bin/lsphp
  backlog                 100
  instances               1
  runOnStartUp            3
}}
            "#,
            domain = config.domain,
            php_version = config.php_version.replace(".", "")
        );

        let conf_file = format!("{}/vhconf.conf", vhost_conf_dir);
        fs::write(&conf_file, vhconf_content)
            .map_err(|e| format!("vhconf.conf yazılamadı: {}", e))?;

        // 3. OLS Ana Config Dosyasına Vhost'u Ekle (Simülasyon/Append)
        // Normalde htaccess veya ana OLS XML/conf uzerinde islem yapilir veya OLS Admin API cagrilir.
        // Bu adimi OLS API uzerinden veya Include directive ile yapiyoruz.

        // 4. OLS'yi Yarı Kesintisiz (Graceful) Yeniden Başlat
        Self::reload_ols()?;

        Ok(())
    }

    /// OLS sunucusuna graceful restart atar
    pub fn reload_ols() -> Result<(), String> {
        if !Path::new("/usr/local/lsws").exists() {
            println!("[DEV MODE] OLS Bulunamadı. Restart iptal edildi.");
            return Ok(());
        }

        let output = Command::new("/usr/local/lsws/bin/lswsctrl")
            .arg("restart")
            .output()
            .map_err(|e| format!("OLS komutu çalıştırılamadı: {}", e))?;

        if !output.status.success() {
            return Err(format!("OLS Restart başarısız: {:?}", output.stderr));
        }

        Ok(())
    }
}
