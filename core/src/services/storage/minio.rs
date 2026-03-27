use anyhow::Result;
use std::collections::hash_map::DefaultHasher;
use std::hash::{Hash, Hasher};
use std::time::{SystemTime, UNIX_EPOCH};

pub struct MinioManager;

impl MinioManager {
    pub fn new() -> Self {
        Self {}
    }

    pub fn create_bucket(&self, bucket_name: &str) -> Result<bool> {
        println!("Creating MinIO bucket: {}", bucket_name);
        // Integrate with minio-rs or reqwest to call MinIO API
        Ok(true)
    }

    pub fn generate_credentials(&self, user: &str) -> Result<(String, String)> {
        let username = user.trim().to_ascii_lowercase();
        if username.is_empty() {
            anyhow::bail!("user is required");
        }

        let access_key = format!("ak_{}", random_hex_for(&username, 8));
        let secret_key = random_hex_for(&format!("{}_secret", username), 24);
        Ok((access_key, secret_key))
    }
}

fn random_hex_for(seed_key: &str, bytes: usize) -> String {
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos();
    let mut out = String::new();
    let mut counter = 0_u64;
    while out.len() < bytes * 2 {
        let mut hasher = DefaultHasher::new();
        seed_key.hash(&mut hasher);
        nanos.hash(&mut hasher);
        counter.hash(&mut hasher);
        let chunk = hasher.finish();
        out.push_str(&format!("{:016x}", chunk));
        counter += 1;
    }
    out.chars().take(bytes * 2).collect()
}
