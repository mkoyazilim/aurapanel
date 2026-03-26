use serde::{Deserialize, Serialize};
use super::docker::DockerManager;

// ─── Docker App Şablonları ──────────────────────────────────

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct DockerAppTemplate {
    pub id: String,
    pub name: String,
    pub description: String,
    pub image: String,
    pub default_ports: Vec<String>,
    pub default_env: Vec<String>,
    pub default_volumes: Vec<String>,
    pub category: String,
    pub icon: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct DockerPackage {
    pub id: String,
    pub name: String,
    pub memory_limit: String,
    pub cpu_limit: String,
    pub max_containers: u32,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct CreateDockerAppRequest {
    pub template_id: String,
    pub app_name: String,
    pub package_id: Option<String>,
    pub custom_env: Vec<String>,
}

pub struct DockerAppsManager;

impl DockerAppsManager {
    /// Hazır Docker uygulama şablonlarını döndürür
    pub fn list_templates() -> Vec<DockerAppTemplate> {
        vec![
            DockerAppTemplate {
                id: "wordpress".into(),
                name: "WordPress".into(),
                description: "Popüler blog ve CMS platformu".into(),
                image: "wordpress:latest".into(),
                default_ports: vec!["8080:80".into()],
                default_env: vec![
                    "WORDPRESS_DB_HOST=mysql-db:3306".into(),
                    "WORDPRESS_DB_USER=wp_user".into(),
                    "WORDPRESS_DB_PASSWORD=wp_secret".into(),
                    "WORDPRESS_DB_NAME=wordpress".into(),
                ],
                default_volumes: vec!["/data/wordpress:/var/www/html".into()],
                category: "CMS".into(),
                icon: "📝".into(),
            },
            DockerAppTemplate {
                id: "mysql".into(),
                name: "MySQL / MariaDB".into(),
                description: "İlişkisel veritabanı sunucusu".into(),
                image: "mariadb:11".into(),
                default_ports: vec!["3306:3306".into()],
                default_env: vec![
                    "MYSQL_ROOT_PASSWORD=root_secret".into(),
                    "MYSQL_DATABASE=mydb".into(),
                ],
                default_volumes: vec!["/data/mysql:/var/lib/mysql".into()],
                category: "Database".into(),
                icon: "🗄️".into(),
            },
            DockerAppTemplate {
                id: "redis".into(),
                name: "Redis".into(),
                description: "Yüksek performanslı önbellek ve veri deposu".into(),
                image: "redis:7-alpine".into(),
                default_ports: vec!["6379:6379".into()],
                default_env: vec![],
                default_volumes: vec!["/data/redis:/data".into()],
                category: "Cache".into(),
                icon: "⚡".into(),
            },
            DockerAppTemplate {
                id: "mongodb".into(),
                name: "MongoDB".into(),
                description: "NoSQL belge tabanlı veritabanı".into(),
                image: "mongo:7".into(),
                default_ports: vec!["27017:27017".into()],
                default_env: vec![
                    "MONGO_INITDB_ROOT_USERNAME=admin".into(),
                    "MONGO_INITDB_ROOT_PASSWORD=mongo_secret".into(),
                ],
                default_volumes: vec!["/data/mongo:/data/db".into()],
                category: "Database".into(),
                icon: "🍃".into(),
            },
            DockerAppTemplate {
                id: "phpmyadmin".into(),
                name: "phpMyAdmin".into(),
                description: "Web tabanlı MySQL yönetim aracı".into(),
                image: "phpmyadmin:latest".into(),
                default_ports: vec!["8081:80".into()],
                default_env: vec![
                    "PMA_HOST=mysql-db".into(),
                    "PMA_PORT=3306".into(),
                ],
                default_volumes: vec![],
                category: "Tool".into(),
                icon: "🔧".into(),
            },
            DockerAppTemplate {
                id: "nginx".into(),
                name: "Nginx".into(),
                description: "Yüksek performanslı web sunucusu / reverse proxy".into(),
                image: "nginx:alpine".into(),
                default_ports: vec!["80:80".into(), "443:443".into()],
                default_env: vec![],
                default_volumes: vec!["/data/nginx/html:/usr/share/nginx/html".into()],
                category: "Web Server".into(),
                icon: "🌐".into(),
            },
            DockerAppTemplate {
                id: "nodejs".into(),
                name: "Node.js".into(),
                description: "Node.js uygulama ortamı".into(),
                image: "node:20-alpine".into(),
                default_ports: vec!["3000:3000".into()],
                default_env: vec!["NODE_ENV=production".into()],
                default_volumes: vec!["/data/nodeapp:/app".into()],
                category: "Runtime".into(),
                icon: "💚".into(),
            },
            DockerAppTemplate {
                id: "postgres".into(),
                name: "PostgreSQL".into(),
                description: "Gelişmiş açık kaynak ilişkisel veritabanı".into(),
                image: "postgres:16-alpine".into(),
                default_ports: vec!["5432:5432".into()],
                default_env: vec![
                    "POSTGRES_PASSWORD=pg_secret".into(),
                    "POSTGRES_DB=mydb".into(),
                ],
                default_volumes: vec!["/data/postgres:/var/lib/postgresql/data".into()],
                category: "Database".into(),
                icon: "🐘".into(),
            },
        ]
    }

    /// Kaynak limiti paketlerini döndürür
    pub fn list_packages() -> Vec<DockerPackage> {
        vec![
            DockerPackage { id: "starter".into(), name: "Starter".into(), memory_limit: "256m".into(), cpu_limit: "0.5".into(), max_containers: 3 },
            DockerPackage { id: "pro".into(), name: "Professional".into(), memory_limit: "1g".into(), cpu_limit: "1.0".into(), max_containers: 10 },
            DockerPackage { id: "enterprise".into(), name: "Enterprise".into(), memory_limit: "4g".into(), cpu_limit: "2.0".into(), max_containers: 50 },
        ]
    }

    /// Şablondan Docker uygulaması oluşturur
    pub fn create_app(req: &CreateDockerAppRequest) -> Result<String, String> {
        let templates = Self::list_templates();
        let template = templates.iter()
            .find(|t| t.id == req.template_id)
            .ok_or_else(|| format!("Şablon bulunamadı: {}", req.template_id))?;

        // Paketten kaynak limiti al
        let packages = Self::list_packages();
        let package = req.package_id.as_ref()
            .and_then(|pid| packages.iter().find(|p| p.id == *pid));

        // Ortam değişkenlerini birleştir
        let mut env = template.default_env.clone();
        env.extend(req.custom_env.clone());

        let config = super::docker::CreateContainerConfig {
            name: req.app_name.clone(),
            image: template.image.clone(),
            ports: template.default_ports.clone(),
            env,
            volumes: template.default_volumes.clone(),
            restart_policy: Some("unless-stopped".to_string()),
            memory_limit: package.map(|p| p.memory_limit.clone()),
            cpu_limit: package.map(|p| p.cpu_limit.clone()),
        };

        DockerManager::create_container(&config)
    }
}
