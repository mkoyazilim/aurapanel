use reqwest::Client;
use serde::{Deserialize, Serialize};
use std::collections::{HashMap, HashSet};

#[derive(Serialize, Deserialize, Clone)]
pub struct DnsRecord {
    pub name: String,
    pub record_type: String, // A, MX, TXT, CNAME, NS
    pub content: String,
    pub ttl: u32,
}

#[derive(Serialize, Deserialize)]
pub struct DnsZoneConfig {
    pub domain: String,
    pub server_ip: String,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct DefaultNameservers {
    pub ns1: String,
    pub ns2: String,
}

impl Default for DefaultNameservers {
    fn default() -> Self {
        Self {
            ns1: "ns1.example.com".to_string(),
            ns2: "ns2.example.com".to_string(),
        }
    }
}

pub struct PowerDnsManager {
    #[allow(dead_code)]
    api_url: String,
    #[allow(dead_code)]
    api_key: String,
    #[allow(dead_code)]
    client: Client,
    storage_path: String,
    records_path: String,
    managed_path: String,
    config_path: String,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct LocalZone {
    pub id: u32,
    pub name: String,
    pub kind: String,
    pub records: u32,
    #[serde(default)]
    pub dnssec_enabled: bool,
}

#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct DnsReconcileResult {
    pub domain: String,
    pub zone_created: bool,
    pub added: u32,
    pub updated: u32,
    pub removed: u32,
    pub total_records: u32,
}

#[derive(Serialize, Deserialize, Clone, Debug, Eq, PartialEq, Hash)]
struct ManagedRecordKey {
    name: String,
    record_type: String,
}

impl PowerDnsManager {
    pub fn new() -> Self {
        std::fs::create_dir_all("/var/lib/aurapanel").unwrap_or_default();
        Self {
            api_url: "http://127.0.0.1:8081/api/v1/servers/localhost".to_string(),
            api_key: "aurapanel_pdns_secret".to_string(),
            client: Client::new(),
            storage_path: "/var/lib/aurapanel/dns_zones.json".to_string(),
            records_path: "/var/lib/aurapanel/dns_records.json".to_string(),
            managed_path: "/var/lib/aurapanel/dns_managed_keys.json".to_string(),
            config_path: "/var/lib/aurapanel/dns_config.json".to_string(),
        }
    }

    fn read_zones(&self) -> Vec<LocalZone> {
        if let Ok(data) = std::fs::read_to_string(&self.storage_path) {
            serde_json::from_str(&data).unwrap_or_default()
        } else {
            Vec::new()
        }
    }

    fn write_zones(&self, zones: &[LocalZone]) {
        if let Ok(data) = serde_json::to_string_pretty(zones) {
            let _ = std::fs::write(&self.storage_path, data);
        }
    }

    fn read_all_records(&self) -> HashMap<String, Vec<DnsRecord>> {
        if let Ok(data) = std::fs::read_to_string(&self.records_path) {
            serde_json::from_str(&data).unwrap_or_default()
        } else {
            HashMap::new()
        }
    }

    fn write_all_records(&self, records: &HashMap<String, Vec<DnsRecord>>) {
        if let Ok(data) = serde_json::to_string_pretty(records) {
            let _ = std::fs::write(&self.records_path, data);
        }
    }

    fn read_managed_keys(&self) -> HashMap<String, Vec<ManagedRecordKey>> {
        if let Ok(data) = std::fs::read_to_string(&self.managed_path) {
            serde_json::from_str(&data).unwrap_or_default()
        } else {
            HashMap::new()
        }
    }

    fn write_managed_keys(&self, managed: &HashMap<String, Vec<ManagedRecordKey>>) {
        if let Ok(data) = serde_json::to_string_pretty(managed) {
            let _ = std::fs::write(&self.managed_path, data);
        }
    }

    fn update_zone_record_count(&self, domain: &str, count: usize) {
        let mut zones = self.read_zones();
        let target_name = format!("{}.", domain);
        if let Some(zone) = zones.iter_mut().find(|z| z.name == target_name) {
            zone.records = count as u32;
            self.write_zones(&zones);
        }
    }

    fn update_zone_dnssec(&self, domain: &str, enabled: bool) -> Result<LocalZone, String> {
        let mut zones = self.read_zones();
        let target_name = format!("{}.", Self::normalize_domain(domain));
        let zone = zones
            .iter_mut()
            .find(|z| z.name == target_name)
            .ok_or_else(|| "Zone bulunamadi.".to_string())?;
        zone.dnssec_enabled = enabled;
        let updated = zone.clone();
        self.write_zones(&zones);
        Ok(updated)
    }

    fn normalize_host(host: &str) -> String {
        let trimmed = host.trim().trim_end_matches('.').to_lowercase();
        if trimmed.is_empty() {
            ".".to_string()
        } else {
            format!("{}.", trimmed)
        }
    }

    fn normalize_domain(domain: &str) -> String {
        domain.trim().trim_end_matches('.').to_lowercase()
    }

