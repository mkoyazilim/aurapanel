use axum::{
    extract::{Multipart, Request},
    http::{header, HeaderMap, Method, StatusCode},
    middleware::{from_fn, Next},
    response::{IntoResponse, Response},
    routing::{get, post},
    Extension, Json, Router,
};
use serde::{Deserialize, Serialize};
use serde_json::json;

use crate::api::terminal::terminal_ws_handler;
use crate::auth::jwt;
use crate::services::analytics::{AnalyticsManager, TrafficQuery};
use crate::services::apps::{AppManager, CmsInstallConfig, NodeAppRequest, PythonAppRequest};
use crate::services::audit::AuditLogger;
use crate::services::cloudflare::cloudflare::*;
use crate::services::cloudflare::CloudFlareManager;
use crate::services::db::backup::{DbBackupManager, DbBackupRequest, DbRestoreRequest};
use crate::services::db::ops::{
    allow_remote_access_mariadb, allow_remote_access_postgres, change_password_mariadb,
    change_password_postgres, check_connection_readiness, list_remote_access_mariadb,
    list_remote_access_postgres, DbPasswordChangeRequest, RemoteAccessGrantRequest,
};
use crate::services::db::{auradb::DbExplorerManager, DbConfig, MariaDbManager, PostgresManager};
use crate::services::dns::{DnsZoneConfig, PowerDnsManager};
use crate::services::docker::{
    apps::{CreateDockerAppRequest, DockerAppsManager},
    docker::{CreateContainerConfig, PullImageConfig},
    DockerManager,
};
use crate::services::federated::{FederatedManager, WireguardConfig};
use crate::services::filemanager::FileManager;
use crate::services::mail::{
    MailCatchAllConfig, MailForwardConfig, MailForwardDeleteRequest, MailManager,
    MailRoutingConfig, MailRoutingDeleteRequest, MailWebmailSsoRequest, MailboxConfig,
    MailboxPasswordResetRequest,
};
use crate::services::malware::{
    MalwareQuarantineRequest, MalwareRestoreRequest, MalwareScanStartRequest, MalwareScanner,
};
use crate::services::migration::{
    MigrationAnalyzeRequest, MigrationImportRequest, MigrationManager,
};
use crate::services::monitor::gitops::{GitOpsConfig, GitOpsManager};
use crate::services::monitor::MonitorManager;
use crate::services::nitro::{NitroEngine, VHostConfig, VHostUpdateConfig};
use crate::services::ols_tuning::{OlsTuningConfig, OlsTuningManager};
use crate::services::packages::{CreatePackageRequest, PackageManager, UpdatePackageRequest};
use crate::services::perf::{PerfManager, RedisConfig};
use crate::services::php::PhpManager;
use crate::services::secure_connect::{
    FtpPasswordResetRequest, FtpUserConfig, SecureConnectManager, SftpPasswordResetRequest,
    SftpUserConfig,
};
use crate::services::security::waf::{HttpRequest as WafHttpRequest, MlWaf};
use crate::services::security::{FirewallRule, HardeningRequest, SecurityManager};
use crate::services::ssl::{SslConfig, SslManager};
use crate::services::status::StatusManager;
use crate::services::storage::{
    BackupConfig, BackupDestination, BackupManager, BackupSchedule, MinioBucketRequest,
    MinioCredentialsRequest, StorageManager,
};
use crate::services::users::reseller::{
    AclAssignment, AclPolicy, ResellerManager, ResellerQuota, WhiteLabelConfig,
};
use crate::services::users::{ChangePasswordRequest, CreateUserRequest, PanelUser, UserManager};
use crate::services::websites::{
    ConvertSubdomainRequest, CreateSubdomainRequest, SubdomainPhpUpdateRequest,
    WebsiteAliasRequest, WebsiteCustomSslRequest, WebsiteDbLinkRequest, WebsiteOpenBasedirRequest,
    WebsiteRewriteRequest, WebsiteVhostConfigRequest, WebsitesManager,
};
use crate::services::wordpress::{
    WordPressBackupRequest, WordPressBackupRestoreRequest, WordPressExtensionActionRequest,
    WordPressManager, WordPressStagingRequest,
};

#[derive(Serialize)]
struct StatusResponse {
    status: String,
    uptime: u64,
    version: String,
}

#[derive(Clone, Debug)]
struct AuthContext {
    username: String,
    role: String,
}

const ROLE_ADMIN: &str = "admin";
const ROLE_RESELLER: &str = "reseller";
const ROLE_USER: &str = "user";

#[derive(Deserialize)]
struct LoginRequest {
    email: String,
    password: String,
    #[serde(default)]
    totp_token: Option<String>,
}

fn public_user_view(user: &PanelUser) -> serde_json::Value {
    json!({
        "id": user.id,
        "username": user.username,
        "email": user.email,
        "role": user.role,
        "package": user.package,
        "sites": user.sites,
        "active": user.active,
        "two_fa_enabled": user.totp_enabled,
    })
}

fn bearer_token(headers: &HeaderMap) -> Result<&str, String> {
    let value = headers
        .get(header::AUTHORIZATION)
        .ok_or_else(|| "Authorization header gerekli.".to_string())?
        .to_str()
        .map_err(|_| "Authorization header gecersiz.".to_string())?;

    value
        .strip_prefix("Bearer ")
        .map(str::trim)
        .filter(|token| !token.is_empty())
        .ok_or_else(|| "Bearer token gerekli.".to_string())
}

fn normalize_role(role: &str) -> String {
    match role.trim().to_ascii_lowercase().as_str() {
        ROLE_ADMIN => ROLE_ADMIN.to_string(),
        ROLE_RESELLER => ROLE_RESELLER.to_string(),
        _ => ROLE_USER.to_string(),
    }
}

fn path_matches_prefix(path: &str, prefix: &str) -> bool {
    path == prefix || path.starts_with(&format!("{}/", prefix.trim_end_matches('/')))
}

fn reseller_can_access(path: &str) -> bool {
    // Operational surfaces for reseller role.
    const RESELLER_PREFIXES: [&str; 27] = [
        "/auth/me",
        "/vhost",
        "/websites",
        "/dns",
        "/db",
        "/mail",
        "/ftp",
        "/sftp",
        "/backup",
        "/apps",
        "/wordpress",
        "/files",
        "/php",
        "/ssl",
        "/monitor/cron",
        "/monitor/logs/site",
        "/security/status",
        "/security/firewall",
        "/security/ssh-keys",
        "/security/2fa",
        "/security/immutable/status",
        "/security/ebpf/events",
        "/security/malware",
        "/status/metrics",
        "/status/services",
        "/status/processes",
        "/analytics/website-traffic",
    ];

    RESELLER_PREFIXES
        .iter()
        .any(|prefix| path_matches_prefix(path, prefix))
}

fn user_can_access(method: &Method, path: &str) -> bool {
    if path == "/auth/me" || path == "/status/metrics" || path == "/status/services" {
        return true;
    }

    // Personal-security actions are allowed for end users.
    if path_matches_prefix(path, "/security/2fa")
        || path_matches_prefix(path, "/security/ssh-keys")
        || path_matches_prefix(path, "/security/status")
        || path_matches_prefix(path, "/security/immutable/status")
        || path_matches_prefix(path, "/security/ebpf/events")
    {
        return true;
    }

    // File manager needs POST/DELETE based APIs.
    if path_matches_prefix(path, "/files") {
        return true;
    }

    // Tenant read-only surfaces.
    if method == Method::GET {
        return path_matches_prefix(path, "/vhost/list")
            || path_matches_prefix(path, "/websites/aliases")
            || path_matches_prefix(path, "/websites/advanced-config")
            || path_matches_prefix(path, "/websites/custom-ssl")
            || path_matches_prefix(path, "/monitor/logs/site")
            || path_matches_prefix(path, "/analytics/website-traffic");
    }

    false
}

fn is_authorized_for_path(role: &str, method: &Method, path: &str) -> bool {
    match normalize_role(role).as_str() {
        ROLE_ADMIN => true,
        ROLE_RESELLER => reseller_can_access(path),
        ROLE_USER => user_can_access(method, path),
        _ => false,
    }
}

fn env_admin_fallback_enabled() -> bool {
    std::env::var("AURAPANEL_ENV_ADMIN_FALLBACK")
        .ok()
        .map(|v| {
            matches!(
                v.trim().to_ascii_lowercase().as_str(),
                "1" | "true" | "yes" | "on"
            )
        })
        .unwrap_or(true)
}

fn env_admin_credentials() -> Option<(String, String)> {
    let email = std::env::var("AURAPANEL_ADMIN_EMAIL")
        .ok()
        .map(|v| v.trim().to_string())
        .filter(|v| !v.is_empty())
        .unwrap_or_else(|| "admin@server.com".to_string());

    let direct_password = std::env::var("AURAPANEL_ADMIN_PASSWORD")
        .ok()
        .map(|v| v.trim().to_string())
        .filter(|v| !v.is_empty());
    let file_password = std::fs::read_to_string("/opt/aurapanel/logs/initial_password.txt")
        .ok()
        .map(|v| v.trim().to_string())
        .filter(|v| !v.is_empty());

    direct_password
        .or(file_password)
        .map(|password| (email, password))
}

fn env_admin_user_if_valid(identity: &str, password: &str) -> Option<PanelUser> {
    if !env_admin_fallback_enabled() {
        return None;
    }

    let (email, configured_password) = env_admin_credentials()?;
    let normalized_identity = identity.trim().to_ascii_lowercase();
    if normalized_identity != email.to_ascii_lowercase() && normalized_identity != "admin" {
        return None;
    }
    if password != configured_password {
        return None;
    }

    Some(PanelUser {
        id: 1,
        username: "admin".to_string(),
        email,
        role: ROLE_ADMIN.to_string(),
        package: "default".to_string(),
        sites: 0,
        active: true,
        password_hash: String::new(),
        totp_enabled: false,
        totp_secret: None,
    })
}

async fn auth_middleware(mut request: Request, next: Next) -> Response {
    // Check header first, fallback to query for websockets
    let token = match bearer_token(request.headers()) {
        Ok(token) => token.to_string(),
        Err(_) => {
            // Check query string
            let query = request.uri().query().unwrap_or("");
            let token_opt = query
                .split('&')
                .find(|p| p.starts_with("token="))
                .map(|p| p.trim_start_matches("token="));
            match token_opt {
                Some(t) if !t.is_empty() => t.to_string(),
                _ => {
                    return (
                        StatusCode::UNAUTHORIZED,
                        Json(json!({
                            "status": "error",
                            "message": "Authorization header or token query parameter gerekli.",
                        })),
                    )
                        .into_response()
                }
            }
        }
    };

    let claims = match jwt::verify_token(&token) {
        Ok(claims) => claims,
        Err(message) => {
            return (
                StatusCode::UNAUTHORIZED,
                Json(json!({
                    "status": "error",
                    "message": message,
                })),
            )
                .into_response()
        }
    };

    let normalized_role = normalize_role(&claims.role);
    let request_path = request.uri().path().to_string();
    if !is_authorized_for_path(&normalized_role, request.method(), &request_path) {
        return (
            StatusCode::FORBIDDEN,
            Json(json!({
                "status": "error",
                "message": "Bu role bu endpoint icin yetki verilmedi.",
            })),
        )
            .into_response();
    }

    request.extensions_mut().insert(AuthContext {
        username: claims.sub,
        role: normalized_role,
    });

    next.run(request).await
}

async fn auth_login_handler(Json(payload): Json<LoginRequest>) -> impl IntoResponse {
    let identity = payload.email.trim();
    if identity.is_empty() || payload.password.trim().is_empty() {
        return (
            StatusCode::BAD_REQUEST,
            Json(json!({
                "status": "error",
                "message": "email/kullanici adi ve sifre zorunludur.",
            })),
        );
    }

    let user = match UserManager::find_by_identity(identity) {
        Ok(Some(user)) => user,
        Ok(None) => match env_admin_user_if_valid(identity, payload.password.trim()) {
            Some(fallback_user) => fallback_user,
            None => {
                return (
                    StatusCode::UNAUTHORIZED,
                    Json(json!({
                        "status": "error",
                        "message": "Giris basarisiz. Bilgilerinizi kontrol edin.",
                    })),
                )
            }
        },
        Err(message) => {
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(json!({
                    "status": "error",
                    "message": message,
                })),
            )
        }
    };

    if !user.active {
        return (
            StatusCode::FORBIDDEN,
            Json(json!({
                "status": "error",
                "message": "Bu hesap pasif durumda.",
            })),
        );
    }

    let normalized_role = normalize_role(&user.role);
    if normalized_role != ROLE_ADMIN
        && normalized_role != ROLE_RESELLER
        && normalized_role != ROLE_USER
    {
        return (
            StatusCode::FORBIDDEN,
            Json(json!({
                "status": "error",
                "message": "Bu hesap role olarak desteklenmiyor.",
            })),
        );
    }

    let is_env_fallback_admin =
        user.username == "admin" && user.password_hash.is_empty() && user.email.contains('@');
    let password_ok = if is_env_fallback_admin {
        true
    } else {
        match UserManager::verify_password(&user.username, &payload.password) {
            Ok(valid) => valid,
            Err(message) => {
                return (
                    StatusCode::UNAUTHORIZED,
                    Json(json!({
                        "status": "error",
                        "message": message,
                    })),
                )
            }
        }
    };

    if !password_ok {
        return (
            StatusCode::UNAUTHORIZED,
            Json(json!({
                "status": "error",
                "message": "Giris basarisiz. Bilgilerinizi kontrol edin.",
            })),
        );
    }

    if user.totp_enabled {
        let token = payload
            .totp_token
            .as_deref()
            .unwrap_or("")
            .trim()
            .to_string();
        if token.is_empty() {
            return (
                StatusCode::UNAUTHORIZED,
                Json(json!({
                    "status": "error",
                    "message": "2FA kodu gerekli.",
                    "requires_2fa": true,
                })),
            );
        }

        let secret = user.totp_secret.clone().unwrap_or_default();
        let valid = match SecurityManager::verify_totp(&secret, &token) {
            Ok(valid) => valid,
            Err(message) => {
                return (
                    StatusCode::UNAUTHORIZED,
                    Json(json!({
                        "status": "error",
                        "message": message,
                        "requires_2fa": true,
                    })),
                )
            }
        };

        if !valid {
            return (
                StatusCode::UNAUTHORIZED,
                Json(json!({
                    "status": "error",
                    "message": "2FA kodu gecersiz.",
                    "requires_2fa": true,
                })),
            );
        }
    }

    let token = match jwt::create_token(&user.username, &normalized_role) {
        Ok(token) => token,
        Err(message) => {
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(json!({
                    "status": "error",
                    "message": message,
                })),
            )
        }
    };

    AuditLogger::log(&user.username, "auth.login", identity, "panel");
    (
        StatusCode::OK,
        Json(json!({
            "status": "success",
            "token": token,
            "user": public_user_view(&user),
        })),
    )
}

