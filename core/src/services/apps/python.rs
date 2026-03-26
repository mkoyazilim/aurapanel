use anyhow::Result;
use std::process::Command;
use std::fs;
use std::path::Path;

pub struct PythonManager;

impl PythonManager {
    pub fn new() -> Self {
        Self {}
    }

    pub fn create_venv(&self, dir: &str) -> Result<bool> {
        let output = Command::new("python3")
            .arg("-m")
            .arg("venv")
            .arg("venv")
            .current_dir(dir)
            .output()?;
        Ok(output.status.success())
    }

    pub fn install_requirements(&self, dir: &str) -> Result<bool> {
        let pip_path = Path::new(dir).join("venv/bin/pip");
        let req_path = Path::new(dir).join("requirements.txt");
        
        if req_path.exists() {
            let output = Command::new(pip_path)
                .arg("install")
                .arg("-r")
                .arg("requirements.txt")
                .current_dir(dir)
                .output()?;
            Ok(output.status.success())
        } else {
            Ok(true) // no requirements
        }
    }
    
    pub fn start_gunicorn(&self, app_name: &str, wsgi_module: &str, dir: &str, port: u16) -> Result<bool> {
        let gunicorn_cmd = format!("gunicorn {} -b 127.0.0.1:{}", wsgi_module, port);
        
        let output = Command::new("pm2")
            .arg("start")
            .arg(gunicorn_cmd)
            .arg("--name")
            .arg(app_name)
            .current_dir(dir)
            .output()?;
        
        Ok(output.status.success())
    }
}