    fn default_records_for_domain(&self, domain: &str, server_ip: &str) -> Vec<DnsRecord> {
        let defaults = self.get_default_nameservers();
        let apex = format!("{}.", domain);
        let mail_host = format!("mail.{}.", domain);

        let mut records = vec![
            DnsRecord {
                name: apex.clone(),
                record_type: "A".to_string(),
                content: server_ip.to_string(),
                ttl: 3600,
            },
            DnsRecord {
                name: format!("www.{}.", domain),
                record_type: "CNAME".to_string(),
                content: apex.clone(),
                ttl: 3600,
            },
            DnsRecord {
                name: apex.clone(),
                record_type: "NS".to_string(),
                content: Self::normalize_host(&defaults.ns1),
                ttl: 3600,
            },
            DnsRecord {
                name: apex.clone(),
                record_type: "NS".to_string(),
                content: Self::normalize_host(&defaults.ns2),
                ttl: 3600,
            },
            DnsRecord {
                name: mail_host.clone(),
                record_type: "A".to_string(),
                content: server_ip.to_string(),
                ttl: 3600,
            },
            DnsRecord {
                name: apex.clone(),
                record_type: "MX".to_string(),
                content: format!("10 {}", mail_host),
                ttl: 3600,
            },
            DnsRecord {
                name: apex.clone(),
                record_type: "TXT".to_string(),
                content: format!("\"v=spf1 a mx ip4:{} ~all\"", server_ip),
                ttl: 3600,
            },
            DnsRecord {
                name: format!("_dmarc.{}.", domain),
                record_type: "TXT".to_string(),
                content: format!("\"v=DMARC1; p=none; rua=mailto:postmaster@{}\"", domain),
                ttl: 3600,
            },
            DnsRecord {
                name: format!("autodiscover.{}.", domain),
                record_type: "CNAME".to_string(),
                content: mail_host.clone(),
                ttl: 3600,
            },
            DnsRecord {
                name: format!("autoconfig.{}.", domain),
                record_type: "CNAME".to_string(),
                content: mail_host,
                ttl: 3600,
            },
        ];

        for host in [&defaults.ns1, &defaults.ns2] {
            let normalized = host.trim().trim_end_matches('.').to_lowercase();
            if normalized.ends_with(&format!(".{}", domain)) {
                records.push(DnsRecord {
                    name: format!("{}.", normalized),
                    record_type: "A".to_string(),
                    content: server_ip.to_string(),
                    ttl: 3600,
                });
            }
        }

        records
    }

    fn ensure_zone_exists(&self, domain: &str) -> bool {
        let mut zones = self.read_zones();
        let domain = Self::normalize_domain(domain);
        let zone_name = format!("{}.", domain);
        if zones.iter().any(|z| z.name == zone_name) {
            return false;
        }

        let next_id = zones.iter().map(|z| z.id).max().unwrap_or(0) + 1;
        zones.push(LocalZone {
            id: next_id,
            name: zone_name,
            kind: "Native".to_string(),
            records: 0,
            dnssec_enabled: false,
        });
        self.write_zones(&zones);
        true
    }

    pub fn reconcile_zone_defaults(&self, domain: &str, server_ip: &str) -> Result<DnsReconcileResult, String> {
        let domain = Self::normalize_domain(domain);
        if domain.is_empty() {
            return Err("Domain bos olamaz.".to_string());
        }
        let server_ip = server_ip.trim().to_string();
        if server_ip.is_empty() {
            return Err("server_ip zorunludur.".to_string());
        }

        let zone_created = self.ensure_zone_exists(&domain);
        let desired_records = self.default_records_for_domain(&domain, &server_ip);

        let desired_keys: HashSet<ManagedRecordKey> = desired_records
            .iter()
            .map(|r| ManagedRecordKey {
                name: r.name.clone(),
                record_type: r.record_type.clone(),
            })
            .collect();

        let mut all_records = self.read_all_records();
        let mut domain_records = all_records.remove(&domain).unwrap_or_default();

        let mut managed_map = self.read_managed_keys();
        let previous_managed: HashSet<ManagedRecordKey> = managed_map
            .remove(&domain)
            .unwrap_or_default()
            .into_iter()
            .collect();

        let mut removed: u32 = 0;
        domain_records.retain(|rec| {
            let key = ManagedRecordKey {
                name: rec.name.clone(),
                record_type: rec.record_type.clone(),
            };
            let drop_old = previous_managed.contains(&key);
            if drop_old {
                removed = removed.saturating_add(1);
            }
            !drop_old
        });

        let mut added: u32 = 0;
        let mut updated: u32 = 0;
        for desired in desired_records {
            let exists_same = domain_records.iter().any(|r| {
                r.name == desired.name
                    && r.record_type == desired.record_type
                    && r.content == desired.content
                    && r.ttl == desired.ttl
            });
            if exists_same {
                continue;
            }

            let had_same_key = domain_records.iter().any(|r| {
                r.name == desired.name && r.record_type == desired.record_type
            });

            domain_records.retain(|r| !(r.name == desired.name && r.record_type == desired.record_type));
            domain_records.push(desired);

            if had_same_key {
                updated = updated.saturating_add(1);
            } else {
                added = added.saturating_add(1);
            }
        }

        let total_records = domain_records.len() as u32;
        all_records.insert(domain.clone(), domain_records);
        self.write_all_records(&all_records);
        self.update_zone_record_count(&domain, total_records as usize);

        managed_map.insert(domain.clone(), desired_keys.into_iter().collect());
        self.write_managed_keys(&managed_map);

        Ok(DnsReconcileResult {
            domain,
            zone_created,
            added,
            updated,
            removed,
            total_records,
        })
    }