pub fn routes() -> Router {
    let protected = Router::new()
        // VHost / Sites
        .route("/vhost", post(create_vhost_handler))
        .route("/vhost/list", get(vhost_list_handler))
        .route("/vhost/delete", post(vhost_delete_handler))
        .route("/vhost/suspend", post(vhost_suspend_handler))
        .route("/vhost/unsuspend", post(vhost_unsuspend_handler))
        .route("/vhost/update", post(vhost_update_handler))
        .route(
            "/websites/subdomains",
            get(websites_subdomains_list_handler),
        )
        .route(
            "/websites/subdomains",
            post(websites_subdomains_create_handler),
        )
        .route(
            "/websites/subdomains",
            axum::routing::delete(websites_subdomains_delete_handler),
        )
        .route(
            "/websites/subdomains/php",
            post(websites_subdomains_php_update_handler),
        )
        .route(
            "/websites/subdomains/convert",
            post(websites_subdomains_convert_handler),
        )
        .route("/websites/db-links", get(websites_db_links_list_handler))
        .route("/websites/db-links", post(websites_db_links_create_handler))
        .route(
            "/websites/db-links",
            axum::routing::delete(websites_db_links_delete_handler),
        )
        .route(
            "/websites/db-links/verify",
            post(websites_db_links_verify_handler),
        )
        .route("/websites/aliases", get(websites_aliases_list_handler))
        .route("/websites/aliases", post(websites_aliases_create_handler))
        .route(
            "/websites/aliases",
            axum::routing::delete(websites_aliases_delete_handler),
        )
        .route(
            "/websites/advanced-config",
            get(websites_advanced_config_get_handler),
        )
        .route(
            "/websites/open-basedir",
            post(websites_open_basedir_set_handler),
        )
        .route("/websites/rewrite", post(websites_rewrite_save_handler))
        .route(
            "/websites/vhost-config",
            post(websites_vhost_config_save_handler),
        )
        .route("/websites/custom-ssl", get(websites_custom_ssl_get_handler))
        .route(
            "/websites/custom-ssl",
            post(websites_custom_ssl_save_handler),
        )
        // DNS
        .route("/dns/zone", post(create_dns_zone_handler))
        .route("/dns/zones", get(dns_zones_list_handler))
        .route(
            "/dns/zones/:domain",
            axum::routing::delete(delete_dns_zone_handler),
        )
        .route("/dns/zones/:domain/records", get(get_dns_records_handler))
        .route("/dns/zones/:domain/records", post(add_dns_record_handler))
        .route(
            "/dns/zones/:domain/records",
            axum::routing::delete(delete_dns_record_handler),
        )
        .route("/dns/zones/:domain/dnssec", post(set_dnssec_handler))
        .route("/dns/reconcile", post(dns_reconcile_handler))
        .route("/dns/default-nameservers", get(get_default_ns_handler))
        .route("/dns/default-nameservers", post(set_default_ns_handler))
        .route(
            "/dns/default-nameservers/wizard",
            post(default_ns_wizard_handler),
        )
        .route(
            "/dns/default-nameservers/reset",
            post(default_ns_reset_handler),
        )
        // Databases
        .route("/db/mariadb/list", get(mariadb_list_handler))
        .route("/db/mariadb/create", post(mariadb_create_handler))
        .route("/db/mariadb/drop", post(mariadb_drop_handler))
        .route("/db/mariadb/users", get(mariadb_users_handler))
        .route("/db/mariadb/password", post(mariadb_password_handler))
        .route(
            "/db/mariadb/remote-access",
            get(mariadb_remote_access_list_handler),
        )
        .route(
            "/db/mariadb/remote-access",
            post(mariadb_remote_access_allow_handler),
        )
        .route("/db/postgres/list", get(postgres_list_handler))
        .route("/db/postgres/create", post(postgres_create_handler))
        .route("/db/postgres/drop", post(postgres_drop_handler))
        .route("/db/postgres/users", get(postgres_users_handler))
        .route("/db/postgres/password", post(postgres_password_handler))
        .route(
            "/db/postgres/remote-access",
            get(postgres_remote_access_list_handler),
        )
        .route(
            "/db/postgres/remote-access",
            post(postgres_remote_access_allow_handler),
        )
        .route(
            "/db/explorer/bridge",
            post(db_explorer_bridge_create_handler),
        )
        .route(
            "/db/explorer/bridge/resolve",
            get(db_explorer_bridge_resolve_handler),
        )
        .route("/db/explorer/query", post(db_explorer_query_handler))
        .route("/db/explorer/tables", post(db_explorer_tables_handler))
        .route("/mail/create", post(create_mailbox_handler))
        .route("/mail/list", get(mail_list_handler))
        .route("/mail/delete", post(mail_delete_handler))
        .route("/mail/password", post(mail_password_reset_handler))
        .route("/mail/forwards", get(mail_forwards_list_handler))
        .route("/mail/forwards", post(mail_forwards_create_handler))
        .route(
            "/mail/forwards",
            axum::routing::delete(mail_forwards_delete_handler),
        )
        .route("/mail/catch-all", get(mail_catch_all_get_handler))
        .route("/mail/catch-all", post(mail_catch_all_set_handler))
        .route("/mail/routing", get(mail_routing_list_handler))
        .route("/mail/routing", post(mail_routing_create_handler))
        .route(
            "/mail/routing",
            axum::routing::delete(mail_routing_delete_handler),
        )
        .route("/mail/dkim", get(mail_dkim_get_handler))
        .route("/mail/dkim/rotate", post(mail_dkim_rotate_handler))
        .route("/mail/webmail/sso", post(mail_webmail_sso_handler))
        // Users
        .route("/users/list", get(users_list_handler))
        .route("/users/create", post(users_create_handler))
        .route("/users/delete", post(users_delete_handler))
        .route(
            "/users/change-password",
            post(users_change_password_handler),
        )
        // Packages
        .route("/packages/list", get(packages_list_handler))
        .route("/packages/create", post(packages_create_handler))
        .route("/packages/update", post(packages_update_handler))
        .route("/packages/delete", post(packages_delete_handler))
        .route("/perf/redis", post(create_redis_handler))
        .route("/security/firewall", post(firewall_rule_handler))
        .route("/security/waf", post(waf_inspect_handler))
        .route("/security/status", get(security_status_handler))
        .route("/security/firewall/rules", get(firewall_rules_list_handler))
        .route(
            "/security/firewall/rules",
            axum::routing::delete(firewall_rule_delete_handler),
        )
        .route("/security/ssh-keys", get(ssh_keys_list_handler))
        .route("/security/ssh-keys", post(ssh_keys_add_handler))
        .route(
            "/security/ssh-keys",
            axum::routing::delete(ssh_keys_delete_handler),
        )
        .route("/security/2fa/setup", post(security_2fa_setup_handler))
        .route("/security/2fa/verify", post(security_2fa_verify_handler))
        .route(
            "/security/hardening/apply",
            post(security_hardening_apply_handler),
        )
        .route(
            "/security/immutable/status",
            get(security_immutable_status_handler),
        )
        .route("/security/live-patch", post(security_live_patch_handler))
        .route("/security/ebpf/events", get(security_ebpf_events_handler))
        .route(
            "/security/ebpf/collect",
            post(security_ebpf_collect_handler),
        )
        .route(
            "/security/malware/scan/start",
            post(security_malware_scan_start_handler),
        )
        .route(
            "/security/malware/scan/status",
            get(security_malware_scan_status_handler),
        )
        .route(
            "/security/malware/scan/jobs",
            get(security_malware_scan_jobs_handler),
        )
        .route(
            "/security/malware/quarantine",
            post(security_malware_quarantine_handler),
        )
        .route(
            "/security/malware/quarantine",
            get(security_malware_quarantine_list_handler),
        )
        .route(
            "/security/malware/quarantine/restore",
            post(security_malware_restore_handler),
        )
        .route("/monitor/sre", get(sre_metrics_handler))
        .route("/monitor/sre/log-query", post(sre_log_query_handler))
        .route("/monitor/sre/optimize", get(sre_optimize_handler))
        .route("/monitor/cron/jobs", get(cron_jobs_list_handler))
        .route("/monitor/cron/jobs", post(cron_jobs_create_handler))
        .route(
            "/monitor/cron/jobs",
            axum::routing::delete(cron_jobs_delete_handler),
        )
        .route("/monitor/logs/site", get(site_logs_handler))
        .route("/apps/install", post(install_cms_handler))
        .route("/wordpress/sites", get(wordpress_sites_list_handler))
        .route("/wordpress/scan", post(wordpress_scan_handler))
        .route("/wordpress/plugins", get(wordpress_plugins_list_handler))
        .route(
            "/wordpress/plugins/update",
            post(wordpress_plugins_update_handler),
        )
        .route(
            "/wordpress/plugins",
            axum::routing::delete(wordpress_plugins_delete_handler),
        )
        .route("/wordpress/themes", get(wordpress_themes_list_handler))
        .route(
            "/wordpress/themes/update",
            post(wordpress_themes_update_handler),
        )
        .route(
            "/wordpress/themes",
            axum::routing::delete(wordpress_themes_delete_handler),
        )
        .route("/wordpress/staging", get(wordpress_staging_list_handler))
        .route("/wordpress/staging", post(wordpress_staging_create_handler))
        .route("/wordpress/backups", get(wordpress_backups_list_handler))
        .route("/wordpress/backups", post(wordpress_backups_create_handler))
        .route(
            "/wordpress/backups/restore",
            post(wordpress_backups_restore_handler),
        )
        .route(
            "/wordpress/backups/download",
            get(wordpress_backups_download_handler),
        )
        .route("/apps/runtime/list", get(runtime_apps_list_handler))
        .route(
            "/apps/runtime/node/install-deps",
            post(node_install_deps_handler),
        )
        .route("/apps/runtime/node/start", post(node_start_handler))
        .route("/apps/runtime/node/stop", post(node_stop_handler))
        .route(
            "/apps/runtime/python/venv",
            post(python_create_venv_handler),
        )
        .route(
            "/apps/runtime/python/install",
            post(python_install_requirements_handler),
        )
        .route("/apps/runtime/python/start", post(python_start_handler))
        .route("/federated/join", post(cluster_join_handler))
        .route("/federated/nodes", get(cluster_nodes_list_handler))
        .route("/federated/mode", get(cluster_mode_handler))
        .route("/ssl/issue", post(issue_ssl_handler))
        .route("/ssl/details", post(ssl_details_handler))
        .route("/ssl/hostname/issue", post(issue_hostname_ssl_handler))
        .route("/ssl/mail/issue", post(issue_mail_ssl_handler))
        .route("/ssl/bindings", get(ssl_bindings_handler))
        .route("/ftp/create", post(create_ftp_handler))
        .route("/ftp/list", get(list_ftp_handler))
        .route("/ftp/delete", post(delete_ftp_handler))
        .route("/ftp/password", post(reset_ftp_password_handler))
        .route("/sftp/create", post(create_sftp_handler))
        .route("/sftp/list", get(list_sftp_handler))
        .route("/sftp/delete", post(delete_sftp_handler))
        .route("/sftp/password", post(reset_sftp_password_handler))
        .route("/backup/create", post(create_backup_handler))
        .route("/backup/restore", post(restore_backup_handler))
        .route("/backup/snapshots", post(backup_snapshots_handler))
        .route(
            "/backup/destinations",
            get(backup_destinations_list_handler),
        )
        .route(
            "/backup/destinations",
            post(backup_destinations_upsert_handler),
        )
        .route(
            "/backup/destinations",
            axum::routing::delete(backup_destinations_delete_handler),
        )
        .route("/backup/schedules", get(backup_schedules_list_handler))
        .route("/backup/schedules", post(backup_schedules_upsert_handler))
        .route(
            "/backup/schedules",
            axum::routing::delete(backup_schedules_delete_handler),
        )
        .route("/storage/minio/buckets", get(minio_buckets_list_handler))
        .route("/storage/minio/buckets", post(minio_buckets_create_handler))
        .route(
            "/storage/minio/credentials",
            post(minio_credentials_handler),
        )
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
        .route("/docker/images/remove", post(docker_remove_image))
        // Docker Apps
        .route("/docker/apps/templates", get(docker_apps_list_templates))
        .route("/docker/packages", get(docker_apps_list_packages))
        .route("/docker/apps/installed", get(docker_apps_list_installed))
        .route("/docker/apps/install", post(docker_apps_install))
        .route("/docker/apps/remove", post(docker_apps_remove))
        // CloudFlare API
        .route("/cloudflare/zones", post(cf_list_zones))
        .route("/cloudflare/dns/list", post(cf_list_dns_records))
        .route("/cloudflare/dns/create", post(cf_create_dns_record))
        .route("/cloudflare/dns/delete", post(cf_delete_dns_record))
        .route("/cloudflare/ssl", post(cf_set_ssl_mode))
        .route("/cloudflare/cache/purge", post(cf_purge_cache))
        .route("/cloudflare/security", post(cf_set_security_level))
        .route("/cloudflare/devmode", post(cf_set_dev_mode))
        .route("/cloudflare/analytics", post(cf_get_analytics))
        // File Manager API
        .route("/files/list", post(file_list_handler))
        .route("/files/read", post(file_read_handler))
        .route("/files/write", post(file_write_handler))
        .route("/files/create_dir", post(file_create_dir_handler))
        .route("/files/delete", post(file_delete_handler))
        .route("/files/rename", post(file_rename_handler))
        .route("/files/compress", post(file_compress_handler))
        .route("/files/extract", post(file_extract_handler))
        .route("/files/trash", post(file_trash_handler))
        // Migration / Transfer Wizard
        .route("/migration/upload", post(migration_upload_handler))
        .route("/migration/analyze", post(migration_analyze_handler))
        .route(
            "/migration/import/start",
            post(migration_import_start_handler),
        )
        .route(
            "/migration/import/status",
            get(migration_import_status_handler),
        )
        // Website analytics
        .route("/analytics/website-traffic", get(website_traffic_handler))
        // PHP Management API
        .route("/php/versions", get(php_list_versions))
        .route("/php/install", post(php_install_handler))
        .route("/php/remove", post(php_remove_handler))
        .route("/php/restart", post(php_restart_handler))
        .route("/php/ini/get", post(php_get_ini_handler))
        .route("/php/ini/save", post(php_save_ini_handler))
        // Server Status API
        .route("/status/metrics", get(status_metrics_handler))
        .route("/status/services", get(status_services_handler))
        .route("/status/processes", get(status_processes_handler))
        .route(
            "/status/service/control",
            post(status_service_control_handler),
        )
        .route("/status/panel-port", get(status_panel_port_handler))
        .route("/status/panel-port", post(status_panel_port_update_handler))
        .route("/ols/tuning", get(ols_tuning_get_handler))
        .route("/ols/tuning", post(ols_tuning_save_handler))
        .route("/ols/tuning/apply", post(ols_tuning_apply_handler))
        .route("/reseller/quotas", get(reseller_quotas_list_handler))
        .route("/reseller/quotas", post(reseller_quotas_upsert_handler))
        .route(
            "/reseller/whitelabel",
            get(reseller_whitelabel_list_handler),
        )
        .route(
            "/reseller/whitelabel",
            post(reseller_whitelabel_upsert_handler),
        )
        .route("/acl/policies", get(acl_policies_list_handler))
        .route("/acl/policies", post(acl_policies_upsert_handler))
        .route(
            "/acl/policies",
            axum::routing::delete(acl_policies_delete_handler),
        )
        .route("/acl/assignments", get(acl_assignments_list_handler))
        .route("/acl/assignments", post(acl_assignments_upsert_handler))
        .route(
            "/acl/assignments",
            axum::routing::delete(acl_assignments_delete_handler),
        )
        .route("/acl/effective", get(acl_effective_permissions_handler))
        // Activity / Audit Log
        .route("/activity/log", get(activity_log_handler))
        // Database Backup
        .route("/db/backup/list", get(db_backup_list_handler))
        .route("/db/backup/create", post(db_backup_create_handler))
        .route("/db/backup/restore", post(db_backup_restore_handler))
        .route("/db/backup/delete", post(db_backup_delete_handler))
        .route("/db/backup/download", get(db_backup_download_handler))
        // Web Terminal
        .route("/terminal/ws", get(terminal_ws_handler))
        // SSL Wildcard
        .route("/ssl/wildcard/issue", post(issue_wildcard_ssl_handler))
        .route("/auth/me", get(auth_me_handler))
        .route_layer(from_fn(auth_middleware));

    Router::new()
        .route("/health", get(health_check))
        .route("/auth/login", post(auth_login_handler))
        .merge(protected)
}

async fn health_check() -> Json<StatusResponse> {
    Json(StatusResponse {
        status: "online".to_string(),
        uptime: 0,
        version: "1.0.0-alpha".to_string(),
    })
}

async fn auth_me_handler(Extension(auth): Extension<AuthContext>) -> impl IntoResponse {
    let user = match UserManager::find_by_identity(&auth.username) {
        Ok(Some(u)) => u,
        _ => {
            return (
                StatusCode::UNAUTHORIZED,
                Json(json!({ "status": "error", "message": "Unauthorized" })),
            )
                .into_response()
        }
    };

    (
        StatusCode::OK,
        Json(json!({
            "id": user.id,
            "name": user.username,
            "email": user.email,
            "role": user.role
        })),
    )
        .into_response()
}

