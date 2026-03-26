use reqwest::Client;
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize)]
pub struct DnsRecord {
    pub name: String,
    pub record_type: String, // A, MX, TXT, CNAME
    pub content: String,
    pub ttl: u32,
}

#[derive(Serialize, Deserialize)]
pub struct DnsZoneConfig {
    pub domain: String,
    pub server_ip: String,
}

pub struct PowerDnsManager {
    api_url: String,
    api_key: String,
    client: Client,
}

impl PowerDnsManager {
    pub fn new() -> Self {
        // Normalde config dosyasından okunacak
        Self {
            api_url: "http://127.0.0.1:8081/api/v1/servers/localhost".to_string(),
            api_key: "aurapanel_pdns_secret".to_string(),
            client: Client::new(),
        }
    }

    /// Yeni bir DNS Zone (Alan Adı) oluşturur
    pub async fn create_zone(&self, config: &DnsZoneConfig) -> Result<(), String> {
        let payload = serde_json::json!({
            "name": format!("{}.", config.domain),
            "kind": "Native",
            "nameservers": [
                format!("ns1.{}.", config.domain),
                format!("ns2.{}.", config.domain),
            ]
        });

        println!("[DEV MODE] Creating PowerDNS Zone for: {}", config.domain);

        /* Gerçek API entegrasyonu:
        let res = self.client.post(&format!("{}/zones", self.api_url))
            .header("X-API-Key", &self.api_key)
            .json(&payload)
            .send()
            .await
            .map_err(|e| format!("PowerDNS'e ulaşılamadı: {}", e))?;

        if !res.status().is_success() {
            return Err(format!("Zone oluşturulamadı. Kod: {}", res.status()));
        }
        */

        // Otomatik A Kaydı ekle
        self.add_record(&config.domain, DnsRecord {
            name: format!("{}.", config.domain),
            record_type: "A".to_string(),
            content: config.server_ip.clone(),
            ttl: 3600,
        }).await?;

        // Otomatik www kaydı ekle (CNAME)
        self.add_record(&config.domain, DnsRecord {
            name: format!("www.{}.", config.domain),
            record_type: "CNAME".to_string(),
            content: format!("{}.", config.domain),
            ttl: 3600,
        }).await?;

        Ok(())
    }

    /// Mevcut bir Zone'a kayıt ekler
    pub async fn add_record(&self, domain: &str, record: DnsRecord) -> Result<(), String> {
        let payload = serde_json::json!({
            "rrsets": [
                {
                    "name": record.name,
                    "type": record.record_type,
                    "ttl": record.ttl,
                    "changetype": "REPLACE",
                    "records": [
                        {
                            "content": record.content,
                            "disabled": false
                        }
                    ]
                }
            ]
        });

        println!("[DEV MODE] Adding DNS Record: {} -> {} ({})", record.name, record.content, record.record_type);

        /*
        let res = self.client.patch(&format!("{}/zones/{}.,", self.api_url, domain))
            .header("X-API-Key", &self.api_key)
            .json(&payload)
            .send()
            .await
            .map_err(|e| format!("PowerDNS Record eklenemedi: {}", e))?;

        if !res.status().is_success() {
            return Err(format!("Kayıt eklenemedi. Kod: {}", res.status()));
        }
        */

        Ok(())
    }
}
