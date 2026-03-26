use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Debug)]
pub struct MailboxConfig {
    pub domain: String,
    pub username: String,
    pub password: String,
    pub quota_mb: u32,
}

pub struct MailManager;

impl MailManager {
    /// Yeni bir e-posta kutusu oluşturur (Stalwart JMAP/REST API kullanılarak)
    pub async fn create_mailbox(config: &MailboxConfig) -> Result<(), String> {
        let email_address = format!("{}@{}", config.username, config.domain);
        println!("[DEV MODE] Creating Mailbox: {} with {} MB quota", email_address, config.quota_mb);

        // Gerçek implementasyonda Stalwart REST API veya JMAP veya Directory üzerinden hesap açılır.
        /*
        let payload = serde_json::json!({
            "name": config.username,
            "secret": config.password,
            "quota": config.quota_mb * 1024 * 1024,
            "description": format!("Auto-created via AuraPanel for {}", config.domain)
        });

        let client = reqwest::Client::new();
        let _res = client.put(&format!("http://127.0.0.1:8080/api/directory/accounts/{}", email_address))
            .header("Authorization", "Bearer stalward_admin_secret_token")
            .json(&payload)
            .send()
            .await
            .map_err(|e| format!("Stalwart sunucusuna ulaşılamadı: {}", e))?;
        */

        Ok(())
    }

    pub async fn delete_mailbox(email: &str) -> Result<(), String> {
        println!("[DEV MODE] Deleting Mailbox: {}", email);
        Ok(())
    }
}
