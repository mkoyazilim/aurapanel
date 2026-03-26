use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Debug)]
pub struct WireguardConfig {
    pub node_name: String,
    pub ip_address: String,
    pub pub_key: String,
}

pub struct FederatedManager;

impl FederatedManager {
    /// Başka bir AuraPanel sunucusunu mevcut kümeye bağlar (VPN Tüneli açar)
    pub async fn add_cluster_node(config: &WireguardConfig) -> Result<(), String> {
        println!("[DEV MODE] Adding federated node {} at IP {} to WireGuard mesh.", config.node_name, config.ip_address);

        /*
        // Gerçekte wg komutları çalıştırılır veya wg-quick up wg0 gibi işlemler yapılır.
        // Command::new("wg").args(["set", "wg0", "peer", &config.pub_key, "allowed-ips", &config.ip_address, "endpoint", &format!("{}:51820", config.ip_address)]).output();
        */

        Ok(())
    }
}
