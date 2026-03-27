use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct ResellerQuota {
    pub username: String,
    pub plan: String,
    pub disk_gb: u32,
    pub bandwidth_gb: u32,
    pub max_sites: u32,
    pub updated_at: u64,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct WhiteLabelConfig {
    pub username: String,
    pub panel_name: String,
    pub logo_url: String,
    pub updated_at: u64,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct AclPolicy {
    pub id: String,
    pub name: String,
    pub description: String,
    pub permissions: Vec<String>,
    pub updated_at: u64,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct AclAssignment {
    pub username: String,
    pub policy_id: String,
    pub updated_at: u64,
}

#[derive(Serialize, Deserialize, Debug, Clone, Default)]
struct ResellerState {
    #[serde(default)]
    quotas: Vec<ResellerQuota>,
    #[serde(default)]
    white_labels: Vec<WhiteLabelConfig>,
    #[serde(default)]
    policies: Vec<AclPolicy>,
    #[serde(default)]
    assignments: Vec<AclAssignment>,
}

pub struct ResellerManager;

impl ResellerManager {
    pub fn list_quotas() -> Result<Vec<ResellerQuota>, String> {
        Ok(load_state()?.quotas)
    }

    pub fn upsert_quota(mut quota: ResellerQuota) -> Result<ResellerQuota, String> {
        quota.username = normalize_username(&quota.username)
            .ok_or_else(|| "valid username is required".to_string())?;
        if quota.plan.trim().is_empty() {
            return Err("plan is required".to_string());
        }
        quota.plan = quota.plan.trim().to_string();
        quota.updated_at = now_ts();

        let mut state = load_state()?;
        if let Some(existing) = state.quotas.iter_mut().find(|x| x.username == quota.username) {
            *existing = quota.clone();
        } else {
            state.quotas.push(quota.clone());
        }
        save_state(&state)?;
        Ok(quota)
    }

    pub fn list_white_labels() -> Result<Vec<WhiteLabelConfig>, String> {
        Ok(load_state()?.white_labels)
    }

    pub fn upsert_white_label(mut wl: WhiteLabelConfig) -> Result<WhiteLabelConfig, String> {
        wl.username = normalize_username(&wl.username)
            .ok_or_else(|| "valid username is required".to_string())?;
        if wl.panel_name.trim().is_empty() {
            return Err("panel_name is required".to_string());
        }
        wl.panel_name = wl.panel_name.trim().to_string();
        wl.logo_url = wl.logo_url.trim().to_string();
        wl.updated_at = now_ts();

        let mut state = load_state()?;
        if let Some(existing) = state.white_labels.iter_mut().find(|x| x.username == wl.username) {
            *existing = wl.clone();
        } else {
            state.white_labels.push(wl.clone());
        }
        save_state(&state)?;
        Ok(wl)
    }

    pub fn list_policies() -> Result<Vec<AclPolicy>, String> {
        Ok(load_state()?.policies)
    }

    pub fn upsert_policy(mut policy: AclPolicy) -> Result<AclPolicy, String> {
        if policy.name.trim().is_empty() {
            return Err("policy name is required".to_string());
        }

        policy.name = policy.name.trim().to_string();
        policy.description = policy.description.trim().to_string();
        policy.permissions = sanitize_permissions(&policy.permissions);
        if policy.permissions.is_empty() {
            return Err("at least one permission is required".to_string());
        }

        if policy.id.trim().is_empty() {
            policy.id = format!("pol_{}", short_hash(&format!("{}:{}", policy.name, now_ts())));
        }
        policy.updated_at = now_ts();

        let mut state = load_state()?;
        if let Some(existing) = state.policies.iter_mut().find(|x| x.id == policy.id) {
            *existing = policy.clone();
        } else {
            state.policies.push(policy.clone());
        }
        save_state(&state)?;
        Ok(policy)
    }

    pub fn delete_policy(id: &str) -> Result<(), String> {
        let id = id.trim();
        if id.is_empty() {
            return Err("policy id is required".to_string());
        }

        let mut state = load_state()?;
        let before = state.policies.len();
        state.policies.retain(|x| x.id != id);
        state.assignments.retain(|x| x.policy_id != id);

        if before == state.policies.len() {
            return Err("policy not found".to_string());
        }

        save_state(&state)
    }

    pub fn list_assignments() -> Result<Vec<AclAssignment>, String> {
        Ok(load_state()?.assignments)
    }

    pub fn assign_policy(mut assignment: AclAssignment) -> Result<AclAssignment, String> {
        assignment.username = normalize_username(&assignment.username)
            .ok_or_else(|| "valid username is required".to_string())?;
        assignment.policy_id = assignment.policy_id.trim().to_string();
        if assignment.policy_id.is_empty() {
            return Err("policy_id is required".to_string());
        }

        let mut state = load_state()?;
        if !state.policies.iter().any(|p| p.id == assignment.policy_id) {
            return Err("policy_id not found".to_string());
        }

        assignment.updated_at = now_ts();
        if let Some(existing) = state
            .assignments
            .iter_mut()
            .find(|x| x.username == assignment.username)
        {
            *existing = assignment.clone();
        } else {
            state.assignments.push(assignment.clone());
        }

        save_state(&state)?;
        Ok(assignment)
    }

    pub fn remove_assignment(username: &str) -> Result<(), String> {
        let username = normalize_username(username)
            .ok_or_else(|| "valid username is required".to_string())?;

        let mut state = load_state()?;
        let before = state.assignments.len();
        state.assignments.retain(|x| x.username != username);
        if before == state.assignments.len() {
            return Err("assignment not found".to_string());
        }
        save_state(&state)
    }

    pub fn effective_permissions(username: &str) -> Result<Vec<String>, String> {
        let username = normalize_username(username)
            .ok_or_else(|| "valid username is required".to_string())?;

        let state = load_state()?;
        let mut permission_set = BTreeSet::new();

        for assignment in state.assignments.iter().filter(|x| x.username == username) {
            if let Some(policy) = state.policies.iter().find(|p| p.id == assignment.policy_id) {
                for permission in &policy.permissions {
                    permission_set.insert(permission.to_string());
                }
            }
        }

        Ok(permission_set.into_iter().collect())
    }
}

fn state_root() -> PathBuf {
    if let Ok(path) = std::env::var("AURAPANEL_STATE_DIR") {
        let p = PathBuf::from(path.trim());
        if !p.as_os_str().is_empty() {
            return p;
        }
    }

    let prod = Path::new("/var/lib/aurapanel");
    if prod.exists() {
        prod.to_path_buf()
    } else {
        std::env::temp_dir().join("aurapanel")
    }
}

fn state_file() -> PathBuf {
    state_root().join("reseller_state.json")
}

fn load_state() -> Result<ResellerState, String> {
    let path = state_file();
    if !path.exists() {
        return Ok(ResellerState::default());
    }
    let raw = fs::read_to_string(path).map_err(|e| e.to_string())?;
    serde_json::from_str(&raw).map_err(|e| e.to_string())
}

fn save_state(state: &ResellerState) -> Result<(), String> {
    let path = state_file();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| e.to_string())?;
    }
    let payload = serde_json::to_string_pretty(state).map_err(|e| e.to_string())?;
    fs::write(path, payload).map_err(|e| e.to_string())
}

fn normalize_username(value: &str) -> Option<String> {
    let cleaned = value
        .trim()
        .to_ascii_lowercase()
        .chars()
        .filter(|c| c.is_ascii_alphanumeric() || *c == '_' || *c == '-')
        .collect::<String>();
    if cleaned.is_empty() {
        None
    } else {
        Some(cleaned)
    }
}

fn sanitize_permissions(values: &[String]) -> Vec<String> {
    let mut unique = BTreeMap::new();
    for item in values {
        let key = item.trim().to_ascii_lowercase();
        if !key.is_empty() {
            unique.insert(key.clone(), key);
        }
    }
    unique.into_values().collect()
}

fn short_hash(input: &str) -> String {
    use std::collections::hash_map::DefaultHasher;
    use std::hash::{Hash, Hasher};

    let mut hasher = DefaultHasher::new();
    input.hash(&mut hasher);
    format!("{:x}", hasher.finish())
}

fn now_ts() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0)
}
