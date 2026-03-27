// Development-phase lint suppression (remove before 1.0 release)
#![allow(unused_imports)]
#![allow(dead_code)]
#![allow(unused_variables)]

use axum::{routing::get, Router};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

mod api;
mod auth;
mod config;
mod runtime;
mod services;
#[cfg(test)]
mod test_support;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    // structured logging init
    tracing_subscriber::registry()
        .with(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "aurapanel_core=debug,tower_http=debug".into()),
        )
        .with(tracing_subscriber::fmt::layer())
        .init();

    tracing::info!("AuraPanel Micro-Core starting up...");
    runtime::validate_startup().map_err(anyhow::Error::msg)?;
    tracing::info!("Runtime mode: {}", runtime::mode_name());
    tracing::info!("Security policy: {}", runtime::security_policy_name());
    tracing::info!("Gateway-only mode: {}", runtime::gateway_only_enabled());
    let bind_addr = runtime::core_bind_addr();

    // Bootstrap users
    if let Err(e) = services::users::UserManager::seed_default_admin() {
        tracing::warn!("Failed to seed default admin: {}", e);
    }

    // build our application with a route
    let app = Router::new()
        .route("/", get(|| async { "AuraPanel Core - System is healthy." }))
        .nest("/api/v1", api::routes());

    // run it
    let listener = tokio::net::TcpListener::bind(&bind_addr).await.unwrap();
    tracing::info!("Core listening on {}", listener.local_addr().unwrap());

    axum::serve(listener, app).await.unwrap();

    Ok(())
}
