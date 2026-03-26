use axum::{
    routing::get,
    Router,
};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

mod api;
mod auth;
mod services;
mod config;

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

    // build our application with a route
    let app = Router::new()
        .route("/", get(|| async { "AuraPanel Core - System is healthy." }))
        .nest("/api/v1", api::routes());

    // run it
    let listener = tokio::net::TcpListener::bind("127.0.0.1:8000")
        .await
        .unwrap();
    tracing::info!("Core listening on {}", listener.local_addr().unwrap());
    
    axum::serve(listener, app).await.unwrap();

    Ok(())
}
