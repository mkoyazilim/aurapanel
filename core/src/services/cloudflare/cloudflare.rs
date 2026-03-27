use serde::{Deserialize, Serialize};
use std::process::Command;

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct CloudFlareConfig {
    pub api_key: String,
    pub email: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct CfZone {
    pub id: String,
    pub name: String,
    pub status: String,
    pub name_servers: Vec<String>,
    pub plan: String,
    pub ssl_mode: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct CfDnsRecord {
    pub id: String,
    pub r#type: String,
    pub name: String,
    pub content: String,
    pub ttl: u32,
    pub proxied: bool,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct CreateDnsRecordRequest {
    pub zone_id: String,
    pub r#type: String,
    pub name: String,
    pub content: String,
    pub ttl: Option<u32>,
    pub proxied: Option<bool>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct DeleteDnsRecordRequest {
    pub zone_id: String,
    pub record_id: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct SetSslModeRequest {
    pub zone_id: String,
    pub mode: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct PurgeCacheRequest {
    pub zone_id: String,
    pub purge_everything: Option<bool>,
    pub files: Option<Vec<String>>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct SetSecurityLevelRequest {
    pub zone_id: String,
    pub level: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct DevModeRequest {
    pub zone_id: String,
    pub enabled: bool,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct PageRuleRequest {
    pub zone_id: String,
    pub url_pattern: String,
    pub actions: Vec<PageRuleAction>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct PageRuleAction {
    pub id: String,
    pub value: serde_json::Value,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct CfPageRule {
    pub id: String,
    pub targets: Vec<String>,
    pub actions: Vec<String>,
    pub status: String,
    pub priority: u32,
}

pub struct CloudFlareManager;

impl CloudFlareManager {
    fn api_call(
        method: &str,
        endpoint: &str,
        api_key: &str,
        email: &str,
        body: Option<&str>,
    ) -> Result<String, String> {
        let url = format!("https://api.cloudflare.com/client/v4{}", endpoint);

        let mut cmd = Command::new("curl");
        cmd.args(["-s", "-X", method, &url])
            .args(["-H", &format!("X-Auth-Email: {}", email)])
            .args(["-H", &format!("X-Auth-Key: {}", api_key)])
            .args(["-H", "Content-Type: application/json"]);

        if let Some(payload) = body {
            cmd.args(["-d", payload]);
        }

        let output = cmd
            .output()
            .map_err(|e| format!("CloudFlare API call failed ({} {}): {}", method, endpoint, e))?;

        if output.status.success() {
            Ok(String::from_utf8_lossy(&output.stdout).to_string())
        } else {
            Err(String::from_utf8_lossy(&output.stderr).to_string())
        }
    }

    pub fn list_zones(api_key: &str, email: &str) -> Result<Vec<CfZone>, String> {
        let resp = Self::api_call("GET", "/zones?per_page=50", api_key, email, None)?;
        let parsed: serde_json::Value =
            serde_json::from_str(&resp).map_err(|e| format!("JSON parse error: {}", e))?;

        if parsed["success"].as_bool() != Some(true) {
            return Err(format!("CF API error: {:?}", parsed["errors"]));
        }

        let zones: Vec<CfZone> = parsed["result"]
            .as_array()
            .unwrap_or(&vec![])
            .iter()
            .map(|z| CfZone {
                id: z["id"].as_str().unwrap_or("").to_string(),
                name: z["name"].as_str().unwrap_or("").to_string(),
                status: z["status"].as_str().unwrap_or("").to_string(),
                name_servers: z["name_servers"]
                    .as_array()
                    .map(|ns| {
                        ns.iter()
                            .map(|n| n.as_str().unwrap_or("").to_string())
                            .collect()
                    })
                    .unwrap_or_default(),
                plan: z["plan"]["name"].as_str().unwrap_or("Free").to_string(),
                ssl_mode: String::new(),
            })
            .collect();

        Ok(zones)
    }

    pub fn list_dns_records(
        api_key: &str,
        email: &str,
        zone_id: &str,
    ) -> Result<Vec<CfDnsRecord>, String> {
        let endpoint = format!("/zones/{}/dns_records?per_page=100", zone_id);
        let resp = Self::api_call("GET", &endpoint, api_key, email, None)?;
        let parsed: serde_json::Value =
            serde_json::from_str(&resp).map_err(|e| format!("JSON parse error: {}", e))?;

        if parsed["success"].as_bool() != Some(true) {
            return Err(format!("CF API error: {:?}", parsed["errors"]));
        }

        let records: Vec<CfDnsRecord> = parsed["result"]
            .as_array()
            .unwrap_or(&vec![])
            .iter()
            .map(|r| CfDnsRecord {
                id: r["id"].as_str().unwrap_or("").to_string(),
                r#type: r["type"].as_str().unwrap_or("").to_string(),
                name: r["name"].as_str().unwrap_or("").to_string(),
                content: r["content"].as_str().unwrap_or("").to_string(),
                ttl: r["ttl"].as_u64().unwrap_or(1) as u32,
                proxied: r["proxied"].as_bool().unwrap_or(false),
            })
            .collect();

        Ok(records)
    }

    pub fn create_dns_record(
        api_key: &str,
        email: &str,
        req: &CreateDnsRecordRequest,
    ) -> Result<String, String> {
        let endpoint = format!("/zones/{}/dns_records", req.zone_id);
        let body = serde_json::json!({
            "type": req.r#type,
            "name": req.name,
            "content": req.content,
            "ttl": req.ttl.unwrap_or(1),
            "proxied": req.proxied.unwrap_or(false),
        });

        let resp = Self::api_call("POST", &endpoint, api_key, email, Some(&body.to_string()))?;
        let parsed: serde_json::Value = serde_json::from_str(&resp).unwrap_or_default();
        if parsed["success"].as_bool() == Some(true) {
            Ok(format!(
                "DNS record created: {} {} -> {}",
                req.r#type, req.name, req.content
            ))
        } else {
            Err(format!("CF API error: {:?}", parsed["errors"]))
        }
    }

    pub fn delete_dns_record(
        api_key: &str,
        email: &str,
        req: &DeleteDnsRecordRequest,
    ) -> Result<String, String> {
        let endpoint = format!("/zones/{}/dns_records/{}", req.zone_id, req.record_id);
        let resp = Self::api_call("DELETE", &endpoint, api_key, email, None)?;
        let parsed: serde_json::Value = serde_json::from_str(&resp).unwrap_or_default();
        if parsed["success"].as_bool() == Some(true) {
            Ok("DNS record deleted".to_string())
        } else {
            Err(format!("CF API error: {:?}", parsed["errors"]))
        }
    }

    pub fn set_ssl_mode(
        api_key: &str,
        email: &str,
        req: &SetSslModeRequest,
    ) -> Result<String, String> {
        let endpoint = format!("/zones/{}/settings/ssl", req.zone_id);
        let body = serde_json::json!({ "value": req.mode });
        Self::api_call("PATCH", &endpoint, api_key, email, Some(&body.to_string()))
            .map(|_| format!("SSL mode updated: {}", req.mode))
    }

    pub fn purge_cache(
        api_key: &str,
        email: &str,
        req: &PurgeCacheRequest,
    ) -> Result<String, String> {
        let endpoint = format!("/zones/{}/purge_cache", req.zone_id);
        let body = if req.purge_everything.unwrap_or(false) {
            serde_json::json!({ "purge_everything": true })
        } else {
            serde_json::json!({ "files": req.files.clone().unwrap_or_default() })
        };

        Self::api_call("POST", &endpoint, api_key, email, Some(&body.to_string()))
            .map(|_| "Cache purge completed".to_string())
    }

    pub fn set_security_level(
        api_key: &str,
        email: &str,
        req: &SetSecurityLevelRequest,
    ) -> Result<String, String> {
        let endpoint = format!("/zones/{}/settings/security_level", req.zone_id);
        let body = serde_json::json!({ "value": req.level });
        Self::api_call("PATCH", &endpoint, api_key, email, Some(&body.to_string()))
            .map(|_| format!("Security level updated: {}", req.level))
    }

    pub fn set_dev_mode(
        api_key: &str,
        email: &str,
        req: &DevModeRequest,
    ) -> Result<String, String> {
        let endpoint = format!("/zones/{}/settings/development_mode", req.zone_id);
        let body = serde_json::json!({ "value": if req.enabled { "on" } else { "off" } });
        Self::api_call("PATCH", &endpoint, api_key, email, Some(&body.to_string())).map(|_| {
            format!(
                "Development mode: {}",
                if req.enabled { "enabled" } else { "disabled" }
            )
        })
    }

    pub fn set_always_https(
        api_key: &str,
        email: &str,
        zone_id: &str,
        enabled: bool,
    ) -> Result<String, String> {
        let endpoint = format!("/zones/{}/settings/always_use_https", zone_id);
        let body = serde_json::json!({ "value": if enabled { "on" } else { "off" } });
        Self::api_call("PATCH", &endpoint, api_key, email, Some(&body.to_string())).map(|_| {
            format!(
                "Always HTTPS: {}",
                if enabled { "enabled" } else { "disabled" }
            )
        })
    }

    pub fn set_minify(
        api_key: &str,
        email: &str,
        zone_id: &str,
        js: bool,
        css: bool,
        html: bool,
    ) -> Result<String, String> {
        let endpoint = format!("/zones/{}/settings/minify", zone_id);
        let body = serde_json::json!({
            "value": {
                "js": if js { "on" } else { "off" },
                "css": if css { "on" } else { "off" },
                "html": if html { "on" } else { "off" }
            }
        });

        Self::api_call("PATCH", &endpoint, api_key, email, Some(&body.to_string()))
            .map(|_| "Minify settings updated".to_string())
    }

    pub fn get_zone_analytics(
        api_key: &str,
        email: &str,
        zone_id: &str,
    ) -> Result<serde_json::Value, String> {
        let endpoint = format!("/zones/{}/analytics/dashboard?since=-1440&continuous=true", zone_id);
        let resp = Self::api_call("GET", &endpoint, api_key, email, None)?;
        serde_json::from_str(&resp).map_err(|e| e.to_string())
    }
}
