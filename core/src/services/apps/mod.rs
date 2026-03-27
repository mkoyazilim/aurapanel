pub mod nodejs;
pub mod python;

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::process::Command;
use std::sync::{Mutex, OnceLock};

use self::nodejs::NodeManager;
use self::python::PythonManager;

#[derive(Serialize, Deserialize, Debug)]
pub struct CmsInstallConfig {
    pub domain: String,
    pub app_type: String, // "wordpress", "laravel"
    pub db_name: String,
    pub db_user: String,
    pub db_pass: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct NodeAppRequest {
    pub app_name: String,
    pub start_script: String,
    pub dir: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct PythonAppRequest {
    pub app_name: String,
    pub wsgi_module: String,
    pub dir: String,
    pub port: u16,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct RuntimeAppInfo {
    pub app_name: String,
    pub runtime: String,
    pub dir: String,
    pub status: String,
}

#[derive(Default)]
struct RuntimeState {
    apps: HashMap<String, RuntimeAppInfo>,
}

fn runtime_state() -> &'static Mutex<RuntimeState> {
    static STATE: OnceLock<Mutex<RuntimeState>> = OnceLock::new();
    STATE.get_or_init(|| Mutex::new(RuntimeState::default()))
}

fn command_available(cmd: &str) -> bool {
    Command::new("which")
        .arg(cmd)
        .output()
        .map(|o| o.status.success())
        .unwrap_or(false)
}

pub struct AppManager;

impl AppManager {
    /// Tek tıklamayla WordPress, Laravel veya benzeri CMS'leri indirip kurar
    pub async fn install_cms(config: &CmsInstallConfig) -> Result<(), String> {
        let public_html = format!("/home/aurapanel/public_html/{}", config.domain);

        match config.app_type.as_str() {
            "wordpress" => {
                if !command_available("wp") {
                    return Err("wp-cli is not installed.".to_string());
                }
                std::fs::create_dir_all(&public_html)
                    .map_err(|e| format!("Failed to create target directory: {}", e))?;

                let output = Command::new("wp")
                    .args(["core", "download", "--allow-root", "--path"])
                    .arg(&public_html)
                    .output()
                    .map_err(|e| format!("Failed to run wp-cli: {}", e))?;
                if !output.status.success() {
                    return Err(String::from_utf8_lossy(&output.stderr).to_string());
                }
            },
            "laravel" => {
                if !command_available("composer") {
                    return Err("composer is not installed.".to_string());
                }
                let output = Command::new("composer")
                    .args(["create-project", "--prefer-dist", "--no-interaction", "laravel/laravel"])
                    .arg(&public_html)
                    .output()
                    .map_err(|e| format!("Failed to run composer: {}", e))?;
                if !output.status.success() {
                    return Err(String::from_utf8_lossy(&output.stderr).to_string());
                }
            },
            _ => return Err(format!("Desteklenmeyen uygulama tipi: {}", config.app_type)),
        }

        Ok(())
    }

    pub fn list_runtime_apps() -> Result<Vec<RuntimeAppInfo>, String> {
        let guard = runtime_state().lock().map_err(|e| e.to_string())?;
        Ok(guard.apps.values().cloned().collect())
    }

    pub fn node_install_dependencies(dir: &str) -> Result<String, String> {
        if cfg!(target_os = "windows") {
            return Err("Node dependency install is not supported on Windows hosts.".to_string());
        }
        let ok = NodeManager::new()
            .install_dependencies(dir)
            .map_err(|e| e.to_string())?;
        if ok {
            Ok("Node dependencies installed.".to_string())
        } else {
            Err("Node dependencies install failed.".to_string())
        }
    }

    pub fn node_start(req: &NodeAppRequest) -> Result<String, String> {
        if cfg!(target_os = "windows") {
            return Err("Node runtime start is not supported on Windows hosts.".to_string());
        }
        let ok = NodeManager::new()
            .start_app(&req.app_name, &req.start_script, &req.dir)
            .map_err(|e| e.to_string())?;
        if ok {
            let mut guard = runtime_state().lock().map_err(|e| e.to_string())?;
            guard.apps.insert(
                req.app_name.clone(),
                RuntimeAppInfo {
                    app_name: req.app_name.clone(),
                    runtime: "node".to_string(),
                    dir: req.dir.clone(),
                    status: "running".to_string(),
                },
            );
            Ok("Node app started.".to_string())
        } else {
            Err("Node app start failed.".to_string())
        }
    }

    pub fn node_stop(app_name: &str) -> Result<String, String> {
        if cfg!(target_os = "windows") {
            return Err("Node runtime stop is not supported on Windows hosts.".to_string());
        }
        let ok = NodeManager::new().stop_app(app_name).map_err(|e| e.to_string())?;
        if ok {
            let mut guard = runtime_state().lock().map_err(|e| e.to_string())?;
            if let Some(app) = guard.apps.get_mut(app_name) {
                app.status = "stopped".to_string();
            }
            Ok("Node app stopped.".to_string())
        } else {
            Err("Node app stop failed.".to_string())
        }
    }

    pub fn python_create_venv(dir: &str) -> Result<String, String> {
        if cfg!(target_os = "windows") {
            return Err("Python venv creation is not supported on Windows hosts.".to_string());
        }
        let ok = PythonManager::new().create_venv(dir).map_err(|e| e.to_string())?;
        if ok {
            Ok("Python venv created.".to_string())
        } else {
            Err("Python venv creation failed.".to_string())
        }
    }

    pub fn python_install_requirements(dir: &str) -> Result<String, String> {
        if cfg!(target_os = "windows") {
            return Err("Python requirements install is not supported on Windows hosts.".to_string());
        }
        let ok = PythonManager::new()
            .install_requirements(dir)
            .map_err(|e| e.to_string())?;
        if ok {
            Ok("Python requirements installed.".to_string())
        } else {
            Err("Python requirements install failed.".to_string())
        }
    }

    pub fn python_start(req: &PythonAppRequest) -> Result<String, String> {
        if cfg!(target_os = "windows") {
            return Err("Python runtime start is not supported on Windows hosts.".to_string());
        }

        let ok = PythonManager::new()
            .start_gunicorn(&req.app_name, &req.wsgi_module, &req.dir, req.port)
            .map_err(|e| e.to_string())?;
        if ok {
            let mut guard = runtime_state().lock().map_err(|e| e.to_string())?;
            guard.apps.insert(
                req.app_name.clone(),
                RuntimeAppInfo {
                    app_name: req.app_name.clone(),
                    runtime: "python".to_string(),
                    dir: req.dir.clone(),
                    status: "running".to_string(),
                },
            );
            Ok("Python app started.".to_string())
        } else {
            Err("Python app start failed.".to_string())
        }
    }
}