    pub fn list_zones(&self) -> Vec<LocalZone> {
        self.read_zones()
    }

    pub fn delete_zone(&self, domain: &str) -> Result<(), String> {
        let domain = Self::normalize_domain(domain);
        let mut zones = self.read_zones();
        let target_name = format!("{}.", domain);

        if let Some(pos) = zones.iter().position(|z| z.name == target_name) {
            zones.remove(pos);
            self.write_zones(&zones);

            let mut all_records = self.read_all_records();
            all_records.remove(&domain);
            self.write_all_records(&all_records);

            let mut managed = self.read_managed_keys();
            managed.remove(&domain);
            self.write_managed_keys(&managed);

            Ok(())
        } else {
            Err("Zone bulunamadi.".to_string())
        }
    }

    pub fn get_default_nameservers(&self) -> DefaultNameservers {
        if let Ok(data) = std::fs::read_to_string(&self.config_path) {
            serde_json::from_str(&data).unwrap_or_default()
        } else {
            DefaultNameservers::default()
        }
    }

    pub fn set_default_nameservers(&self, config: DefaultNameservers) -> Result<(), String> {
        if let Ok(data) = serde_json::to_string_pretty(&config) {
            std::fs::write(&self.config_path, data).map_err(|e| e.to_string())?;
            Ok(())
        } else {
            Err("Failed to serialize config".to_string())
        }
    }

    pub fn suggest_default_nameservers(&self, base_domain: &str) -> Result<DefaultNameservers, String> {
        let base = Self::normalize_domain(base_domain);
        if base.is_empty() {
            return Err("base_domain zorunludur.".to_string());
        }
        Ok(DefaultNameservers {
            ns1: format!("ns1.{}", base),
            ns2: format!("ns2.{}", base),
        })
    }

    pub fn reset_default_nameservers(&self) -> Result<DefaultNameservers, String> {
        let defaults = DefaultNameservers::default();
        self.set_default_nameservers(defaults.clone())?;
        Ok(defaults)
    }

    pub fn set_dnssec_enabled(&self, domain: &str, enabled: bool) -> Result<LocalZone, String> {
        self.update_zone_dnssec(domain, enabled)
    }

    pub async fn create_zone(&self, config: &DnsZoneConfig) -> Result<(), String> {
        let domain = Self::normalize_domain(&config.domain);
        let _payload = serde_json::json!({
            "name": format!("{}.", domain),
            "kind": "Native",
            "nameservers": []
        });

        println!("[DNS] Creating or reconciling PowerDNS zone for: {}", domain);
        self.reconcile_zone_defaults(&domain, &config.server_ip)?;

        Ok(())
    }

    pub async fn add_record(&self, domain: &str, record: DnsRecord) -> Result<(), String> {
        let _payload = serde_json::json!({
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

        println!(
            "[DNS] Adding DNS record: {} -> {} ({})",
            record.name, record.content, record.record_type
        );

        let mut all_records = self.read_all_records();
        let mut domain_records = all_records.remove(domain).unwrap_or_default();

        let exists = domain_records.iter().any(|existing| {
            existing.name == record.name
                && existing.record_type == record.record_type
                && existing.content == record.content
                && existing.ttl == record.ttl
        });

        if !exists {
            domain_records.push(record);
        }

        let record_count = domain_records.len();
        all_records.insert(domain.to_string(), domain_records);
        self.write_all_records(&all_records);
        self.update_zone_record_count(domain, record_count);

        Ok(())
    }

    pub fn get_records(&self, domain: &str) -> Vec<DnsRecord> {
        let all_records = self.read_all_records();
        all_records.get(domain).cloned().unwrap_or_default()
    }

    pub fn delete_record(&self, domain: &str, record_type: &str, record_name: &str) -> Result<(), String> {
        let mut all_records = self.read_all_records();

        if let Some(mut records) = all_records.remove(domain) {
            records.retain(|r| !(r.record_type == record_type && r.name == record_name));
            let remaining_count = records.len();
            all_records.insert(domain.to_string(), records);
            self.write_all_records(&all_records);
            self.update_zone_record_count(domain, remaining_count);
            Ok(())
        } else {
            Err("Domain records not found".to_string())
        }
    }
}
