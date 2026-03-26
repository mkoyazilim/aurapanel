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
use crate::services::db::{DbConfig, MariaDbManager, PostgresManager};
use crate::services::mail::{MailManager, MailboxConfig};
use crate::services::perf::{PerfManager, RedisConfig};
use crate::services::security::{SecurityManager, FirewallRule};
use crate::services::security::waf::{MlWaf, HttpRequest as WafHttpRequest};
use crate::services::monitor::MonitorManager;
use crate::services::apps::{AppManager, CmsInstallConfig};
use crate::services::federated::{FederatedManager, WireguardConfig};
use crate::services::ssl::{SslManager, SslConfig};
use crate::services::secure_connect::{SecureConnectManager, SftpUserConfig};
use crate::services::storage::{BackupManager, BackupConfig};
use crate::services::monitor::gitops::{GitOpsManager, GitOpsConfig};
use crate::services::docker::{DockerManager, docker::{CreateContainerConfig, PullImageConfig}};

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
        .route("/db/mariadb/list", get(mariadb_list_handler))
        .route("/db/mariadb/create", post(mariadb_create_handler))
        .route("/db/mariadb/drop", post(mariadb_drop_handler))
        .route("/db/mariadb/users", get(mariadb_users_handler))
        .route("/db/postgres/list", get(postgres_list_handler))
        .route("/db/postgres/create", post(postgres_create_handler))
        .route("/db/postgres/drop", post(postgres_drop_handler))
        .route("/db/postgres/users", get(postgres_users_handler))
        .route("/mail/create", post(create_mailbox_handler))
        .route("/perf/redis", post(create_redis_handler))
        .route("/security/firewall", post(firewall_rule_handler))
        .route("/security/waf", post(waf_inspect_handler))
        .route("/monitor/sre", get(sre_metrics_handler))
        .route("/apps/install", post(install_cms_handler))
        .route("/federated/join", post(cluster_join_handler))
        .route("/ssl/issue", post(issue_ssl_handler))
        .route("/sftp/create", post(create_sftp_handler))
        .route("/backup/create", post(create_backup_handler))
        .route("/gitops/deploy", post(gitops_deploy_handler))
        // Docker Manager
        .route("/docker/containers", get(docker_list_containers))
        .route("/docker/containers/create", post(docker_create_container))
        .route("/docker/containers/start", post(docker_action_handler))
        .route("/docker/containers/stop", post(docker_action_handler))
        .route("/docker/containers/restart", post(docker_action_handler))
        .route("/docker/containers/remove", post(docker_action_handler))
        .route("/docker/images", get(docker_list_images))
        .route("/docker/images/pull", post(docker_pull_image))
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

// ─── MariaDB Handlers ─────────────────────────────────────────

async fn mariadb_list_handler() -> Json<serde_json::Value> {
    match MariaDbManager::list_databases() {
        Ok(dbs) => Json(json!({ "status": "success", "data": dbs })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mariadb_create_handler(Json(payload): Json<DbConfig>) -> Json<serde_json::Value> {
    match MariaDbManager::create_database(&payload) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct DropDbRequest {
    name: String,
}

async fn mariadb_drop_handler(Json(payload): Json<DropDbRequest>) -> Json<serde_json::Value> {
    match MariaDbManager::drop_database(&payload.name) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mariadb_users_handler() -> Json<serde_json::Value> {
    match MariaDbManager::list_users() {
        Ok(users) => Json(json!({ "status": "success", "data": users })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// ─── PostgreSQL Handlers ──────────────────────────────────────

async fn postgres_list_handler() -> Json<serde_json::Value> {
    match PostgresManager::list_databases() {
        Ok(dbs) => Json(json!({ "status": "success", "data": dbs })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn postgres_create_handler(Json(payload): Json<DbConfig>) -> Json<serde_json::Value> {
    match PostgresManager::create_database(&payload) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn postgres_drop_handler(Json(payload): Json<DropDbRequest>) -> Json<serde_json::Value> {
    match PostgresManager::drop_database(&payload.name) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn postgres_users_handler() -> Json<serde_json::Value> {
    match PostgresManager::list_users() {
        Ok(users) => Json(json!({ "status": "success", "data": users })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
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

// Handler for issuing SSL certificate
async fn issue_ssl_handler(
    Json(payload): Json<SslConfig>,
) -> Json<serde_json::Value> {
    match SslManager::issue_certificate(&payload).await {
        Ok(_) => Json(json!({
            "status": "success",
            "message": format!("SSL certificate for {} issued successfully.", payload.domain),
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

// Handler for creating SFTP user
async fn create_sftp_handler(
    Json(payload): Json<SftpUserConfig>,
) -> Json<serde_json::Value> {
    match SecureConnectManager::create_sftp_user(&payload).await {
        Ok(_) => Json(json!({
            "status": "success",
            "message": format!("SFTP user {} created.", payload.username),
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

// Handler for creating backup
async fn create_backup_handler(
    Json(payload): Json<BackupConfig>,
) -> Json<serde_json::Value> {
    match BackupManager::create_backup(&payload).await {
        Ok(snapshot) => Json(json!({
            "status": "success",
            "message": format!("Backup created for {}.", payload.domain),
            "snapshot_id": snapshot,
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

// Handler for WAF inspection
async fn waf_inspect_handler(
    Json(payload): Json<WafHttpRequest>,
) -> Json<serde_json::Value> {
    let verdict = MlWaf::inspect(&payload);
    Json(json!({
        "allowed": verdict.allowed,
        "score": verdict.score,
        "reason": verdict.reason,
    }))
}

// Handler for GitOps deploy
async fn gitops_deploy_handler(
    Json(payload): Json<GitOpsConfig>,
) -> Json<serde_json::Value> {
    match GitOpsManager::deploy(&payload).await {
        Ok(_) => Json(json!({
            "status": "success",
            "message": format!("Deployed {} to {}.", payload.repo_url, payload.deploy_path),
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

// ─── Docker Handlers ────────────────────────────────────────

async fn docker_list_containers() -> Json<serde_json::Value> {
    match DockerManager::list_containers() {
        Ok(containers) => Json(json!({ "status": "success", "data": containers })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn docker_create_container(
    Json(payload): Json<CreateContainerConfig>,
) -> Json<serde_json::Value> {
    match DockerManager::create_container(&payload) {
        Ok(id) => Json(json!({ "status": "success", "container_id": id })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct DockerActionPayload {
    id: String,
    action: String, // start, stop, restart, remove
    force: Option<bool>,
}

async fn docker_action_handler(
    Json(payload): Json<DockerActionPayload>,
) -> Json<serde_json::Value> {
    let result = match payload.action.as_str() {
        "start" => DockerManager::start_container(&payload.id),
        "stop" => DockerManager::stop_container(&payload.id),
        "restart" => DockerManager::restart_container(&payload.id),
        "remove" => DockerManager::remove_container(&payload.id, payload.force.unwrap_or(false)),
        _ => Err("Bilinmeyen eylem".to_string()),
    };

    match result {
        Ok(_) => Json(json!({ "status": "success", "message": format!("{} -> {}", payload.action, payload.id) })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn docker_list_images() -> Json<serde_json::Value> {
    match DockerManager::list_images() {
        Ok(images) => Json(json!({ "status": "success", "data": images })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn docker_pull_image(
    Json(payload): Json<PullImageConfig>,
) -> Json<serde_json::Value> {
    match DockerManager::pull_image(&payload) {
        Ok(_) => Json(json!({ "status": "success", "message": format!("Image {} pulled.", payload.image) })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}
