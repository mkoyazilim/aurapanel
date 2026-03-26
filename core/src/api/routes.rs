use axum::{
    routing::{get, post},
    Router,
    Json,
    extract::State,
};
use serde::{Deserialize, Serialize};
use serde_json::json;

use crate::services::nitro::{NitroEngine, VHostConfig};
use crate::services::dns::{PowerDnsManager, DnsZoneConfig};
use crate::services::db::{DbManager, DbConfig};
use crate::services::mail::{MailManager, MailboxConfig};
use crate::services::perf::{PerfManager, RedisConfig};
use crate::services::security::{SecurityManager, FirewallRule};
use crate::services::monitor::MonitorManager;
use crate::services::apps::{AppManager, CmsInstallConfig};
use crate::services::federated::{FederatedManager, WireguardConfig};

#[derive(Serialize)]
struct StatusResponse {
    status: String,
    uptime: u64,
    version: String,
}

pub fn routes() -> Router {
    Router::new()
        .route("/health", get(health_check))
        .route("/vhost", post(create_vhost_handler))
        .route("/dns/zone", post(create_dns_zone_handler))
        .route("/db/create", post(create_database_handler))
        .route("/mail/create", post(create_mailbox_handler))
        .route("/perf/redis", post(create_redis_handler))
        .route("/security/firewall", post(firewall_rule_handler))
        .route("/monitor/sre", get(sre_metrics_handler))
        .route("/apps/install", post(install_cms_handler))
        .route("/federated/join", post(cluster_join_handler))
}

async fn health_check() -> Json<StatusResponse> {
    Json(StatusResponse {
        status: "online".to_string(),
        uptime: 0,
        version: "1.0.0-alpha".to_string(),
    })
}

// Handler for Federated Join Node
async fn cluster_join_handler(
    Json(payload): Json<WireguardConfig>,
) -> Json<serde_json::Value> {
    match FederatedManager::add_cluster_node(&payload).await {
        Ok(_) => Json(json!({
            "status": "success",
            "message": format!("Node {} successfully added to cluster.", payload.node_name),
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

// Handler for SRE Metrics
async fn sre_metrics_handler() -> Json<serde_json::Value> {
    match MonitorManager::predict_bottleneck().await {
        Ok(prediction) => Json(json!({
            "status": "success",
            "prediction": prediction,
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

// Handler for CMS Installation
async fn install_cms_handler(
    Json(payload): Json<CmsInstallConfig>,
) -> Json<serde_json::Value> {
    match AppManager::install_cms(&payload).await {
        Ok(_) => Json(json!({
            "status": "success",
            "message": format!("{} successfully installed on {}.", payload.app_type, payload.domain),
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

// Handler for creating isolated Redis
async fn create_redis_handler(
    Json(payload): Json<RedisConfig>,
) -> Json<serde_json::Value> {
    match PerfManager::create_redis_instance(&payload).await {
        Ok(_) => Json(json!({
            "status": "success",
            "message": format!("Isolated Redis for {} activated.", payload.domain),
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

// Handler for IP blocking
async fn firewall_rule_handler(
    Json(payload): Json<FirewallRule>,
) -> Json<serde_json::Value> {
    match SecurityManager::apply_firewall_rule(&payload).await {
        Ok(_) => Json(json!({
            "status": "success",
            "message": format!("Firewall rule for {} applied.", payload.ip_address),
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

// Handler for creating a new mailbox
async fn create_mailbox_handler(
    Json(payload): Json<MailboxConfig>,
) -> Json<serde_json::Value> {
    match MailManager::create_mailbox(&payload).await {
        Ok(_) => Json(json!({
            "status": "success",
            "message": format!("Mailbox {}@{} created.", payload.username, payload.domain),
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

// Handler for creating a new database
async fn create_database_handler(
    Json(payload): Json<DbConfig>,
) -> Json<serde_json::Value> {
    match DbManager::create_database(&payload).await {
        Ok(_) => Json(json!({
            "status": "success",
            "message": format!("Database {} created and user {} bound.", payload.db_name, payload.db_user),
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

// Handler for creating a new DNS Zone
async fn create_dns_zone_handler(
    Json(payload): Json<DnsZoneConfig>,
) -> Json<serde_json::Value> {
    let pdns = PowerDnsManager::new();
    match pdns.create_zone(&payload).await {
        Ok(_) => Json(json!({
            "status": "success",
            "message": format!("DNS Zone for {} created with default A/CNAME records.", payload.domain),
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

// Handler for creating a new website / vhost
async fn create_vhost_handler(
    Json(payload): Json<VHostConfig>,
) -> Json<serde_json::Value> {
    match NitroEngine::create_vhost(&payload) {
        Ok(_) => Json(json!({
            "status": "success",
            "message": format!("VHost for {} created successfully.", payload.domain),
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}
