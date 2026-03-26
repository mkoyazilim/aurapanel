use serde::{Deserialize, Serialize};
use reqwest::{Client, Error}; // Requires reqwest crate

#[derive(Serialize, Deserialize)]
pub struct DnsRecord {
    pub content: String,
    pub disabled: bool,
    pub name: String,
    pub ttl: u32,
    pub r#type: String, // A, MX, TXT
}

pub struct PowerDnsClient {
    base_url: String,
    api_key: String,
    client: Client,
}

impl PowerDnsClient {
    pub fn new(base_url: &str, api_key: &str) -> Self {
        Self {
            base_url: base_url.to_string(),
            api_key: api_key.to_string(),
            client: Client::new(),
        }
    }

    pub async fn get_zones(&self) -> Result<String, Error> {
        let url = format!("{}/api/v1/servers/localhost/zones", self.base_url);
        let res = self.client.get(&url)
            .header("X-API-Key", &self.api_key)
            .send()
            .await?;
        
        let body = res.text().await?;
        Ok(body)
    }

    pub async fn create_zone(&self, domain: &str) -> Result<bool, Error> {
        let url = format!("{}/api/v1/servers/localhost/zones", self.base_url);
        
        let payload = serde_json::json!({
            "name": format!("{}.", domain),
            "kind": "Native",
            "nameservers": ["ns1.aurapanel.local", "ns2.aurapanel.local"]
        });

        let res = self.client.post(&url)
            .header("X-API-Key", &self.api_key)
            .json(&payload)
            .send()
            .await?;
            
        Ok(res.status().is_success())
    }
}