// Handler for Federated Join Node
async fn cluster_join_handler(Json(payload): Json<WireguardConfig>) -> Json<serde_json::Value> {
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

async fn cluster_nodes_list_handler() -> Json<serde_json::Value> {
    match FederatedManager::list_cluster_nodes() {
        Ok(nodes) => Json(json!({
            "status": "success",
            "data": nodes,
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

async fn cluster_mode_handler() -> Json<serde_json::Value> {
    let mode = FederatedManager::mode_status();
    Json(json!({
        "status": "success",
        "data": mode,
    }))
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

#[derive(Deserialize)]
struct SreLogQueryPayload {
    query: String,
}

// Handler for AI-SRE natural language log query
async fn sre_log_query_handler(Json(payload): Json<SreLogQueryPayload>) -> Json<serde_json::Value> {
    match MonitorManager::analyze_log_query(&payload.query).await {
        Ok(result) => Json(json!({
            "status": "success",
            "data": result,
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

// Handler for AI-SRE optimization suggestions
async fn sre_optimize_handler() -> Json<serde_json::Value> {
    match MonitorManager::suggest_optimizations().await {
        Ok(actions) => Json(json!({
            "status": "success",
            "actions": actions,
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

#[derive(Deserialize)]
struct CronCreatePayload {
    user: String,
    schedule: String,
    command: String,
}

#[derive(Deserialize)]
struct CronDeleteQuery {
    id: u64,
}

async fn cron_jobs_list_handler() -> Json<serde_json::Value> {
    match MonitorManager::list_cron_jobs() {
        Ok(jobs) => Json(json!({ "status": "success", "data": jobs })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn cron_jobs_create_handler(
    Json(payload): Json<CronCreatePayload>,
) -> Json<serde_json::Value> {
    match MonitorManager::add_cron_job(&payload.user, &payload.schedule, &payload.command) {
        Ok(job) => Json(json!({ "status": "success", "data": job })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn cron_jobs_delete_handler(
    axum::extract::Query(query): axum::extract::Query<CronDeleteQuery>,
) -> Json<serde_json::Value> {
    match MonitorManager::delete_cron_job(query.id) {
        Ok(_) => Json(json!({ "status": "success", "message": "Cron job deleted." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct SiteLogsQuery {
    domain: String,
    lines: Option<u32>,
    kind: Option<String>,
}

async fn site_logs_handler(
    axum::extract::Query(query): axum::extract::Query<SiteLogsQuery>,
) -> Json<serde_json::Value> {
    match MonitorManager::stream_site_logs_kind(
        &query.domain,
        query.kind.as_deref(),
        query.lines.unwrap_or(50),
    ) {
        Ok(logs) => Json(json!({ "status": "success", "data": logs })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// Handler for CMS Installation
async fn install_cms_handler(Json(payload): Json<CmsInstallConfig>) -> Json<serde_json::Value> {
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

#[derive(Deserialize)]
struct WordPressDomainQuery {
    domain: Option<String>,
}

#[derive(Deserialize)]
struct WordPressBackupDownloadQuery {
    id: String,
}

async fn wordpress_sites_list_handler() -> Json<serde_json::Value> {
    match WordPressManager::list_sites() {
        Ok(items) => Json(json!({ "status": "success", "data": items })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn wordpress_scan_handler() -> Json<serde_json::Value> {
    match WordPressManager::scan_sites() {
        Ok(items) => Json(json!({ "status": "success", "data": items })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn wordpress_plugins_list_handler(
    axum::extract::Query(query): axum::extract::Query<WordPressDomainQuery>,
) -> Json<serde_json::Value> {
    let domain = match query.domain {
        Some(domain) if !domain.trim().is_empty() => domain,
        _ => return Json(json!({ "status": "error", "message": "domain zorunludur." })),
    };
    match WordPressManager::list_plugins(&domain) {
        Ok(items) => Json(json!({ "status": "success", "data": items })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn wordpress_plugins_update_handler(
    Extension(auth): Extension<AuthContext>,
    Json(payload): Json<WordPressExtensionActionRequest>,
) -> Json<serde_json::Value> {
    match WordPressManager::update_plugins(&payload) {
        Ok(msg) => {
            AuditLogger::log(
                &auth.username,
                "wordpress.plugins.update",
                &payload.domain,
                "panel",
            );
            Json(json!({ "status": "success", "message": msg }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn wordpress_plugins_delete_handler(
    Extension(auth): Extension<AuthContext>,
    Json(payload): Json<WordPressExtensionActionRequest>,
) -> Json<serde_json::Value> {
    match WordPressManager::delete_plugins(&payload) {
        Ok(msg) => {
            AuditLogger::log(
                &auth.username,
                "wordpress.plugins.delete",
                &payload.domain,
                "panel",
            );
            Json(json!({ "status": "success", "message": msg }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn wordpress_themes_list_handler(
    axum::extract::Query(query): axum::extract::Query<WordPressDomainQuery>,
) -> Json<serde_json::Value> {
    let domain = match query.domain {
        Some(domain) if !domain.trim().is_empty() => domain,
        _ => return Json(json!({ "status": "error", "message": "domain zorunludur." })),
    };
    match WordPressManager::list_themes(&domain) {
        Ok(items) => Json(json!({ "status": "success", "data": items })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn wordpress_themes_update_handler(
    Extension(auth): Extension<AuthContext>,
    Json(payload): Json<WordPressExtensionActionRequest>,
) -> Json<serde_json::Value> {
    match WordPressManager::update_themes(&payload) {
        Ok(msg) => {
            AuditLogger::log(
                &auth.username,
                "wordpress.themes.update",
                &payload.domain,
                "panel",
            );
            Json(json!({ "status": "success", "message": msg }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn wordpress_themes_delete_handler(
    Extension(auth): Extension<AuthContext>,
    Json(payload): Json<WordPressExtensionActionRequest>,
) -> Json<serde_json::Value> {
    match WordPressManager::delete_themes(&payload) {
        Ok(msg) => {
            AuditLogger::log(
                &auth.username,
                "wordpress.themes.delete",
                &payload.domain,
                "panel",
            );
            Json(json!({ "status": "success", "message": msg }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn wordpress_staging_list_handler(
    axum::extract::Query(query): axum::extract::Query<WordPressDomainQuery>,
) -> Json<serde_json::Value> {
    match WordPressManager::list_staging(query.domain.as_deref()) {
        Ok(items) => Json(json!({ "status": "success", "data": items })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn wordpress_staging_create_handler(
    Extension(auth): Extension<AuthContext>,
    Json(payload): Json<WordPressStagingRequest>,
) -> Json<serde_json::Value> {
    match WordPressManager::create_staging(&payload) {
        Ok(entry) => {
            AuditLogger::log(
                &auth.username,
                "wordpress.staging.create",
                &payload.source_domain,
                "panel",
            );
            Json(json!({ "status": "success", "data": entry }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn wordpress_backups_list_handler(
    axum::extract::Query(query): axum::extract::Query<WordPressDomainQuery>,
) -> Json<serde_json::Value> {
    match WordPressManager::list_backups(query.domain.as_deref()) {
        Ok(items) => Json(json!({ "status": "success", "data": items })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn wordpress_backups_create_handler(
    Extension(auth): Extension<AuthContext>,
    Json(payload): Json<WordPressBackupRequest>,
) -> Json<serde_json::Value> {
    match WordPressManager::create_backup(&payload) {
        Ok(entry) => {
            AuditLogger::log(
                &auth.username,
                "wordpress.backup.create",
                &payload.domain,
                "panel",
            );
            Json(json!({ "status": "success", "data": entry }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn wordpress_backups_restore_handler(
    Extension(auth): Extension<AuthContext>,
    Json(payload): Json<WordPressBackupRestoreRequest>,
) -> Json<serde_json::Value> {
    match WordPressManager::restore_backup(&payload) {
        Ok(msg) => {
            AuditLogger::log(
                &auth.username,
                "wordpress.backup.restore",
                &payload.id,
                "panel",
            );
            Json(json!({ "status": "success", "message": msg }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn wordpress_backups_download_handler(
    axum::extract::Query(q): axum::extract::Query<WordPressBackupDownloadQuery>,
) -> axum::response::Response {
    use axum::body::Body;
    use axum::http::{header, HeaderValue, StatusCode};
    use axum::response::IntoResponse;

    let path = match WordPressManager::backup_file_path(&q.id) {
        Ok(p) => p,
        Err(e) => {
            return (
                StatusCode::NOT_FOUND,
                Json(json!({ "status": "error", "message": e })),
            )
                .into_response()
        }
    };

    let data = match std::fs::read(&path) {
        Ok(bytes) => bytes,
        Err(e) => {
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(json!({
                    "status": "error",
                    "message": format!("Backup dosyasi okunamadi: {}", e),
                })),
            )
                .into_response()
        }
    };

    let filename = path
        .file_name()
        .and_then(|v| v.to_str())
        .unwrap_or("wordpress-backup.tar.gz");
    let disposition = format!("attachment; filename=\"{}\"", filename);

    let mut response = Body::from(data).into_response();
    response.headers_mut().insert(
        header::CONTENT_TYPE,
        HeaderValue::from_static("application/octet-stream"),
    );
    if let Ok(v) = HeaderValue::from_str(&disposition) {
        response
            .headers_mut()
            .insert(header::CONTENT_DISPOSITION, v);
    }
    response
}

async fn runtime_apps_list_handler() -> Json<serde_json::Value> {
    match AppManager::list_runtime_apps() {
        Ok(apps) => Json(json!({ "status": "success", "data": apps })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct RuntimeDirPayload {
    dir: String,
}

#[derive(Deserialize)]
struct NodeStopPayload {
    app_name: String,
}

async fn node_install_deps_handler(
    Json(payload): Json<RuntimeDirPayload>,
) -> Json<serde_json::Value> {
    match AppManager::node_install_dependencies(&payload.dir) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn node_start_handler(Json(payload): Json<NodeAppRequest>) -> Json<serde_json::Value> {
    match AppManager::node_start(&payload) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn node_stop_handler(Json(payload): Json<NodeStopPayload>) -> Json<serde_json::Value> {
    match AppManager::node_stop(&payload.app_name) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn python_create_venv_handler(
    Json(payload): Json<RuntimeDirPayload>,
) -> Json<serde_json::Value> {
    match AppManager::python_create_venv(&payload.dir) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn python_install_requirements_handler(
    Json(payload): Json<RuntimeDirPayload>,
) -> Json<serde_json::Value> {
    match AppManager::python_install_requirements(&payload.dir) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn python_start_handler(Json(payload): Json<PythonAppRequest>) -> Json<serde_json::Value> {
    match AppManager::python_start(&payload) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// Handler for creating isolated Redis
async fn create_redis_handler(Json(payload): Json<RedisConfig>) -> Json<serde_json::Value> {
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
async fn firewall_rule_handler(Json(payload): Json<FirewallRule>) -> Json<serde_json::Value> {
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

async fn security_status_handler(
    Extension(auth): Extension<AuthContext>,
) -> Json<serde_json::Value> {
    let mut status = SecurityManager::status();
    status.totp_2fa = UserManager::get_user(&auth.username)
        .ok()
        .flatten()
        .map(|u| u.totp_enabled)
        .unwrap_or(false);

    Json(json!({
        "status": "success",
        "data": status,
    }))
}

async fn firewall_rules_list_handler() -> Json<serde_json::Value> {
    match SecurityManager::list_firewall_rules() {
        Ok(rules) => Json(json!({ "status": "success", "data": rules })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct FirewallDeleteQuery {
    ip_address: String,
}

async fn firewall_rule_delete_handler(
    axum::extract::Query(query): axum::extract::Query<FirewallDeleteQuery>,
) -> Json<serde_json::Value> {
    match SecurityManager::delete_firewall_rule(&query.ip_address) {
        Ok(_) => Json(json!({ "status": "success", "message": "Firewall rule deleted." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct SshKeyCreatePayload {
    user: String,
    title: String,
    public_key: String,
}

#[derive(Deserialize)]
struct SshKeyDeleteQuery {
    user: String,
    key_id: String,
}

#[derive(Deserialize)]
struct SshKeyListQuery {
    user: Option<String>,
}

async fn ssh_keys_add_handler(Json(payload): Json<SshKeyCreatePayload>) -> Json<serde_json::Value> {
    match SecurityManager::add_ssh_key(&payload.user, &payload.title, &payload.public_key) {
        Ok(key) => Json(json!({ "status": "success", "data": key })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn ssh_keys_list_handler(
    axum::extract::Query(query): axum::extract::Query<SshKeyListQuery>,
) -> Json<serde_json::Value> {
    match SecurityManager::list_ssh_keys(query.user.as_deref()) {
        Ok(keys) => Json(json!({ "status": "success", "data": keys })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn ssh_keys_delete_handler(
    axum::extract::Query(query): axum::extract::Query<SshKeyDeleteQuery>,
) -> Json<serde_json::Value> {
    match SecurityManager::delete_ssh_key(&query.user, &query.key_id) {
        Ok(_) => Json(json!({ "status": "success", "message": "SSH key deleted." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct TwoFaSetupPayload {
    account_name: String,
}

#[derive(Deserialize)]
struct TwoFaVerifyPayload {
    token: String,
}

async fn security_2fa_setup_handler(
    Extension(auth): Extension<AuthContext>,
    Json(payload): Json<TwoFaSetupPayload>,
) -> Json<serde_json::Value> {
    let account_name = if payload.account_name.trim().is_empty() {
        auth.username.clone()
    } else {
        payload.account_name.trim().to_string()
    };

    match SecurityManager::setup_totp_for_user(&auth.username, &account_name) {
        Ok((secret, qr_base64)) => Json(json!({
            "status": "success",
            "data": {
                "secret": secret,
                "qr_base64": qr_base64
            }
        })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn security_2fa_verify_handler(
    Extension(auth): Extension<AuthContext>,
    Json(payload): Json<TwoFaVerifyPayload>,
) -> Json<serde_json::Value> {
    match SecurityManager::verify_totp_for_user(&auth.username, &payload.token) {
        Ok(valid) => Json(json!({ "status": "success", "valid": valid })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn security_hardening_apply_handler(
    Json(payload): Json<HardeningRequest>,
) -> Json<serde_json::Value> {
    match SecurityManager::apply_one_click_hardening(&payload) {
        Ok(result) => Json(json!({ "status": "success", "data": result })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn security_immutable_status_handler() -> Json<serde_json::Value> {
    match SecurityManager::immutable_os_status() {
        Ok(status) => Json(json!({ "status": "success", "data": status })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct LivePatchPayload {
    target: String,
}

async fn security_live_patch_handler(
    Json(payload): Json<LivePatchPayload>,
) -> Json<serde_json::Value> {
    match SecurityManager::run_live_patch(&payload.target) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn security_ebpf_events_handler() -> Json<serde_json::Value> {
    match SecurityManager::list_ebpf_events() {
        Ok(events) => Json(json!({ "status": "success", "data": events })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct EbpfCollectPayload {
    limit: Option<usize>,
}

async fn security_ebpf_collect_handler(
    Json(payload): Json<EbpfCollectPayload>,
) -> Json<serde_json::Value> {
    match SecurityManager::collect_ebpf_events(payload.limit.unwrap_or(100)) {
        Ok(events) => Json(json!({ "status": "success", "data": events })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn security_malware_scan_start_handler(
    Json(payload): Json<MalwareScanStartRequest>,
) -> Json<serde_json::Value> {
    match MalwareScanner::start_scan(payload).await {
        Ok(job) => Json(json!({ "status": "success", "data": job })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct MalwareScanStatusQuery {
    id: String,
}

async fn security_malware_scan_status_handler(
    axum::extract::Query(query): axum::extract::Query<MalwareScanStatusQuery>,
) -> Json<serde_json::Value> {
    match MalwareScanner::get_scan_status(&query.id) {
        Ok(job) => Json(json!({ "status": "success", "data": job })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct MalwareScanJobsQuery {
    limit: Option<usize>,
}

async fn security_malware_scan_jobs_handler(
    axum::extract::Query(query): axum::extract::Query<MalwareScanJobsQuery>,
) -> Json<serde_json::Value> {
    match MalwareScanner::list_scan_jobs(query.limit) {
        Ok(jobs) => Json(json!({ "status": "success", "data": jobs })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn security_malware_quarantine_handler(
    Json(payload): Json<MalwareQuarantineRequest>,
) -> Json<serde_json::Value> {
    match MalwareScanner::quarantine_finding(payload) {
        Ok(record) => Json(json!({ "status": "success", "data": record })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn security_malware_quarantine_list_handler() -> Json<serde_json::Value> {
    match MalwareScanner::list_quarantine() {
        Ok(records) => Json(json!({ "status": "success", "data": records })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn security_malware_restore_handler(
    Json(payload): Json<MalwareRestoreRequest>,
) -> Json<serde_json::Value> {
    match MalwareScanner::restore_quarantine(payload) {
        Ok(message) => Json(json!({ "status": "success", "message": message })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

fn normalize_name_token(value: &str) -> String {
    value.trim().trim_matches('.').to_ascii_lowercase()
}

fn resolve_owner_by_domain(domain: &str) -> Option<String> {
    let target = normalize_name_token(domain);
    if target.is_empty() {
        return None;
    }

    NitroEngine::list_vhosts()
        .ok()?
        .into_iter()
        .find_map(|site| {
            let site_domain = site
                .get("domain")
                .and_then(|v| v.as_str())
                .map(normalize_name_token)?;
            if site_domain != target {
                return None;
            }
            site.get("owner")
                .or_else(|| site.get("user"))
                .and_then(|v| v.as_str())
                .map(|s| s.trim().to_ascii_lowercase())
                .filter(|s| !s.is_empty())
        })
}

fn resolve_effective_owner(owner: Option<&str>, domain_hint: Option<&str>) -> Option<String> {
    if let Some(owner) = owner {
        let cleaned = owner.trim();
        if !cleaned.is_empty() {
            return Some(cleaned.to_ascii_lowercase());
        }
    }
    domain_hint.and_then(resolve_owner_by_domain)
}

fn owner_domain_set(owner: &str) -> std::collections::HashSet<String> {
    NitroEngine::list_vhosts()
        .unwrap_or_default()
        .into_iter()
        .filter_map(|site| {
            let site_owner = site
                .get("owner")
                .or_else(|| site.get("user"))
                .and_then(|v| v.as_str())?;
            if !site_owner.trim().eq_ignore_ascii_case(owner.trim()) {
                return None;
            }
            site.get("domain")
                .and_then(|v| v.as_str())
                .map(normalize_name_token)
        })
        .filter(|d| !d.is_empty())
        .collect()
}

fn count_owner_mailboxes(owner: &str) -> u32 {
    let domains = owner_domain_set(owner);
    if domains.is_empty() {
        return 0;
    }

    MailManager::list_mailboxes()
        .into_iter()
        .filter(|mb| domains.contains(&normalize_name_token(&mb.domain)))
        .count() as u32
}

fn normalize_db_engine_quota(engine: &str) -> Option<&'static str> {
    match engine.trim().to_ascii_lowercase().as_str() {
        "mariadb" | "mysql" => Some("mariadb"),
        "postgres" | "postgresql" => Some("postgresql"),
        _ => None,
    }
}

fn count_owner_databases(owner: &str, engine: &str) -> usize {
    let Some(engine) = normalize_db_engine_quota(engine) else {
        return 0;
    };
    let domains = owner_domain_set(owner);
    let owner_prefix = format!("{}_", owner.trim().to_ascii_lowercase());

    let linked_count = WebsitesManager::list_db_links(None)
        .unwrap_or_default()
        .into_iter()
        .filter(|link| {
            normalize_db_engine_quota(&link.engine) == Some(engine)
                && domains.contains(&normalize_name_token(&link.domain))
        })
        .count();

    // Legacy fallback: some databases may be created before db-link metadata.
    let prefixed_count = match engine {
        "mariadb" => MariaDbManager::list_databases()
            .unwrap_or_default()
            .into_iter()
            .filter(|db| db.name.to_ascii_lowercase().starts_with(&owner_prefix))
            .count(),
        "postgresql" => PostgresManager::list_databases()
            .unwrap_or_default()
            .into_iter()
            .filter(|db| db.name.to_ascii_lowercase().starts_with(&owner_prefix))
            .count(),
        _ => 0,
    };

    linked_count.max(prefixed_count)
}

// Handler for creating a new mailbox
async fn create_mailbox_handler(Json(payload): Json<MailboxConfig>) -> Json<serde_json::Value> {
    let effective_owner = resolve_effective_owner(payload.owner.as_deref(), Some(&payload.domain));

    // Quota enforcement: owner bazli email limitini kontrol et
    if let Some(owner) = effective_owner.as_deref() {
        let quota_exceeded = || -> Option<(u32, u32)> {
            let users = crate::services::users::UserManager::list_users().ok()?;
            let user = users.iter().find(|u| u.username == owner)?;
            let pkg = crate::services::packages::PackageManager::get_package_by_name(&user.package)
                .ok()??;
            if pkg.emails == 0 {
                return None;
            } // 0 = limitsiz
            let count = count_owner_mailboxes(owner);
            Some((count, pkg.emails))
        };
        if let Some((count, limit)) = quota_exceeded() {
            if count >= limit {
                return Json(json!({
                    "status": "error",
                    "message": format!("Email kotasi doldu ({}/{}).", count, limit),
                }));
            }
        }
    }

    match MailManager::create_mailbox(&payload).await {
        Ok(_) => {
            AuditLogger::log(
                effective_owner.as_deref().unwrap_or("admin"),
                "mail.create",
                &format!("{}@{}", payload.username, payload.domain),
                "panel",
            );
            Json(json!({
                "status": "success",
                "message": format!("Mailbox {}@{} created.", payload.username, payload.domain),
            }))
        }
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
    let effective_owner =
        resolve_effective_owner(payload.owner.as_deref(), payload.site_domain.as_deref());

    // Quota enforcement (owner bazli)
    if let Some(owner) = effective_owner.as_deref() {
        let quota_exceeded = || -> Option<(usize, u32)> {
            let users = crate::services::users::UserManager::list_users().ok()?;
            let user = users.iter().find(|u| u.username == owner)?;
            let pkg = crate::services::packages::PackageManager::get_package_by_name(&user.package)
                .ok()??;
            if pkg.databases == 0 {
                return None;
            }
            let count = count_owner_databases(owner, "mariadb");
            Some((count, pkg.databases))
        };
        if let Some((count, limit)) = quota_exceeded() {
            if count as u32 >= limit {
                return Json(json!({
                    "status": "error",
                    "message": format!("Veritabani kotasi doldu ({}/{}).", count, limit),
                }));
            }
        }
    }

    match MariaDbManager::create_database(&payload) {
        Ok(result) => {
            AuditLogger::log(
                effective_owner.as_deref().unwrap_or("admin"),
                "db.mariadb.create",
                &payload.db_name,
                "panel",
            );
            Json(json!({
                "status": "success",
                "message": result.message,
                "data": result,
            }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct DropDbRequest {
    name: String,
}

#[derive(Deserialize)]
struct RemoteAccessListQuery {
    db_user: Option<String>,
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

async fn mariadb_password_handler(
    Json(payload): Json<DbPasswordChangeRequest>,
) -> Json<serde_json::Value> {
    match change_password_mariadb(&payload) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mariadb_remote_access_list_handler(
    axum::extract::Query(query): axum::extract::Query<RemoteAccessListQuery>,
) -> Json<serde_json::Value> {
    match list_remote_access_mariadb(query.db_user.as_deref()) {
        Ok(rules) => Json(json!({ "status": "success", "data": rules })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mariadb_remote_access_allow_handler(
    Json(payload): Json<RemoteAccessGrantRequest>,
) -> Json<serde_json::Value> {
    match allow_remote_access_mariadb(&payload) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
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
    let effective_owner =
        resolve_effective_owner(payload.owner.as_deref(), payload.site_domain.as_deref());

    // Quota enforcement (owner bazli)
    if let Some(owner) = effective_owner.as_deref() {
        let quota_exceeded = || -> Option<(usize, u32)> {
            let users = crate::services::users::UserManager::list_users().ok()?;
            let user = users.iter().find(|u| u.username == owner)?;
            let pkg = crate::services::packages::PackageManager::get_package_by_name(&user.package)
                .ok()??;
            if pkg.databases == 0 {
                return None;
            }
            let count = count_owner_databases(owner, "postgresql");
            Some((count, pkg.databases))
        };
        if let Some((count, limit)) = quota_exceeded() {
            if count as u32 >= limit {
                return Json(json!({
                    "status": "error",
                    "message": format!("Veritabani kotasi doldu ({}/{}).", count, limit),
                }));
            }
        }
    }

    match PostgresManager::create_database(&payload) {
        Ok(result) => {
            AuditLogger::log(
                effective_owner.as_deref().unwrap_or("admin"),
                "db.postgres.create",
                &payload.db_name,
                "panel",
            );
            Json(json!({
                "status": "success",
                "message": result.message,
                "data": result,
            }))
        }
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

async fn postgres_password_handler(
    Json(payload): Json<DbPasswordChangeRequest>,
) -> Json<serde_json::Value> {
    match change_password_postgres(&payload) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn postgres_remote_access_list_handler(
    axum::extract::Query(query): axum::extract::Query<RemoteAccessListQuery>,
) -> Json<serde_json::Value> {
    match list_remote_access_postgres(query.db_user.as_deref()) {
        Ok(rules) => Json(json!({ "status": "success", "data": rules })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn postgres_remote_access_allow_handler(
    Json(payload): Json<RemoteAccessGrantRequest>,
) -> Json<serde_json::Value> {
    match allow_remote_access_postgres(&payload) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct DbExplorerQueryPayload {
    db_type: Option<String>,
    connection_string: Option<String>,
    query: String,
    bridge_token: Option<String>,
}

#[derive(Deserialize)]
struct DbExplorerTablesPayload {
    db_type: Option<String>,
    connection_string: Option<String>,
    bridge_token: Option<String>,
}

#[derive(Deserialize)]
struct DbExplorerBridgeCreatePayload {
    domain: Option<String>,
    engine: String,
    db_name: String,
    db_user: Option<String>,
    ttl_seconds: Option<u64>,
}

#[derive(Deserialize)]
struct DbExplorerBridgeResolveQuery {
    token: String,
}

fn normalize_db_engine(engine: &str) -> Option<&'static str> {
    let normalized = engine.trim().to_ascii_lowercase();
    match normalized.as_str() {
        "mysql" | "mariadb" => Some("mariadb"),
        "postgres" | "postgresql" => Some("postgresql"),
        _ => None,
    }
}

async fn db_explorer_query_handler(
    Json(payload): Json<DbExplorerQueryPayload>,
) -> Json<serde_json::Value> {
    let mgr = DbExplorerManager::new();
    if let Some(token) = payload
        .bridge_token
        .as_deref()
        .map(str::trim)
        .filter(|x| !x.is_empty())
    {
        return match mgr.execute_query_with_bridge(token, &payload.query) {
            Ok(result) => Json(json!({ "status": "success", "data": result })),
            Err(e) => Json(json!({ "status": "error", "message": e.to_string() })),
        };
    }

    let db_type = payload.db_type.unwrap_or_default();
    let connection_string = payload.connection_string.unwrap_or_default();
    if db_type.trim().is_empty() || connection_string.trim().is_empty() {
        return Json(json!({
            "status": "error",
            "message": "db_type and connection_string are required when bridge_token is not provided."
        }));
    }

    match mgr.execute_query(&db_type, &connection_string, &payload.query) {
        Ok(result) => Json(json!({ "status": "success", "data": result })),
        Err(e) => Json(json!({ "status": "error", "message": e.to_string() })),
    }
}

async fn db_explorer_tables_handler(
    Json(payload): Json<DbExplorerTablesPayload>,
) -> Json<serde_json::Value> {
    let mgr = DbExplorerManager::new();
    if let Some(token) = payload
        .bridge_token
        .as_deref()
        .map(str::trim)
        .filter(|x| !x.is_empty())
    {
        return match mgr.list_tables_with_bridge(token) {
            Ok(tables) => Json(json!({ "status": "success", "data": tables })),
            Err(e) => Json(json!({ "status": "error", "message": e.to_string() })),
        };
    }

    let db_type = payload.db_type.unwrap_or_default();
    let connection_string = payload.connection_string.unwrap_or_default();
    if db_type.trim().is_empty() || connection_string.trim().is_empty() {
        return Json(json!({
            "status": "error",
            "message": "db_type and connection_string are required when bridge_token is not provided."
        }));
    }

    match mgr.list_tables(&db_type, &connection_string) {
        Ok(tables) => Json(json!({ "status": "success", "data": tables })),
        Err(e) => Json(json!({ "status": "error", "message": e.to_string() })),
    }
}

async fn db_explorer_bridge_create_handler(
    Json(payload): Json<DbExplorerBridgeCreatePayload>,
) -> Json<serde_json::Value> {
    let engine = match normalize_db_engine(&payload.engine) {
        Some(e) => e.to_string(),
        None => {
            return Json(json!({
                "status": "error",
                "message": "Unsupported database engine.",
            }))
        }
    };

    let db_name = payload.db_name.trim().to_string();
    if db_name.is_empty() {
        return Json(json!({
            "status": "error",
            "message": "db_name is required.",
        }));
    }

    let mut domain = payload
        .domain
        .unwrap_or_default()
        .trim()
        .to_ascii_lowercase();
    let mut db_user = payload.db_user.unwrap_or_default().trim().to_string();

    if domain.is_empty() || db_user.is_empty() {
        match WebsitesManager::list_db_links(None) {
            Ok(links) => {
                if let Some(link) = links.into_iter().find(|x| {
                    x.engine == engine
                        && x.db_name == db_name
                        && (domain.is_empty() || x.domain == domain)
                }) {
                    if domain.is_empty() {
                        domain = link.domain;
                    }
                    if db_user.is_empty() {
                        db_user = link.db_user;
                    }
                }
            }
            Err(e) => {
                return Json(json!({
                    "status": "error",
                    "message": e,
                }));
            }
        }
    }

    if domain.is_empty() || db_user.is_empty() {
        return Json(json!({
            "status": "error",
            "message": "Website-DB link bulunamadi. Once website-db baglantisi olusturun.",
        }));
    }

    match DbExplorerManager::create_bridge_ticket(
        &domain,
        &engine,
        &db_name,
        &db_user,
        payload.ttl_seconds,
    ) {
        Ok(ticket) => Json(json!({
            "status": "success",
            "data": {
                "token": ticket.token,
                "expires_at": ticket.expires_at,
                "profile": ticket.profile,
                "url": format!("/auradb?bridge={}", ticket.token),
            }
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e.to_string(),
        })),
    }
}

async fn db_explorer_bridge_resolve_handler(
    axum::extract::Query(query): axum::extract::Query<DbExplorerBridgeResolveQuery>,
) -> Json<serde_json::Value> {
    match DbExplorerManager::resolve_bridge_token(&query.token) {
        Ok(profile) => Json(json!({ "status": "success", "data": profile })),
        Err(e) => Json(json!({ "status": "error", "message": e.to_string() })),
    }
}

// Handler for creating a new DNS Zone
async fn create_dns_zone_handler(Json(payload): Json<DnsZoneConfig>) -> Json<serde_json::Value> {
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

#[derive(Deserialize)]
struct CreateVhostPayload {
    domain: String,
    user: Option<String>,
    php_version: String,
    package: Option<String>,
    email: Option<String>,
    mail_domain: Option<bool>,
    apache_backend: Option<bool>,
}

fn detect_server_ip() -> Option<String> {
    if let Ok(ip) = std::env::var("AURAPANEL_SERVER_IP") {
        let ip = ip.trim().to_string();
        if !ip.is_empty() {
            return Some(ip);
        }
    }

    let socket = std::net::UdpSocket::bind("0.0.0.0:0").ok()?;
    socket.connect("1.1.1.1:80").ok()?;
    let ip = socket.local_addr().ok()?.ip();

    if ip.is_loopback() || ip.is_unspecified() {
        None
    } else {
        Some(ip.to_string())
    }
}

fn resolve_dns_server_ip(preferred: Option<String>) -> Result<String, String> {
    if let Some(ip) = preferred {
        let ip = ip.trim().to_string();
        if !ip.is_empty() {
            return Ok(ip);
        }
    }

    match detect_server_ip() {
        Some(ip) => Ok(ip),
        None => {
            if crate::runtime::is_production() {
                Err("Gercek sunucu IP'si tespit edilemedi. Production modda AURAPANEL_SERVER_IP tanimlayin.".to_string())
            } else {
                Ok("127.0.0.1".to_string())
            }
        }
    }
}

// Handler for creating a new website / vhost
async fn create_vhost_handler(Json(payload): Json<CreateVhostPayload>) -> Json<serde_json::Value> {
    let domain = payload.domain.trim().to_lowercase();
    let owner = payload
        .user
        .unwrap_or_else(|| "aura".to_string())
        .trim()
        .to_string();
    let php_version = payload.php_version.trim().to_string();

    if domain.is_empty() || owner.is_empty() || php_version.is_empty() {
        return Json(json!({
            "status": "error",
            "message": "domain, user ve php_version zorunludur",
        }));
    }

    // ── Package quota check ────────────────────────────────────────────────
    if let Ok(users) = UserManager::list_users() {
        if let Some(user) = users.iter().find(|u| u.username == owner) {
            if let Ok(Some(pkg)) = PackageManager::get_package_by_name(&user.package) {
                if pkg.domains > 0 {
                    // Count existing sites for this user
                    let existing = NitroEngine::list_vhosts()
                        .unwrap_or_default()
                        .iter()
                        .filter(|s| s.get("owner").and_then(|v| v.as_str()) == Some(&owner))
                        .count() as u32;
                    if existing >= pkg.domains {
                        return Json(json!({
                            "status": "error",
                            "message": format!(
                                "Paket limiti asild: '{}' paketi en fazla {} domain'e izin veriyor.",
                                pkg.name, pkg.domains
                            ),
                        }));
                    }
                }
            }
        }
    }

    let config = VHostConfig {
        domain: domain.clone(),
        user: owner.clone(),
        php_version: php_version.clone(),
    };

    match NitroEngine::create_vhost(&config) {
        Ok(_) => {
            let mut warnings: Vec<String> = Vec::new();
            let server_ip = match resolve_dns_server_ip(None) {
                Ok(ip) => {
                    if ip == "127.0.0.1" {
                        warnings.push("Gercek sunucu IP'si tespit edilemedi, DNS bootstrap 127.0.0.1 ile olusturuldu.".to_string());
                    }
                    ip
                }
                Err(e) => {
                    return Json(json!({
                        "status": "error",
                        "message": e,
                    }))
                }
            };

            // Best-effort DNS otomasyonu: vhost olustuktan sonra zone ve varsayilan kayitlari olustur.
            let pdns = PowerDnsManager::new();
            let dns_cfg = DnsZoneConfig {
                domain: domain.clone(),
                server_ip,
            };
            if let Err(e) = pdns.create_zone(&dns_cfg).await {
                warnings.push(format!("DNS zone otomatik olusturulamadi: {}", e));
            }

            let update_cfg = VHostUpdateConfig {
                domain: domain.clone(),
                owner: Some(owner),
                php_version: Some(php_version),
                package: payload.package,
                email: payload.email,
            };
            if let Err(e) = NitroEngine::update_vhost(&update_cfg) {
                warnings.push(format!("Website metadata guncellenemedi: {}", e));
            }

            if payload.mail_domain.unwrap_or(false) {
                warnings.push("Mail domain bootstrap su an metadata seviyesinde; DKIM/MX otomasyonu sonraki fazda.".to_string());
            }
            if payload.apache_backend.unwrap_or(false) {
                warnings.push("Apache backend toggle secildi; runtime backend switch sonraki fazda tamamlanacak.".to_string());
            }

            AuditLogger::log("admin", "vhost.create", &domain, "panel");
            Json(json!({
                "status": "success",
                "message": format!("VHost for {} created successfully.", domain),
                "warnings": warnings,
            }))
        }
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

// Handler for issuing SSL certificate
async fn issue_ssl_handler(Json(payload): Json<SslConfig>) -> Json<serde_json::Value> {
    match SslManager::issue_certificate(&payload).await {
        Ok(_) => {
            AuditLogger::log("admin", "ssl.issue", &payload.domain, "panel");
            Json(json!({
                "status": "success",
                "message": format!("SSL certificate for {} issued successfully.", payload.domain),
            }))
        }
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

#[derive(Deserialize)]
struct SslDetailsRequest {
    domain: String,
}

async fn ssl_details_handler(Json(payload): Json<SslDetailsRequest>) -> Json<serde_json::Value> {
    match SslManager::certificate_details(&payload.domain) {
        Ok(data) => Json(json!({
            "status": "success",
            "data": data,
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

async fn issue_hostname_ssl_handler(Json(payload): Json<SslConfig>) -> Json<serde_json::Value> {
    match SslManager::issue_hostname_certificate(&payload).await {
        Ok(_) => Json(json!({
            "status": "success",
            "message": format!("Hostname SSL certificate issued for {}.", payload.domain),
            "warning": "Certificate was issued. Panel TLS listener binding should be configured in gateway/reverse-proxy.",
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

async fn issue_mail_ssl_handler(Json(payload): Json<SslConfig>) -> Json<serde_json::Value> {
    match SslManager::issue_mail_server_certificate(&payload).await {
        Ok(_) => Json(json!({
            "status": "success",
            "message": format!("Mail server SSL certificate issued and bound for {}.", payload.domain),
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

async fn ssl_bindings_handler() -> Json<serde_json::Value> {
    match SslManager::get_bindings() {
        Ok(data) => Json(json!({
            "status": "success",
            "data": data,
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

#[derive(Deserialize)]
struct FtpListQuery {
    domain: Option<String>,
}

#[derive(Deserialize)]
struct FtpDeletePayload {
    username: String,
}

async fn create_ftp_handler(Json(payload): Json<FtpUserConfig>) -> Json<serde_json::Value> {
    match SecureConnectManager::create_ftp_user(&payload) {
        Ok(_) => Json(json!({
            "status": "success",
            "message": format!("FTP user {} created.", payload.username),
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

async fn list_ftp_handler(
    axum::extract::Query(query): axum::extract::Query<FtpListQuery>,
) -> Json<serde_json::Value> {
    match SecureConnectManager::list_ftp_users(query.domain.as_deref()) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn delete_ftp_handler(Json(payload): Json<FtpDeletePayload>) -> Json<serde_json::Value> {
    match SecureConnectManager::delete_ftp_user(&payload.username) {
        Ok(_) => Json(json!({ "status": "success", "message": "FTP user deleted." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn reset_ftp_password_handler(
    Json(payload): Json<FtpPasswordResetRequest>,
) -> Json<serde_json::Value> {
    match SecureConnectManager::reset_ftp_password(&payload) {
        Ok(_) => Json(json!({ "status": "success", "message": "FTP password updated." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// Handler for creating SFTP user
async fn create_sftp_handler(Json(payload): Json<SftpUserConfig>) -> Json<serde_json::Value> {
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

#[derive(Deserialize)]
struct SftpDeletePayload {
    username: String,
}

async fn list_sftp_handler() -> Json<serde_json::Value> {
    match SecureConnectManager::list_sftp_users() {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn delete_sftp_handler(Json(payload): Json<SftpDeletePayload>) -> Json<serde_json::Value> {
    match SecureConnectManager::delete_sftp_user(&payload.username) {
        Ok(_) => Json(json!({ "status": "success", "message": "SFTP user deleted." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn reset_sftp_password_handler(
    Json(payload): Json<SftpPasswordResetRequest>,
) -> Json<serde_json::Value> {
    match SecureConnectManager::reset_sftp_password(&payload) {
        Ok(_) => Json(json!({ "status": "success", "message": "SFTP password updated." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// Handler for creating backup
async fn create_backup_handler(Json(payload): Json<BackupConfig>) -> Json<serde_json::Value> {
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

#[derive(Deserialize)]
struct BackupRestorePayload {
    domain: String,
    backup_path: String,
    remote_repo: Option<String>,
    password: Option<String>,
    snapshot_id: String,
}

async fn restore_backup_handler(
    Json(payload): Json<BackupRestorePayload>,
) -> Json<serde_json::Value> {
    let cfg = BackupConfig {
        domain: payload.domain,
        backup_path: payload.backup_path,
        remote_repo: payload.remote_repo.unwrap_or_default(),
        password: payload.password.unwrap_or_default(),
        incremental: None,
    };

    match BackupManager::restore_backup(&cfg, &payload.snapshot_id).await {
        Ok(_) => Json(json!({ "status": "success", "message": "Backup restore tamamlandi." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct BackupDeleteQuery {
    id: String,
}

async fn backup_snapshots_handler(Json(payload): Json<BackupConfig>) -> Json<serde_json::Value> {
    match BackupManager::list_snapshots(&payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn backup_destinations_list_handler() -> Json<serde_json::Value> {
    match BackupManager::list_destinations() {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn backup_destinations_upsert_handler(
    Json(payload): Json<BackupDestination>,
) -> Json<serde_json::Value> {
    match BackupManager::upsert_destination(payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn backup_destinations_delete_handler(
    axum::extract::Query(query): axum::extract::Query<BackupDeleteQuery>,
) -> Json<serde_json::Value> {
    match BackupManager::delete_destination(&query.id) {
        Ok(_) => Json(json!({ "status": "success", "message": "Backup destination deleted." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn backup_schedules_list_handler() -> Json<serde_json::Value> {
    match BackupManager::list_schedules() {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn backup_schedules_upsert_handler(
    Json(payload): Json<BackupSchedule>,
) -> Json<serde_json::Value> {
    match BackupManager::upsert_schedule(payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn backup_schedules_delete_handler(
    axum::extract::Query(query): axum::extract::Query<BackupDeleteQuery>,
) -> Json<serde_json::Value> {
    match BackupManager::delete_schedule(&query.id) {
        Ok(_) => Json(json!({ "status": "success", "message": "Backup schedule deleted." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn minio_buckets_list_handler() -> Json<serde_json::Value> {
    match StorageManager::list_buckets() {
        Ok(buckets) => Json(json!({
            "status": "success",
            "data": buckets,
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

async fn minio_buckets_create_handler(
    Json(payload): Json<MinioBucketRequest>,
) -> Json<serde_json::Value> {
    match StorageManager::create_bucket(&payload.bucket_name) {
        Ok(_) => Json(json!({
            "status": "success",
            "message": "Bucket created.",
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

async fn minio_credentials_handler(
    Json(payload): Json<MinioCredentialsRequest>,
) -> Json<serde_json::Value> {
    match StorageManager::generate_credentials(&payload.user) {
        Ok(creds) => Json(json!({
            "status": "success",
            "data": creds,
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

// Handler for WAF inspection
async fn waf_inspect_handler(Json(payload): Json<WafHttpRequest>) -> Json<serde_json::Value> {
    let verdict = MlWaf::inspect(&payload);
    Json(json!({
        "allowed": verdict.allowed,
        "score": verdict.score,
        "reason": verdict.reason,
    }))
}

// Handler for GitOps deploy
async fn gitops_deploy_handler(Json(payload): Json<GitOpsConfig>) -> Json<serde_json::Value> {
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
        Ok(_) => Json(
            json!({ "status": "success", "message": format!("{} -> {}", payload.action, payload.id) }),
        ),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn docker_list_images() -> Json<serde_json::Value> {
    match DockerManager::list_images() {
        Ok(images) => Json(json!({ "status": "success", "data": images })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn docker_pull_image(Json(payload): Json<PullImageConfig>) -> Json<serde_json::Value> {
    match DockerManager::pull_image(&payload) {
        Ok(_) => Json(
            json!({ "status": "success", "message": format!("Image {} pulled.", payload.image) }),
        ),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct DockerImageRemovePayload {
    id: String,
    force: Option<bool>,
}

async fn docker_remove_image(
    Json(payload): Json<DockerImageRemovePayload>,
) -> Json<serde_json::Value> {
    match DockerManager::remove_image(&payload.id, payload.force.unwrap_or(false)) {
        Ok(_) => Json(
            json!({ "status": "success", "message": format!("Image removed: {}", payload.id) }),
        ),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// ─── Docker Apps Handlers ─────────────────────────────────────

async fn docker_apps_list_templates() -> Json<serde_json::Value> {
    let templates = DockerAppsManager::list_templates();
    Json(json!({ "status": "success", "data": templates }))
}

async fn docker_apps_list_packages() -> Json<serde_json::Value> {
    let packages = DockerAppsManager::list_packages();
    Json(json!({ "status": "success", "data": packages }))
}

async fn docker_apps_list_installed() -> Json<serde_json::Value> {
    // Şimdilik sistemdeki tüm container'ları uygulama olarak döndürüyoruz
    match DockerManager::list_containers() {
        Ok(containers) => Json(json!({ "status": "success", "data": containers })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn docker_apps_install(
    Json(payload): Json<CreateDockerAppRequest>,
) -> Json<serde_json::Value> {
    match DockerAppsManager::create_app(&payload) {
        Ok(id) => Json(
            json!({ "status": "success", "message": format!("Uygulama {} başarıyla kuruldu. Container ID: {}", payload.app_name, id) }),
        ),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct DockerAppRemovePayload {
    app_name: String,
}

async fn docker_apps_remove(
    Json(payload): Json<DockerAppRemovePayload>,
) -> Json<serde_json::Value> {
    // DockerAppManager uygulamaları adlarıyla siliyor
    match DockerManager::remove_container(&payload.app_name, true) {
        Ok(_) => Json(
            json!({ "status": "success", "message": format!("Uygulama {} başarıyla kaldırıldı.", payload.app_name) }),
        ),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// ─── CloudFlare Handlers ─────────────────────────────────────

#[derive(Deserialize)]
struct CfAuthPayload {
    api_key: String,
    email: String,
}

#[derive(Deserialize)]
struct CfZonePayload {
    api_key: String,
    email: String,
    zone_id: String,
}

async fn cf_list_zones(Json(payload): Json<CfAuthPayload>) -> Json<serde_json::Value> {
    match CloudFlareManager::list_zones(&payload.api_key, &payload.email) {
        Ok(zones) => Json(json!({ "status": "success", "data": zones })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn cf_list_dns_records(Json(payload): Json<CfZonePayload>) -> Json<serde_json::Value> {
    match CloudFlareManager::list_dns_records(&payload.api_key, &payload.email, &payload.zone_id) {
        Ok(records) => Json(json!({ "status": "success", "data": records })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct CfCreateDnsPayload {
    api_key: String,
    email: String,
    zone_id: String,
    r#type: String,
    name: String,
    content: String,
    ttl: Option<u32>,
    proxied: Option<bool>,
}

async fn cf_create_dns_record(Json(payload): Json<CfCreateDnsPayload>) -> Json<serde_json::Value> {
    let req = CreateDnsRecordRequest {
        zone_id: payload.zone_id,
        r#type: payload.r#type,
        name: payload.name,
        content: payload.content,
        ttl: payload.ttl,
        proxied: payload.proxied,
    };
    match CloudFlareManager::create_dns_record(&payload.api_key, &payload.email, &req) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct CfDeleteDnsPayload {
    api_key: String,
    email: String,
    zone_id: String,
    record_id: String,
}

async fn cf_delete_dns_record(Json(payload): Json<CfDeleteDnsPayload>) -> Json<serde_json::Value> {
    let req = DeleteDnsRecordRequest {
        zone_id: payload.zone_id,
        record_id: payload.record_id,
    };
    match CloudFlareManager::delete_dns_record(&payload.api_key, &payload.email, &req) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct CfSslPayload {
    api_key: String,
    email: String,
    zone_id: String,
    mode: String,
}

async fn cf_set_ssl_mode(Json(payload): Json<CfSslPayload>) -> Json<serde_json::Value> {
    let req = SetSslModeRequest {
        zone_id: payload.zone_id,
        mode: payload.mode,
    };
    match CloudFlareManager::set_ssl_mode(&payload.api_key, &payload.email, &req) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct CfPurgeCachePayload {
    api_key: String,
    email: String,
    zone_id: String,
    purge_everything: Option<bool>,
    files: Option<Vec<String>>,
}

async fn cf_purge_cache(Json(payload): Json<CfPurgeCachePayload>) -> Json<serde_json::Value> {
    let req = PurgeCacheRequest {
        zone_id: payload.zone_id,
        purge_everything: payload.purge_everything,
        files: payload.files,
    };
    match CloudFlareManager::purge_cache(&payload.api_key, &payload.email, &req) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct CfSecurityPayload {
    api_key: String,
    email: String,
    zone_id: String,
    level: String,
}

async fn cf_set_security_level(Json(payload): Json<CfSecurityPayload>) -> Json<serde_json::Value> {
    let req = SetSecurityLevelRequest {
        zone_id: payload.zone_id,
        level: payload.level,
    };
    match CloudFlareManager::set_security_level(&payload.api_key, &payload.email, &req) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct CfDevModePayload {
    api_key: String,
    email: String,
    zone_id: String,
    enabled: bool,
}

async fn cf_set_dev_mode(Json(payload): Json<CfDevModePayload>) -> Json<serde_json::Value> {
    let req = DevModeRequest {
        zone_id: payload.zone_id,
        enabled: payload.enabled,
    };
    match CloudFlareManager::set_dev_mode(&payload.api_key, &payload.email, &req) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn cf_get_analytics(Json(payload): Json<CfZonePayload>) -> Json<serde_json::Value> {
    match CloudFlareManager::get_zone_analytics(&payload.api_key, &payload.email, &payload.zone_id)
    {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// ─── File Manager Handlers ───────────────────────────────────

#[derive(Deserialize)]
struct FilePathPayload {
    path: String,
}

#[derive(Deserialize)]
struct FileWritePayload {
    path: String,
    content: String,
}

#[derive(Deserialize)]
struct FileRenamePayload {
    old_path: String,
    new_path: String,
}

#[derive(Deserialize)]
struct FileCompressPayload {
    format: String,
    dest_path: String,
    sources: Vec<String>,
}

#[derive(Deserialize)]
struct FileExtractPayload {
    archive_path: String,
    dest_dir: String,
}

async fn file_list_handler(Json(payload): Json<FilePathPayload>) -> Json<serde_json::Value> {
    match FileManager::list_dir(&payload.path) {
        Ok(items) => Json(json!({ "status": "success", "data": items })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn file_read_handler(Json(payload): Json<FilePathPayload>) -> Json<serde_json::Value> {
    match FileManager::read_file(&payload.path) {
        Ok(content) => Json(json!({ "status": "success", "data": content })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn file_write_handler(Json(payload): Json<FileWritePayload>) -> Json<serde_json::Value> {
    match FileManager::write_file(&payload.path, &payload.content) {
        Ok(_) => Json(json!({ "status": "success", "message": "Dosya kaydedildi." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn file_create_dir_handler(Json(payload): Json<FilePathPayload>) -> Json<serde_json::Value> {
    match FileManager::create_dir(&payload.path) {
        Ok(_) => Json(json!({ "status": "success", "message": "Klasör oluşturuldu." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn file_delete_handler(Json(payload): Json<FilePathPayload>) -> Json<serde_json::Value> {
    match FileManager::delete_item(&payload.path) {
        Ok(_) => Json(json!({ "status": "success", "message": "Silindi." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn file_rename_handler(Json(payload): Json<FileRenamePayload>) -> Json<serde_json::Value> {
    match FileManager::rename_item(&payload.old_path, &payload.new_path) {
        Ok(_) => Json(json!({ "status": "success", "message": "İsim değiştirildi." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn file_compress_handler(
    Json(payload): Json<FileCompressPayload>,
) -> Json<serde_json::Value> {
    match FileManager::compress_items(&payload.format, &payload.dest_path, payload.sources) {
        Ok(_) => {
            Json(json!({ "status": "success", "message": "Dosyalar basariyla sikistirildi." }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn file_extract_handler(Json(payload): Json<FileExtractPayload>) -> Json<serde_json::Value> {
    match FileManager::extract_item(&payload.archive_path, &payload.dest_dir) {
        Ok(_) => Json(json!({ "status": "success", "message": "Arsiv basariyla cikarildi." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn file_trash_handler(Json(payload): Json<FilePathPayload>) -> Json<serde_json::Value> {
    match FileManager::trash_item(&payload.path) {
        Ok(_) => Json(json!({ "status": "success", "message": "Oge cop kutusuna tasindi." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// ─── PHP Management Handlers ────────────────────────────────

fn sanitize_upload_filename(name: &str) -> String {
    let cleaned = name
        .chars()
        .filter(|c| c.is_ascii_alphanumeric() || *c == '.' || *c == '-' || *c == '_')
        .collect::<String>();
    if cleaned.is_empty() {
        format!("migration-{}.tar.gz", chrono::Utc::now().timestamp())
    } else {
        cleaned
    }
}

async fn migration_upload_handler(mut multipart: Multipart) -> Json<serde_json::Value> {
    let mut saved_path: Option<String> = None;
    loop {
        let next = match multipart.next_field().await {
            Ok(v) => v,
            Err(e) => {
                return Json(json!({
                    "status": "error",
                    "message": format!("Multipart okunamadi: {}", e),
                }))
            }
        };

        let Some(field) = next else { break };
        let file_name = field
            .file_name()
            .map(sanitize_upload_filename)
            .unwrap_or_else(|| format!("migration-{}.tar.gz", chrono::Utc::now().timestamp()));
        let data = match field.bytes().await {
            Ok(v) => v,
            Err(e) => {
                return Json(json!({
                    "status": "error",
                    "message": format!("Dosya okunamadi: {}", e),
                }))
            }
        };

        let upload_dir = MigrationManager::upload_dir();
        if let Err(e) = std::fs::create_dir_all(&upload_dir) {
            return Json(json!({
                "status": "error",
                "message": format!("Upload dizini olusturulamadi: {}", e),
            }));
        }
        let path = upload_dir.join(file_name);
        match tokio::fs::write(&path, &data).await {
            Ok(_) => {
                saved_path = Some(path.to_string_lossy().to_string());
            }
            Err(e) => {
                return Json(json!({
                    "status": "error",
                    "message": format!("Dosya kaydedilemedi: {}", e),
                }))
            }
        }
    }

    match saved_path {
        Some(path) => Json(json!({ "status": "success", "data": { "archive_path": path } })),
        None => Json(json!({ "status": "error", "message": "Yuklenecek dosya bulunamadi." })),
    }
}

async fn migration_analyze_handler(
    Json(payload): Json<MigrationAnalyzeRequest>,
) -> Json<serde_json::Value> {
    match MigrationManager::analyze_backup(&payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn migration_import_start_handler(
    Json(payload): Json<MigrationImportRequest>,
) -> Json<serde_json::Value> {
    match MigrationManager::start_import(payload).await {
        Ok(job) => Json(json!({ "status": "success", "data": job })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct MigrationStatusQuery {
    id: String,
}

async fn migration_import_status_handler(
    axum::extract::Query(query): axum::extract::Query<MigrationStatusQuery>,
) -> Json<serde_json::Value> {
    match MigrationManager::get_import_job(&query.id) {
        Ok(job) => Json(json!({ "status": "success", "data": job })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn website_traffic_handler(
    axum::extract::Query(query): axum::extract::Query<TrafficQuery>,
) -> Json<serde_json::Value> {
    match AnalyticsManager::website_traffic(&query) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}
#[derive(Deserialize)]
struct PhpVersionPayload {
    version: String,
}

#[derive(Deserialize)]
struct PhpIniSavePayload {
    version: String,
    content: String,
}

async fn php_list_versions() -> Json<serde_json::Value> {
    match PhpManager::list_versions() {
        Ok(versions) => Json(json!({ "status": "success", "data": versions })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn php_install_handler(Json(payload): Json<PhpVersionPayload>) -> Json<serde_json::Value> {
    match PhpManager::install_version(&payload.version) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn php_remove_handler(Json(payload): Json<PhpVersionPayload>) -> Json<serde_json::Value> {
    match PhpManager::remove_version(&payload.version) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn php_restart_handler(Json(payload): Json<PhpVersionPayload>) -> Json<serde_json::Value> {
    match PhpManager::restart_fpm(&payload.version) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn php_get_ini_handler(Json(payload): Json<PhpVersionPayload>) -> Json<serde_json::Value> {
    match PhpManager::get_ini(&payload.version) {
        Ok(content) => Json(json!({ "status": "success", "data": content })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn php_save_ini_handler(Json(payload): Json<PhpIniSavePayload>) -> Json<serde_json::Value> {
    match PhpManager::save_ini(&payload.version, &payload.content) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// ─── Server Status Handlers ─────────────────────────────────

async fn status_metrics_handler() -> Json<serde_json::Value> {
    match StatusManager::get_metrics() {
        Ok(metrics) => Json(json!({ "status": "success", "data": metrics })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn status_services_handler() -> Json<serde_json::Value> {
    match StatusManager::get_services() {
        Ok(services) => Json(json!({ "status": "success", "data": services })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn status_processes_handler() -> Json<serde_json::Value> {
    match StatusManager::get_processes() {
        Ok(procs) => Json(json!({ "status": "success", "data": procs })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct ServiceControlPayload {
    name: String,
    action: String, // start, stop, restart
}

async fn status_service_control_handler(
    Json(payload): Json<ServiceControlPayload>,
) -> Json<serde_json::Value> {
    match StatusManager::control_service(&payload.name, &payload.action) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct PanelPortUpdatePayload {
    port: u16,
    open_firewall: Option<bool>,
}

async fn status_panel_port_handler() -> Json<serde_json::Value> {
    match StatusManager::get_panel_port() {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn status_panel_port_update_handler(
    Json(payload): Json<PanelPortUpdatePayload>,
) -> Json<serde_json::Value> {
    if payload.port == 0 {
        return Json(json!({
            "status": "error",
            "message": "Port araligi 1-65535 olmalidir."
        }));
    }

    match StatusManager::update_panel_port(payload.port, payload.open_firewall.unwrap_or(true)) {
        Ok(data) => Json(json!({
            "status": "success",
            "message": format!("Panel portu {} olarak guncellendi.", payload.port),
            "data": data
        })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// ─── VHost / Sites Handlers ──────────────────────────────────────────────────

async fn ols_tuning_get_handler() -> Json<serde_json::Value> {
    match OlsTuningManager::get_config() {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn ols_tuning_save_handler(Json(payload): Json<OlsTuningConfig>) -> Json<serde_json::Value> {
    match OlsTuningManager::save_config(&payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn ols_tuning_apply_handler(Json(payload): Json<OlsTuningConfig>) -> Json<serde_json::Value> {
    match OlsTuningManager::apply_config(&payload) {
        Ok(message) => Json(json!({ "status": "success", "message": message })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize, Default)]
struct VhostListQuery {
    search: Option<String>,
    php: Option<String>,
    page: Option<usize>,
    per_page: Option<usize>,
}

async fn vhost_list_handler(
    axum::extract::Query(query): axum::extract::Query<VhostListQuery>,
) -> Json<serde_json::Value> {
    use crate::services::nitro::NitroEngine;
    use std::cmp::min;
    match NitroEngine::list_vhosts() {
        Ok(mut sites) => {
            if query.search.is_none()
                && query.php.is_none()
                && query.page.is_none()
                && query.per_page.is_none()
            {
                return Json(json!({ "status": "success", "data": sites }));
            }

            if let Some(search) = query.search.as_ref() {
                let needle = search.to_lowercase();
                sites.retain(|s| {
                    let domain = s
                        .get("domain")
                        .and_then(|x| x.as_str())
                        .unwrap_or_default()
                        .to_lowercase();
                    domain.contains(&needle)
                });
            }

            if let Some(php) = query.php.as_ref() {
                let php = php.to_lowercase();
                sites.retain(|s| {
                    let value = s
                        .get("php")
                        .or_else(|| s.get("php_version"))
                        .and_then(|x| x.as_str())
                        .unwrap_or_default()
                        .to_lowercase();
                    value == php
                });
            }

            let total = sites.len();
            let page = query.page.unwrap_or(1).max(1);
            let per_page = query.per_page.unwrap_or(20).clamp(1, 200);
            let start = (page - 1).saturating_mul(per_page);
            let end = min(start + per_page, total);
            let data = if start >= total {
                Vec::new()
            } else {
                sites[start..end].to_vec()
            };

            Json(json!({
                "status": "success",
                "data": data,
                "pagination": {
                    "page": page,
                    "per_page": per_page,
                    "total": total,
                    "total_pages": total.div_ceil(per_page),
                }
            }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct DomainPayload {
    domain: String,
}

#[derive(Deserialize)]
struct UpdateVhostPayload {
    domain: String,
    owner: Option<String>,
    #[serde(alias = "php")]
    php_version: Option<String>,
    package: Option<String>,
    email: Option<String>,
}

async fn vhost_delete_handler(Json(payload): Json<DomainPayload>) -> Json<serde_json::Value> {
    use crate::services::nitro::NitroEngine;
    match NitroEngine::delete_vhost(&payload.domain) {
        Ok(msg) => {
            let mut warnings: Vec<String> = Vec::new();

            let pdns = PowerDnsManager::new();
            if let Err(e) = pdns.delete_zone(&payload.domain) {
                warnings.push(format!("DNS zone cleanup warning: {}", e));
            }
            if let Err(e) = WebsitesManager::cleanup_for_domain(&payload.domain) {
                warnings.push(format!("Website workflow cleanup warning: {}", e));
            }

            AuditLogger::log("admin", "vhost.delete", &payload.domain, "panel");
            Json(json!({
                "status": "success",
                "message": msg,
                "warnings": warnings,
            }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn vhost_suspend_handler(Json(payload): Json<DomainPayload>) -> Json<serde_json::Value> {
    use crate::services::nitro::NitroEngine;
    match NitroEngine::suspend_vhost(&payload.domain) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn vhost_unsuspend_handler(Json(payload): Json<DomainPayload>) -> Json<serde_json::Value> {
    use crate::services::nitro::NitroEngine;
    match NitroEngine::unsuspend_vhost(&payload.domain) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn vhost_update_handler(Json(payload): Json<UpdateVhostPayload>) -> Json<serde_json::Value> {
    let domain = payload.domain.clone();
    let req = VHostUpdateConfig {
        domain: domain.clone(),
        owner: payload.owner,
        php_version: payload.php_version,
        package: payload.package,
        email: payload.email,
    };

    match NitroEngine::update_vhost(&req) {
        Ok(data) => {
            let mut warnings: Vec<String> = Vec::new();
            let pdns = PowerDnsManager::new();
            match resolve_dns_server_ip(None) {
                Ok(ip) => {
                    if ip == "127.0.0.1" {
                        warnings.push("DNS reconcile 127.0.0.1 fallback ile calisti.".to_string());
                    }
                    if let Err(e) = pdns.reconcile_zone_defaults(&domain, &ip) {
                        warnings.push(format!("DNS reconcile warning: {}", e));
                    }
                }
                Err(e) => warnings.push(format!("DNS reconcile warning: {}", e)),
            }

            Json(json!({
                "status": "success",
                "message": format!("{} website guncellendi.", data.domain),
                "data": data,
                "warnings": warnings,
            }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct SubdomainListQuery {
    domain: Option<String>,
}

#[derive(Deserialize)]
struct SubdomainDeleteQuery {
    fqdn: String,
    delete_docroot: Option<bool>,
}

#[derive(Deserialize)]
struct DbLinksListQuery {
    domain: Option<String>,
}

#[derive(Deserialize)]
struct DbLinkDeleteQuery {
    domain: String,
    engine: String,
    db_name: String,
}

#[derive(Deserialize)]
struct DbLinkVerifyPayload {
    domain: String,
    engine: String,
    db_name: String,
    db_user: String,
}

#[derive(Deserialize)]
struct ConvertSubdomainPayload {
    fqdn: String,
    owner: Option<String>,
    php_version: Option<String>,
    package: Option<String>,
    email: Option<String>,
}

#[derive(Deserialize)]
struct AliasListQuery {
    domain: Option<String>,
}

#[derive(Deserialize)]
struct AliasDeleteQuery {
    domain: String,
    alias: String,
}

#[derive(Deserialize)]
struct AdvancedConfigQuery {
    domain: String,
}

#[derive(Deserialize)]
struct CustomSslQuery {
    domain: String,
}

async fn websites_subdomains_list_handler(
    axum::extract::Query(query): axum::extract::Query<SubdomainListQuery>,
) -> Json<serde_json::Value> {
    match WebsitesManager::list_subdomains(query.domain.as_deref()) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_subdomains_create_handler(
    Json(payload): Json<CreateSubdomainRequest>,
) -> Json<serde_json::Value> {
    match WebsitesManager::create_subdomain(&payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_subdomains_delete_handler(
    axum::extract::Query(query): axum::extract::Query<SubdomainDeleteQuery>,
) -> Json<serde_json::Value> {
    let delete_docroot = query.delete_docroot.unwrap_or(false);
    match WebsitesManager::delete_subdomain_with_options(&query.fqdn, delete_docroot) {
        Ok(removed_docroots) => Json(json!({
            "status": "success",
            "message": if delete_docroot {
                "Subdomain ve secili docrootlar silindi."
            } else {
                "Subdomain silindi."
            },
            "data": {
                "docroot_deleted": delete_docroot,
                "removed_paths": removed_docroots,
            }
        })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_subdomains_php_update_handler(
    Json(payload): Json<SubdomainPhpUpdateRequest>,
) -> Json<serde_json::Value> {
    match WebsitesManager::update_subdomain_php(&payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_subdomains_convert_handler(
    Json(payload): Json<ConvertSubdomainPayload>,
) -> Json<serde_json::Value> {
    let consume_req = ConvertSubdomainRequest {
        fqdn: payload.fqdn.clone(),
    };

    let subdomain = match WebsitesManager::consume_subdomain_for_conversion(&consume_req) {
        Ok(item) => item,
        Err(e) => return Json(json!({ "status": "error", "message": e })),
    };

    let owner = payload.owner.unwrap_or_else(|| "aura".to_string());
    let php_version = payload
        .php_version
        .unwrap_or_else(|| subdomain.php_version.clone());

    let create_cfg = VHostConfig {
        domain: subdomain.fqdn.clone(),
        user: owner.clone(),
        php_version: php_version.clone(),
    };

    if let Err(e) = NitroEngine::create_vhost(&create_cfg) {
        let rollback = CreateSubdomainRequest {
            parent_domain: subdomain.parent_domain,
            subdomain: subdomain.subdomain,
            php_version: subdomain.php_version,
        };
        let _ = WebsitesManager::create_subdomain(&rollback);
        return Json(json!({
            "status": "error",
            "message": format!("Subdomain website'e donusturulemedi: {}", e),
        }));
    }

    let update_req = VHostUpdateConfig {
        domain: create_cfg.domain.clone(),
        owner: Some(owner),
        php_version: Some(php_version),
        package: payload.package,
        email: payload.email,
    };

    let update_data = match NitroEngine::update_vhost(&update_req) {
        Ok(data) => data,
        Err(e) => {
            return Json(json!({
                "status": "error",
                "message": format!("Website olusturuldu ama metadata guncellenemedi: {}", e),
            }))
        }
    };

    Json(json!({
        "status": "success",
        "message": format!("{} full website olarak donusturuldu.", update_data.domain),
        "data": update_data,
    }))
}

async fn websites_db_links_list_handler(
    axum::extract::Query(query): axum::extract::Query<DbLinksListQuery>,
) -> Json<serde_json::Value> {
    match WebsitesManager::list_db_links(query.domain.as_deref()) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_db_links_create_handler(
    Json(payload): Json<WebsiteDbLinkRequest>,
) -> Json<serde_json::Value> {
    match WebsitesManager::attach_db(&payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_db_links_delete_handler(
    axum::extract::Query(query): axum::extract::Query<DbLinkDeleteQuery>,
) -> Json<serde_json::Value> {
    match WebsitesManager::detach_db(&query.domain, &query.engine, &query.db_name) {
        Ok(_) => Json(json!({ "status": "success", "message": "DB baglantisi kaldirildi." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_db_links_verify_handler(
    Json(payload): Json<DbLinkVerifyPayload>,
) -> Json<serde_json::Value> {
    let linked = match WebsitesManager::list_db_links(Some(&payload.domain)) {
        Ok(links) => links.into_iter().any(|x| {
            x.domain == payload.domain
                && x.engine == payload.engine
                && x.db_name == payload.db_name
                && x.db_user == payload.db_user
        }),
        Err(e) => {
            return Json(json!({ "status": "error", "message": e }));
        }
    };

    match check_connection_readiness(&payload.engine, &payload.db_name, &payload.db_user) {
        Ok(result) => Json(json!({
            "status": "success",
            "data": {
                "linked": linked,
                "connection": result,
                "ready": linked && result.ready,
            }
        })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_aliases_list_handler(
    axum::extract::Query(query): axum::extract::Query<AliasListQuery>,
) -> Json<serde_json::Value> {
    match WebsitesManager::list_aliases(query.domain.as_deref()) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_aliases_create_handler(
    Json(payload): Json<WebsiteAliasRequest>,
) -> Json<serde_json::Value> {
    match WebsitesManager::add_alias(&payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_aliases_delete_handler(
    axum::extract::Query(query): axum::extract::Query<AliasDeleteQuery>,
) -> Json<serde_json::Value> {
    match WebsitesManager::delete_alias(&query.domain, &query.alias) {
        Ok(_) => Json(json!({ "status": "success", "message": "Alias silindi." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_advanced_config_get_handler(
    axum::extract::Query(query): axum::extract::Query<AdvancedConfigQuery>,
) -> Json<serde_json::Value> {
    match WebsitesManager::get_advanced_config(&query.domain) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_open_basedir_set_handler(
    Json(payload): Json<WebsiteOpenBasedirRequest>,
) -> Json<serde_json::Value> {
    match WebsitesManager::set_open_basedir(&payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_rewrite_save_handler(
    Json(payload): Json<WebsiteRewriteRequest>,
) -> Json<serde_json::Value> {
    match WebsitesManager::save_rewrite_rules(&payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_vhost_config_save_handler(
    Json(payload): Json<WebsiteVhostConfigRequest>,
) -> Json<serde_json::Value> {
    match WebsitesManager::save_vhost_config(&payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_custom_ssl_get_handler(
    axum::extract::Query(query): axum::extract::Query<CustomSslQuery>,
) -> Json<serde_json::Value> {
    match WebsitesManager::get_custom_ssl(&query.domain) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn websites_custom_ssl_save_handler(
    Json(payload): Json<WebsiteCustomSslRequest>,
) -> Json<serde_json::Value> {
    match WebsitesManager::save_custom_ssl(&payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// ─── DNS Zones List Handler ───────────────────────────────────────────────────

async fn dns_zones_list_handler() -> Json<serde_json::Value> {
    let pdns = PowerDnsManager::new();
    let zones = pdns.list_zones();
    Json(json!({ "status": "success", "data": zones }))
}

async fn delete_dns_zone_handler(
    axum::extract::Path(domain): axum::extract::Path<String>,
) -> Json<serde_json::Value> {
    let pdns = PowerDnsManager::new();
    match pdns.delete_zone(&domain) {
        Ok(_) => {
            Json(json!({ "status": "success", "message": format!("DNS Zone {} deleted.", domain) }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn get_dns_records_handler(
    axum::extract::Path(domain): axum::extract::Path<String>,
) -> Json<serde_json::Value> {
    let pdns = PowerDnsManager::new();
    let records = pdns.get_records(&domain);
    Json(json!({ "status": "success", "data": records }))
}

#[derive(Deserialize)]
struct DnsRecordPayload {
    name: String,
    record_type: String,
    content: String,
    ttl: u32,
}

#[derive(Deserialize)]
struct DnsReconcilePayload {
    domain: String,
    server_ip: Option<String>,
}

async fn add_dns_record_handler(
    axum::extract::Path(domain): axum::extract::Path<String>,
    Json(payload): Json<DnsRecordPayload>,
) -> Json<serde_json::Value> {
    let pdns = PowerDnsManager::new();
    use crate::services::dns::DnsRecord;
    let record = DnsRecord {
        name: payload.name,
        record_type: payload.record_type,
        content: payload.content,
        ttl: payload.ttl,
    };
    match pdns.add_record(&domain, record).await {
        Ok(_) => Json(json!({ "status": "success", "message": "Record added." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn dns_reconcile_handler(
    Json(payload): Json<DnsReconcilePayload>,
) -> Json<serde_json::Value> {
    let domain = payload.domain.trim().to_lowercase();
    if domain.is_empty() {
        return Json(json!({
            "status": "error",
            "message": "domain zorunludur.",
        }));
    }

    let server_ip = match resolve_dns_server_ip(payload.server_ip) {
        Ok(ip) => ip,
        Err(e) => {
            return Json(json!({
                "status": "error",
                "message": e,
            }))
        }
    };

    let pdns = PowerDnsManager::new();
    match pdns.reconcile_zone_defaults(&domain, &server_ip) {
        Ok(result) => Json(json!({
            "status": "success",
            "message": format!("{} zone mutabakati tamamlandi.", domain),
            "data": result,
        })),
        Err(e) => Json(json!({
            "status": "error",
            "message": e,
        })),
    }
}

#[derive(Deserialize)]
struct DeleteRecordQuery {
    record_type: String,
    name: String,
}

#[derive(Deserialize)]
struct DnsSecPayload {
    enabled: bool,
}

#[derive(Deserialize)]
struct DefaultNsWizardPayload {
    base_domain: String,
}

async fn delete_dns_record_handler(
    axum::extract::Path(domain): axum::extract::Path<String>,
    axum::extract::Query(query): axum::extract::Query<DeleteRecordQuery>,
) -> Json<serde_json::Value> {
    let pdns = PowerDnsManager::new();
    match pdns.delete_record(&domain, &query.record_type, &query.name) {
        Ok(_) => Json(json!({ "status": "success", "message": "Record deleted." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn set_dnssec_handler(
    axum::extract::Path(domain): axum::extract::Path<String>,
    Json(payload): Json<DnsSecPayload>,
) -> Json<serde_json::Value> {
    let pdns = PowerDnsManager::new();
    match pdns.set_dnssec_enabled(&domain, payload.enabled) {
        Ok(zone) => Json(json!({
            "status": "success",
            "message": format!("{} icin DNSSEC {}.", domain, if payload.enabled { "etkin" } else { "devre disi" }),
            "data": zone,
        })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn get_default_ns_handler() -> Json<serde_json::Value> {
    let pdns = PowerDnsManager::new();
    let ns = pdns.get_default_nameservers();
    Json(json!({ "status": "success", "data": ns }))
}

use crate::services::dns::DefaultNameservers;
async fn set_default_ns_handler(
    Json(payload): Json<DefaultNameservers>,
) -> Json<serde_json::Value> {
    let pdns = PowerDnsManager::new();
    match pdns.set_default_nameservers(payload) {
        Ok(_) => Json(json!({ "status": "success", "message": "Default nameservers updated." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn default_ns_wizard_handler(
    Json(payload): Json<DefaultNsWizardPayload>,
) -> Json<serde_json::Value> {
    let pdns = PowerDnsManager::new();
    match pdns.suggest_default_nameservers(&payload.base_domain) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn default_ns_reset_handler() -> Json<serde_json::Value> {
    let pdns = PowerDnsManager::new();
    match pdns.reset_default_nameservers() {
        Ok(data) => Json(json!({
            "status": "success",
            "message": "Default nameservers resetlendi.",
            "data": data
        })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// ─── Mail List/Delete Handlers ────────────────────────────────────────────────

async fn mail_list_handler() -> Json<serde_json::Value> {
    let mailboxes = MailManager::list_mailboxes();
    Json(json!({ "status": "success", "data": mailboxes }))
}

#[derive(Deserialize)]
struct MailDeletePayload {
    address: String,
}

#[derive(Deserialize)]
struct MailForwardsListQuery {
    domain: Option<String>,
}

#[derive(Deserialize)]
struct MailCatchAllQuery {
    domain: String,
}

#[derive(Deserialize)]
struct MailDkimRotatePayload {
    domain: String,
}

#[derive(Deserialize)]
struct MailRoutingListQuery {
    domain: Option<String>,
}

async fn mail_delete_handler(Json(payload): Json<MailDeletePayload>) -> Json<serde_json::Value> {
    match MailManager::delete_mailbox(&payload.address).await {
        Ok(_) => Json(
            json!({ "status": "success", "message": format!("{} basariyla silindi.", payload.address) }),
        ),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mail_password_reset_handler(
    Json(payload): Json<MailboxPasswordResetRequest>,
) -> Json<serde_json::Value> {
    match MailManager::reset_mailbox_password(&payload) {
        Ok(_) => Json(
            json!({ "status": "success", "message": format!("{} sifresi guncellendi.", payload.address) }),
        ),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mail_forwards_list_handler(
    axum::extract::Query(query): axum::extract::Query<MailForwardsListQuery>,
) -> Json<serde_json::Value> {
    match MailManager::list_forwards(query.domain.as_deref()) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mail_forwards_create_handler(
    Json(payload): Json<MailForwardConfig>,
) -> Json<serde_json::Value> {
    match MailManager::add_forward(&payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mail_forwards_delete_handler(
    Json(payload): Json<MailForwardDeleteRequest>,
) -> Json<serde_json::Value> {
    match MailManager::delete_forward(&payload) {
        Ok(_) => Json(json!({ "status": "success", "message": "Forward rule deleted." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mail_catch_all_get_handler(
    axum::extract::Query(query): axum::extract::Query<MailCatchAllQuery>,
) -> Json<serde_json::Value> {
    match MailManager::get_catch_all(&query.domain) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mail_catch_all_set_handler(
    Json(payload): Json<MailCatchAllConfig>,
) -> Json<serde_json::Value> {
    match MailManager::set_catch_all(&payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mail_routing_list_handler(
    axum::extract::Query(query): axum::extract::Query<MailRoutingListQuery>,
) -> Json<serde_json::Value> {
    match MailManager::list_routing_rules(query.domain.as_deref()) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mail_routing_create_handler(
    Json(payload): Json<MailRoutingConfig>,
) -> Json<serde_json::Value> {
    match MailManager::add_routing_rule(&payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mail_routing_delete_handler(
    Json(payload): Json<MailRoutingDeleteRequest>,
) -> Json<serde_json::Value> {
    match MailManager::delete_routing_rule(&payload) {
        Ok(_) => Json(json!({ "status": "success", "message": "Routing rule deleted." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mail_dkim_get_handler(
    axum::extract::Query(query): axum::extract::Query<MailCatchAllQuery>,
) -> Json<serde_json::Value> {
    match MailManager::get_dkim(&query.domain) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mail_dkim_rotate_handler(
    Json(payload): Json<MailDkimRotatePayload>,
) -> Json<serde_json::Value> {
    match MailManager::rotate_dkim(&payload.domain) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn mail_webmail_sso_handler(
    Json(payload): Json<MailWebmailSsoRequest>,
) -> Json<serde_json::Value> {
    match MailManager::generate_webmail_sso_link(&payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// ─── Users Handlers ───────────────────────────────────────────────────────────

#[derive(Deserialize)]
struct AclDeletePolicyQuery {
    id: String,
}

#[derive(Deserialize)]
struct AclDeleteAssignmentQuery {
    username: String,
}

#[derive(Deserialize)]
struct AclEffectiveQuery {
    username: String,
}

async fn reseller_quotas_list_handler() -> Json<serde_json::Value> {
    match ResellerManager::list_quotas() {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn reseller_quotas_upsert_handler(
    Json(payload): Json<ResellerQuota>,
) -> Json<serde_json::Value> {
    match ResellerManager::upsert_quota(payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn reseller_whitelabel_list_handler() -> Json<serde_json::Value> {
    match ResellerManager::list_white_labels() {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn reseller_whitelabel_upsert_handler(
    Json(payload): Json<WhiteLabelConfig>,
) -> Json<serde_json::Value> {
    match ResellerManager::upsert_white_label(payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn acl_policies_list_handler() -> Json<serde_json::Value> {
    match ResellerManager::list_policies() {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn acl_policies_upsert_handler(Json(payload): Json<AclPolicy>) -> Json<serde_json::Value> {
    match ResellerManager::upsert_policy(payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn acl_policies_delete_handler(
    axum::extract::Query(query): axum::extract::Query<AclDeletePolicyQuery>,
) -> Json<serde_json::Value> {
    match ResellerManager::delete_policy(&query.id) {
        Ok(_) => Json(json!({ "status": "success", "message": "ACL policy deleted." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn acl_assignments_list_handler() -> Json<serde_json::Value> {
    match ResellerManager::list_assignments() {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn acl_assignments_upsert_handler(
    Json(payload): Json<AclAssignment>,
) -> Json<serde_json::Value> {
    match ResellerManager::assign_policy(payload) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn acl_assignments_delete_handler(
    axum::extract::Query(query): axum::extract::Query<AclDeleteAssignmentQuery>,
) -> Json<serde_json::Value> {
    match ResellerManager::remove_assignment(&query.username) {
        Ok(_) => Json(json!({ "status": "success", "message": "ACL assignment deleted." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn acl_effective_permissions_handler(
    axum::extract::Query(query): axum::extract::Query<AclEffectiveQuery>,
) -> Json<serde_json::Value> {
    match ResellerManager::effective_permissions(&query.username) {
        Ok(data) => Json(json!({ "status": "success", "data": data })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn users_list_handler() -> Json<serde_json::Value> {
    match UserManager::list_users() {
        Ok(users) => Json(json!({ "status": "success", "data": users })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn users_create_handler(Json(payload): Json<CreateUserRequest>) -> Json<serde_json::Value> {
    match UserManager::create_user(&payload) {
        Ok(msg) => {
            AuditLogger::log("admin", "user.create", &payload.username, "panel");
            Json(json!({ "status": "success", "message": msg }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct UsernamePayload {
    username: String,
}

async fn users_delete_handler(Json(payload): Json<UsernamePayload>) -> Json<serde_json::Value> {
    match UserManager::delete_user(&payload.username) {
        Ok(msg) => {
            AuditLogger::log("admin", "user.delete", &payload.username, "panel");
            Json(json!({ "status": "success", "message": msg }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// ─── Packages Handlers ────────────────────────────────────────────────────────

async fn packages_list_handler() -> Json<serde_json::Value> {
    match PackageManager::list_packages() {
        Ok(packages) => Json(json!({ "status": "success", "data": packages })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn packages_create_handler(
    Json(payload): Json<CreatePackageRequest>,
) -> Json<serde_json::Value> {
    match PackageManager::create_package(&payload) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct PackageIdPayload {
    id: u64,
}

async fn packages_update_handler(
    Json(payload): Json<UpdatePackageRequest>,
) -> Json<serde_json::Value> {
    match PackageManager::update_package(&payload) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn packages_delete_handler(Json(payload): Json<PackageIdPayload>) -> Json<serde_json::Value> {
    match PackageManager::delete_package(payload.id) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// ─── Users — Change Password ──────────────────────────────────────────────────

async fn users_change_password_handler(
    Json(payload): Json<ChangePasswordRequest>,
) -> Json<serde_json::Value> {
    match UserManager::change_password(&payload) {
        Ok(msg) => {
            AuditLogger::log("admin", "user.change_password", &payload.username, "panel");
            Json(json!({ "status": "success", "message": msg }))
        }
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// ─── Activity Log ─────────────────────────────────────────────────────────────

#[derive(Deserialize)]
struct ActivityLogQuery {
    user: Option<String>,
    page: Option<usize>,
    per_page: Option<usize>,
}

async fn activity_log_handler(
    axum::extract::Query(q): axum::extract::Query<ActivityLogQuery>,
) -> Json<serde_json::Value> {
    let page = q.page.unwrap_or(0);
    let per_page = q.per_page.unwrap_or(50).min(200);
    match AuditLogger::list(q.user.as_deref(), page, per_page) {
        Ok((entries, total)) => Json(json!({
            "status": "success",
            "data": entries,
            "total": total,
            "page": page,
            "per_page": per_page,
        })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

// ─── Database Backup ──────────────────────────────────────────────────────────

async fn db_backup_list_handler() -> Json<serde_json::Value> {
    let entries = DbBackupManager::list_backups();
    Json(json!({ "status": "success", "data": entries }))
}

async fn db_backup_create_handler(Json(payload): Json<DbBackupRequest>) -> Json<serde_json::Value> {
    match DbBackupManager::create_backup(&payload) {
        Ok(entry) => Json(json!({ "status": "success", "data": entry })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn db_backup_restore_handler(
    Json(payload): Json<DbRestoreRequest>,
) -> Json<serde_json::Value> {
    match DbBackupManager::restore_backup(&payload.backup_id) {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[derive(Deserialize)]
struct BackupIdPayload {
    backup_id: String,
}

#[derive(Deserialize)]
struct BackupDownloadQuery {
    id: String,
}

async fn db_backup_delete_handler(Json(payload): Json<BackupIdPayload>) -> Json<serde_json::Value> {
    match DbBackupManager::delete_backup(&payload.backup_id) {
        Ok(()) => Json(json!({ "status": "success", "message": "Backup silindi." })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

async fn db_backup_download_handler(
    axum::extract::Query(q): axum::extract::Query<BackupDownloadQuery>,
) -> axum::response::Response {
    use axum::body::Body;
    use axum::http::{header, HeaderValue, StatusCode};
    use axum::response::IntoResponse;

    let path = match DbBackupManager::backup_file_path(&q.id) {
        Ok(p) => p,
        Err(e) => {
            return (
                StatusCode::NOT_FOUND,
                Json(json!({ "status": "error", "message": e })),
            )
                .into_response()
        }
    };

    let data = match std::fs::read(&path) {
        Ok(bytes) => bytes,
        Err(e) => {
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(json!({
                    "status": "error",
                    "message": format!("Backup dosyasi okunamadi: {}", e),
                })),
            )
                .into_response()
        }
    };

    let filename = path
        .file_name()
        .and_then(|v| v.to_str())
        .unwrap_or("backup.sql.gz");
    let disposition = format!("attachment; filename=\"{}\"", filename);

    let mut response = Body::from(data).into_response();
    response.headers_mut().insert(
        header::CONTENT_TYPE,
        HeaderValue::from_static("application/gzip"),
    );
    if let Ok(v) = HeaderValue::from_str(&disposition) {
        response
            .headers_mut()
            .insert(header::CONTENT_DISPOSITION, v);
    }
    response
}

// ─── SSL Wildcard ─────────────────────────────────────────────────────────────

async fn issue_wildcard_ssl_handler(Json(payload): Json<SslConfig>) -> Json<serde_json::Value> {
    match SslManager::issue_wildcard_certificate(&payload).await {
        Ok(msg) => Json(json!({ "status": "success", "message": msg })),
        Err(e) => Json(json!({ "status": "error", "message": e })),
    }
}

#[cfg(test)]
mod tests {
    use super::routes;
    use crate::auth::jwt;
    use crate::services::users::{CreateUserRequest, UserManager};
    use crate::test_support::env_lock;
    use axum::body::Body;
    use axum::http::{Method, Request, StatusCode};
    use serde_json::json;
    use std::time::{SystemTime, UNIX_EPOCH};
    use tower::util::ServiceExt;

    fn setup_env(test_name: &str) -> std::path::PathBuf {
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|d| d.as_nanos())
            .unwrap_or(0);
        let path = std::env::temp_dir().join(format!("aurapanel-api-test-{}-{}", test_name, now));
        std::fs::create_dir_all(&path).expect("temp state dir");
        std::env::set_var("AURAPANEL_STATE_DIR", &path);
        std::env::set_var("AURAPANEL_JWT_SECRET", "test-secret");
        path
    }

    fn teardown_env(path: &std::path::Path) {
        std::env::remove_var("AURAPANEL_STATE_DIR");
        std::env::remove_var("AURAPANEL_JWT_SECRET");
        let _ = std::fs::remove_dir_all(path);
    }

    #[tokio::test]
    async fn health_route_is_reachable() {
        let app = routes();
        let response = app
            .oneshot(
                Request::builder()
                    .method(Method::GET)
                    .uri("/health")
                    .body(Body::empty())
                    .expect("request"),
            )
            .await
            .expect("response");

        assert_eq!(response.status(), StatusCode::OK);
    }

    #[tokio::test]
    async fn protected_routes_require_auth() {
        let app = routes();
        let response = app
            .oneshot(
                Request::builder()
                    .method(Method::GET)
                    .uri("/status/services")
                    .body(Body::empty())
                    .expect("request"),
            )
            .await
            .expect("response");

        assert_eq!(response.status(), StatusCode::UNAUTHORIZED);
    }

    #[tokio::test]
    async fn login_route_returns_token_for_valid_credentials() {
        let _guard = env_lock().lock().expect("test lock");
        let state_dir = setup_env("login");
        UserManager::create_user(&CreateUserRequest {
            username: "admin".to_string(),
            email: "admin@example.com".to_string(),
            password: "supersecret".to_string(),
            role: "admin".to_string(),
            package: "default".to_string(),
        })
        .expect("create user");

        let app = routes();
        let response = app
            .oneshot(
                Request::builder()
                    .method(Method::POST)
                    .uri("/auth/login")
                    .header("content-type", "application/json")
                    .body(Body::from(
                        r#"{"email":"admin@example.com","password":"supersecret"}"#,
                    ))
                    .expect("request"),
            )
            .await
            .expect("response");

        assert_eq!(response.status(), StatusCode::OK);
        teardown_env(&state_dir);
    }

    #[tokio::test]
    async fn status_panel_port_route_is_reachable_with_auth() {
        let _guard = env_lock().lock().expect("test lock");
        let state_dir = setup_env("panel-port");
        UserManager::create_user(&CreateUserRequest {
            username: "admin".to_string(),
            email: "admin@example.com".to_string(),
            password: "supersecret".to_string(),
            role: "admin".to_string(),
            package: "default".to_string(),
        })
        .expect("create user");
        let token = jwt::create_token("admin", "admin").expect("token");

        let app = routes();
        let response = app
            .oneshot(
                Request::builder()
                    .method(Method::GET)
                    .uri("/status/panel-port")
                    .header("authorization", format!("Bearer {}", token))
                    .body(Body::empty())
                    .expect("request"),
            )
            .await
            .expect("response");

        assert_eq!(response.status(), StatusCode::OK);
        teardown_env(&state_dir);
    }

    #[tokio::test]
    async fn reseller_cannot_access_admin_endpoints() {
        let _guard = env_lock().lock().expect("test lock");
        let state_dir = setup_env("reseller-rbac");
        UserManager::create_user(&CreateUserRequest {
            username: "reseller1".to_string(),
            email: "reseller@example.com".to_string(),
            password: "supersecret".to_string(),
            role: "reseller".to_string(),
            package: "default".to_string(),
        })
        .expect("create reseller");
        let token = jwt::create_token("reseller1", "reseller").expect("token");

        let response = routes()
            .oneshot(
                Request::builder()
                    .method(Method::GET)
                    .uri("/users/list")
                    .header("authorization", format!("Bearer {}", token))
                    .body(Body::empty())
                    .expect("request"),
            )
            .await
            .expect("response");

        assert_eq!(response.status(), StatusCode::FORBIDDEN);
        teardown_env(&state_dir);
    }

    #[tokio::test]
    async fn user_can_access_basic_status_endpoint() {
        let _guard = env_lock().lock().expect("test lock");
        let state_dir = setup_env("user-rbac");
        UserManager::create_user(&CreateUserRequest {
            username: "user1".to_string(),
            email: "user@example.com".to_string(),
            password: "supersecret".to_string(),
            role: "user".to_string(),
            package: "default".to_string(),
        })
        .expect("create user");
        let token = jwt::create_token("user1", "user").expect("token");

        let response = routes()
            .oneshot(
                Request::builder()
                    .method(Method::GET)
                    .uri("/status/metrics")
                    .header("authorization", format!("Bearer {}", token))
                    .body(Body::empty())
                    .expect("request"),
            )
            .await
            .expect("response");

        assert_ne!(response.status(), StatusCode::FORBIDDEN);
        assert_ne!(response.status(), StatusCode::UNAUTHORIZED);
        teardown_env(&state_dir);
    }

    fn route_specs_from_source() -> Vec<(Method, String)> {
        let source = include_str!("routes.rs");
        let mut out = Vec::new();

        for line in source.lines() {
            let trimmed = line.trim();
            if !trimmed.starts_with(".route(\"") {
                continue;
            }

            let method = if trimmed.contains(", get(") {
                Method::GET
            } else if trimmed.contains(", post(") {
                Method::POST
            } else if trimmed.contains(", axum::routing::delete(") {
                Method::DELETE
            } else {
                continue;
            };

            let path_start = match trimmed.find(".route(\"") {
                Some(idx) => idx + ".route(\"".len(),
                None => continue,
            };
            let rest = &trimmed[path_start..];
            let path_end = match rest.find('"') {
                Some(idx) => idx,
                None => continue,
            };
            let raw_path = &rest[..path_end];

            let normalized_path = raw_path
                .split('/')
                .map(|seg| {
                    if seg.starts_with(':') && seg.len() > 1 {
                        "example.com"
                    } else {
                        seg
                    }
                })
                .collect::<Vec<_>>()
                .join("/");

            out.push((method, normalized_path));
        }

        out
    }

    #[tokio::test]
    async fn all_defined_routes_are_reachable_without_404_405_or_5xx() {
        let _guard = env_lock().lock().expect("test lock");
        let state_dir = setup_env("all-routes-smoke");
        UserManager::create_user(&CreateUserRequest {
            username: "admin".to_string(),
            email: "admin@example.com".to_string(),
            password: "supersecret".to_string(),
            role: "admin".to_string(),
            package: "default".to_string(),
        })
        .expect("create user");
        let token = jwt::create_token("admin", "admin").expect("token");

        let route_specs = route_specs_from_source();
        assert!(!route_specs.is_empty(), "route specs must not be empty");

        for (method, path) in route_specs {
            let mut uri = path.clone();
            if path == "/terminal/ws" {
                uri = format!("/terminal/ws?token={}", token);
            }

            let body = if method == Method::POST || method == Method::DELETE {
                if path == "/auth/login" {
                    Body::from(
                        json!({
                            "email": "admin@example.com",
                            "password": "supersecret"
                        })
                        .to_string(),
                    )
                } else {
                    Body::from("{}")
                }
            } else {
                Body::empty()
            };

            let mut req = Request::builder().method(method.clone()).uri(uri);
            if path != "/health" && path != "/auth/login" {
                req = req.header("authorization", format!("Bearer {}", token));
            }
            if method == Method::POST || method == Method::DELETE {
                req = req.header("content-type", "application/json");
            }

            let response = routes()
                .oneshot(req.body(body).expect("request"))
                .await
                .expect("response");
            let status = response.status();

            assert_ne!(
                status,
                StatusCode::NOT_FOUND,
                "{} {} returned 404",
                method,
                path
            );
            assert_ne!(
                status,
                StatusCode::METHOD_NOT_ALLOWED,
                "{} {} returned 405",
                method,
                path
            );
            assert!(
                !status.is_server_error(),
                "{} {} returned server error {}",
                method,
                path,
                status
            );
        }

        teardown_env(&state_dir);
    }
}
