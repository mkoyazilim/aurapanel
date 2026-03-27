use flate2::read::GzDecoder;
use serde::{Deserialize, Serialize};
use std::collections::{BTreeSet, HashMap, HashSet};
use std::fs;
use std::io::{BufReader, Read};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{SystemTime, UNIX_EPOCH};
use tar::Archive;

use crate::services::nitro::{NitroEngine, VHostConfig};

static MIGRATION_COUNTER: AtomicU64 = AtomicU64::new(1);

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum MigrationSource {
    Cpanel,
    Cyberpanel,
    Unknown,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MigrationAnalyzeRequest {
    pub archive_path: String,
    #[serde(default)]
    pub source_type: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MigrationImportRequest {
    pub archive_path: String,
    #[serde(default)]
    pub source_type: Option<String>,
    #[serde(default)]
    pub target_owner: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MigrationAnalyzeResult {
    pub analysis_id: String,
    pub archive_path: String,
    pub source_type: MigrationSource,
    pub extracted_path: String,
    pub account: String,
    pub stats: MigrationStats,
    pub mysql_dumps: Vec<String>,
    pub email_accounts: Vec<String>,
    pub vhost_candidates: Vec<String>,
    pub warnings: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MigrationStats {
    pub file_count: usize,
    pub database_count: usize,
    pub email_count: usize,
    pub vhost_count: usize,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum MigrationJobState {
    Queued,
    Running,
    Completed,
    Failed,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MigrationJob {
    pub id: String,
    pub status: MigrationJobState,
    pub progress: u8,
    pub source_type: MigrationSource,
    pub archive_path: String,
    pub started_at: u64,
    pub finished_at: Option<u64>,
    pub summary: Option<MigrationImportSummary>,
    pub warnings: Vec<String>,
    pub logs: Vec<String>,
    pub error: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MigrationImportSummary {
    pub analysis: MigrationAnalyzeResult,
    pub converted_db_files: Vec<String>,
    pub email_plan_file: String,
    pub vhost_plan_file: String,
    pub system_apply_enabled: bool,
}

pub struct MigrationManager;

impl MigrationManager {
    pub fn upload_dir() -> PathBuf {
        state_root().join("migrations").join("uploads")
    }

    pub fn analyze_backup(req: &MigrationAnalyzeRequest) -> Result<MigrationAnalyzeResult, String> {
        let archive_path = normalize_existing_path(&req.archive_path)?;
        ensure_archive_format(&archive_path)?;

        let analysis_id = next_id("analysis");
        let extracted_path = analysis_root().join(&analysis_id);
        fs::create_dir_all(&extracted_path)
            .map_err(|e| format!("Analiz dizini olusturulamadi: {}", e))?;

        extract_tar_gz(&archive_path, &extracted_path)?;
        let scanned = scan_extracted_tree(&extracted_path)?;
        let requested_source = parse_source(req.source_type.as_deref());
        let detected_source = detect_source(&scanned);
        let source_type = if requested_source == MigrationSource::Unknown {
            detected_source
        } else {
            requested_source
        };

        let account = guess_account_name(&archive_path, &scanned);
        let mysql_dumps = discover_mysql_dumps(&source_type, &scanned);
        let email_accounts = discover_email_accounts(&source_type, &scanned);
        let vhost_candidates = discover_vhosts(&source_type, &scanned);

        let mut warnings = Vec::new();
        if source_type == MigrationSource::Unknown {
            warnings.push(
                "Kaynak panel tipi otomatik tespit edilemedi, genel parser kullanildi.".to_string(),
            );
        }
        if mysql_dumps.is_empty() {
            warnings.push("MySQL dump dosyasi bulunamadi.".to_string());
        }
        if vhost_candidates.is_empty() {
            warnings.push("VHost adayi bulunamadi.".to_string());
        }

        Ok(MigrationAnalyzeResult {
            analysis_id,
            archive_path: archive_path.to_string_lossy().to_string(),
            source_type,
            extracted_path: extracted_path.to_string_lossy().to_string(),
            account,
            stats: MigrationStats {
                file_count: scanned.files.len(),
                database_count: mysql_dumps.len(),
                email_count: email_accounts.len(),
                vhost_count: vhost_candidates.len(),
            },
            mysql_dumps,
            email_accounts,
            vhost_candidates,
            warnings,
        })
    }

    pub async fn start_import(req: MigrationImportRequest) -> Result<MigrationJob, String> {
        let archive_path = normalize_existing_path(&req.archive_path)?;
        let requested_source = parse_source(req.source_type.as_deref());
        let id = next_id("job");
        let now = now_ts();

        let initial = MigrationJob {
            id: id.clone(),
            status: MigrationJobState::Queued,
            progress: 0,
            source_type: requested_source,
            archive_path: archive_path.to_string_lossy().to_string(),
            started_at: now,
            finished_at: None,
            summary: None,
            warnings: Vec::new(),
            logs: vec!["Import kuyruga alindi.".to_string()],
            error: None,
        };
        save_job(&initial)?;

        let owner = req.target_owner.unwrap_or_else(|| "aura".to_string());
        let archive_str = archive_path.to_string_lossy().to_string();
        let source_opt = req.source_type.clone();
        let id_for_task = id.clone();

        tokio::spawn(async move {
            let id_for_worker = id_for_task.clone();
            let task_result = tokio::task::spawn_blocking(move || {
                Self::run_import_job(&id_for_worker, &archive_str, source_opt.as_deref(), &owner)
            })
            .await;

            if let Err(join_err) = task_result {
                let _ = update_job(
                    &id_for_task,
                    Some(MigrationJobState::Failed),
                    Some(100),
                    Some(format!("Migration gorevi panic oldu: {}", join_err)),
                    None,
                    Some("Background gorev beklenmedik sekilde sonlandi.".to_string()),
                );
            }
        });

        Ok(initial)
    }

    pub fn get_import_job(id: &str) -> Result<MigrationJob, String> {
        let path = jobs_dir().join(format!("{}.json", sanitize_id(id)));
        let raw = fs::read_to_string(&path).map_err(|e| format!("Job okunamadi: {}", e))?;
        serde_json::from_str(&raw).map_err(|e| format!("Job parse edilemedi: {}", e))
    }

    fn run_import_job(
        job_id: &str,
        archive_path: &str,
        source_type: Option<&str>,
        target_owner: &str,
    ) -> Result<(), String> {
        update_job(
            job_id,
            Some(MigrationJobState::Running),
            Some(5),
            None,
            None,
            Some("Backup analizi basladi.".to_string()),
        )?;

        let analysis = Self::analyze_backup(&MigrationAnalyzeRequest {
            archive_path: archive_path.to_string(),
            source_type: source_type.map(|s| s.to_string()),
        })?;

        update_job(
            job_id,
            None,
            Some(20),
            None,
            None,
            Some(format!(
                "Kaynak tespit edildi: {:?}, {} dosya tarandi.",
                analysis.source_type, analysis.stats.file_count
            )),
        )?;

        let out_dir = state_root().join("migrations").join("imports").join(job_id);
        let db_dir = out_dir.join("databases");
        fs::create_dir_all(&db_dir)
            .map_err(|e| format!("DB cikti dizini olusturulamadi: {}", e))?;

        let mut converted_db_files = Vec::new();
        for (idx, dump_rel_path) in analysis.mysql_dumps.iter().enumerate() {
            let absolute_dump = Path::new(&analysis.extracted_path).join(dump_rel_path);
            if !absolute_dump.exists() {
                continue;
            }

            let logical_name = derive_db_name(&absolute_dump);
            let output_file = db_dir.join(format!("{}.sql", logical_name));
            convert_dump_to_plain_sql(&absolute_dump, &output_file)?;
            converted_db_files.push(output_file.to_string_lossy().to_string());

            let progress =
                20u8.saturating_add((((idx + 1) * 30) / analysis.mysql_dumps.len().max(1)) as u8);
            update_job(
                job_id,
                None,
                Some(progress.min(55)),
                None,
                None,
                Some(format!("DB donusumu tamamlandi: {}", dump_rel_path)),
            )?;
        }

        let email_plan = build_email_plan(&analysis.email_accounts);
        let email_plan_file = out_dir.join("mail_import_plan.json");
        write_json_file(&email_plan_file, &email_plan)?;
        update_job(
            job_id,
            None,
            Some(70),
            None,
            None,
            Some(format!(
                "E-posta plani uretildi ({} hesap).",
                analysis.email_accounts.len()
            )),
        )?;

        let vhost_plan = build_vhost_plan(&analysis.vhost_candidates, target_owner);
        let vhost_plan_file = out_dir.join("vhost_import_plan.json");
        write_json_file(&vhost_plan_file, &vhost_plan)?;
        update_job(
            job_id,
            None,
            Some(82),
            None,
            None,
            Some(format!(
                "VHost plani uretildi ({} site).",
                analysis.vhost_candidates.len()
            )),
        )?;

        let apply_system_import = std::env::var("AURAPANEL_MIGRATION_APPLY_SYSTEM_IMPORT")
            .map(|v| matches!(v.to_ascii_lowercase().as_str(), "1" | "true" | "yes"))
            .unwrap_or(false);

        let mut warnings = analysis.warnings.clone();
        if apply_system_import {
            warnings.push(
                "Mailbox otomatik importu bu surumde plan ciktisi olarak uretilir. mail_import_plan.json dosyasini MailManager ile uygulayin."
                    .to_string(),
            );

            for domain in &analysis.vhost_candidates {
                if let Err(err) = NitroEngine::create_vhost(&VHostConfig {
                    domain: domain.clone(),
                    user: target_owner.to_string(),
                    php_version: "8.3".to_string(),
                }) {
                    warnings.push(format!("VHost import atlandi ({}): {}", domain, err));
                }
            }
        } else {
            warnings.push(
                "Sistem importu dry-run modunda. Gercek import icin AURAPANEL_MIGRATION_APPLY_SYSTEM_IMPORT=true ayarlayin."
                    .to_string(),
            );
        }

        let summary = MigrationImportSummary {
            analysis,
            converted_db_files,
            email_plan_file: email_plan_file.to_string_lossy().to_string(),
            vhost_plan_file: vhost_plan_file.to_string_lossy().to_string(),
            system_apply_enabled: apply_system_import,
        };

        update_job(
            job_id,
            Some(MigrationJobState::Completed),
            Some(100),
            None,
            Some(summary),
            Some("Migration import tamamlandi.".to_string()),
        )?;

        if !warnings.is_empty() {
            update_job_warnings(job_id, warnings)?;
        }

        Ok(())
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct EmailPlanEntry {
    address: String,
    username: String,
    domain: String,
}

#[derive(Debug, Clone)]
struct ScannedTree {
    root: PathBuf,
    files: Vec<PathBuf>,
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

fn analysis_root() -> PathBuf {
    #[cfg(test)]
    {
        std::env::temp_dir()
            .join("aurapanel")
            .join("migrations")
            .join("analysis")
    }
    #[cfg(not(test))]
    {
        state_root().join("migrations").join("analysis")
    }
}

fn now_ts() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0)
}

fn next_id(prefix: &str) -> String {
    let seq = MIGRATION_COUNTER.fetch_add(1, Ordering::Relaxed);
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_nanos())
        .unwrap_or(0);
    format!("{}-{}-{}", prefix, nanos, seq)
}

fn sanitize_id(value: &str) -> String {
    value
        .chars()
        .filter(|c| c.is_ascii_alphanumeric() || *c == '-' || *c == '_')
        .collect::<String>()
}

fn jobs_dir() -> PathBuf {
    state_root().join("migrations").join("jobs")
}

fn job_file(job_id: &str) -> PathBuf {
    jobs_dir().join(format!("{}.json", sanitize_id(job_id)))
}

fn save_job(job: &MigrationJob) -> Result<(), String> {
    let path = job_file(&job.id);
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| format!("Job dizini olusturulamadi: {}", e))?;
    }
    let json =
        serde_json::to_string_pretty(job).map_err(|e| format!("Job serialize hatasi: {}", e))?;
    fs::write(path, json).map_err(|e| format!("Job yazilamadi: {}", e))
}

fn update_job(
    job_id: &str,
    status: Option<MigrationJobState>,
    progress: Option<u8>,
    error: Option<String>,
    summary: Option<MigrationImportSummary>,
    log_line: Option<String>,
) -> Result<(), String> {
    let mut job = MigrationManager::get_import_job(job_id)?;
    if let Some(state) = status {
        job.status = state;
    }
    if let Some(value) = progress {
        job.progress = value.min(100);
    }
    if let Some(msg) = error {
        job.error = Some(msg);
    }
    if let Some(payload) = summary {
        job.summary = Some(payload);
    }
    if let Some(line) = log_line {
        job.logs.push(line);
    }
    if matches!(
        job.status,
        MigrationJobState::Completed | MigrationJobState::Failed
    ) {
        job.finished_at = Some(now_ts());
    }
    save_job(&job)
}

fn update_job_warnings(job_id: &str, warnings: Vec<String>) -> Result<(), String> {
    let mut job = MigrationManager::get_import_job(job_id)?;
    job.warnings.extend(warnings);
    save_job(&job)
}

fn normalize_existing_path(raw: &str) -> Result<PathBuf, String> {
    let path = PathBuf::from(raw.trim());
    if path.as_os_str().is_empty() {
        return Err("Dosya yolu zorunludur.".to_string());
    }
    if !path.exists() {
        return Err(format!("Backup dosyasi bulunamadi: {}", path.display()));
    }
    fs::canonicalize(path).map_err(|e| format!("Backup yolu dogrulanamadi: {}", e))
}

fn ensure_archive_format(path: &Path) -> Result<(), String> {
    let name = path
        .file_name()
        .and_then(|v| v.to_str())
        .unwrap_or_default()
        .to_ascii_lowercase();
    if name.ends_with(".tar.gz") || name.ends_with(".tgz") {
        Ok(())
    } else {
        Err("Desteklenen backup formati: .tar.gz veya .tgz".to_string())
    }
}

fn extract_tar_gz(archive: &Path, target_dir: &Path) -> Result<(), String> {
    let file = fs::File::open(archive).map_err(|e| format!("Archive acilamadi: {}", e))?;
    let reader = BufReader::new(file);
    let decoder = GzDecoder::new(reader);
    let mut tar = Archive::new(decoder);
    tar.unpack(target_dir)
        .map_err(|e| format!("Archive acilamadi: {}", e))
}

fn scan_extracted_tree(root: &Path) -> Result<ScannedTree, String> {
    let mut files = Vec::new();
    let mut stack = vec![root.to_path_buf()];

    while let Some(dir) = stack.pop() {
        let entries = fs::read_dir(&dir)
            .map_err(|e| format!("Extracted tree okunamadi ({}): {}", dir.display(), e))?;
        for entry in entries.flatten() {
            let path = entry.path();
            if path.is_dir() {
                stack.push(path);
            } else {
                files.push(path);
            }
        }
    }
    Ok(ScannedTree {
        root: root.to_path_buf(),
        files,
    })
}

fn parse_source(value: Option<&str>) -> MigrationSource {
    match value.unwrap_or("auto").trim().to_ascii_lowercase().as_str() {
        "cpanel" => MigrationSource::Cpanel,
        "cyberpanel" => MigrationSource::Cyberpanel,
        _ => MigrationSource::Unknown,
    }
}

fn detect_source(tree: &ScannedTree) -> MigrationSource {
    let mut cpanel_hits = 0usize;
    let mut cyber_hits = 0usize;
    for file in &tree.files {
        let path = file.to_string_lossy().to_ascii_lowercase();
        if path.contains("cpmove")
            || path.contains("/homedir/")
            || path.contains("/mysql/")
            || path.contains("/userdata/")
        {
            cpanel_hits += 1;
        }
        if path.contains("/cyberpanel/")
            || path.contains("/backupdata/")
            || path.contains("/websiteconfig/")
            || path.contains("/vhosts/")
        {
            cyber_hits += 1;
        }
    }
    if cpanel_hits > cyber_hits && cpanel_hits > 0 {
        MigrationSource::Cpanel
    } else if cyber_hits > cpanel_hits && cyber_hits > 0 {
        MigrationSource::Cyberpanel
    } else {
        MigrationSource::Unknown
    }
}

fn guess_account_name(archive_path: &Path, tree: &ScannedTree) -> String {
    let archive_name = archive_path
        .file_stem()
        .and_then(|v| v.to_str())
        .unwrap_or("migrated-account")
        .to_ascii_lowercase()
        .replace(".tar", "");

    if archive_name.starts_with("cpmove-") {
        return archive_name.trim_start_matches("cpmove-").to_string();
    }

    for file in &tree.files {
        if let Some(name) = file.file_name().and_then(|v| v.to_str()) {
            if name.eq_ignore_ascii_case("cpanel.config") {
                return archive_name.clone();
            }
        }
    }
    archive_name
}

fn to_relative(root: &Path, path: &Path) -> String {
    path.strip_prefix(root)
        .unwrap_or(path)
        .to_string_lossy()
        .replace('\\', "/")
}

fn discover_mysql_dumps(source: &MigrationSource, tree: &ScannedTree) -> Vec<String> {
    let mut results = Vec::new();

    for file in &tree.files {
        let rel = to_relative(&tree.root, file);
        let rel_lower = rel.to_ascii_lowercase();
        let is_dump = rel_lower.ends_with(".sql") || rel_lower.ends_with(".sql.gz");
        if !is_dump {
            continue;
        }
        match source {
            MigrationSource::Cpanel => {
                if rel_lower.contains("/mysql/") || rel_lower.contains("/mysql.sql") {
                    results.push(rel);
                }
            }
            MigrationSource::Cyberpanel => {
                if rel_lower.contains("/databases/")
                    || rel_lower.contains("/db/")
                    || rel_lower.contains("/mysql/")
                {
                    results.push(rel);
                }
            }
            MigrationSource::Unknown => results.push(rel),
        }
    }
    results.sort();
    results.dedup();
    results
}

fn discover_email_accounts(source: &MigrationSource, tree: &ScannedTree) -> Vec<String> {
    let mut accounts = BTreeSet::new();
    for file in &tree.files {
        let rel = file.to_string_lossy().replace('\\', "/");
        let lower = rel.to_ascii_lowercase();

        if lower.contains("/mail/") || lower.contains("/email/") {
            let parts: Vec<&str> = rel.split('/').collect();
            for idx in 0..parts.len().saturating_sub(2) {
                let domain = parts[idx + 1];
                let user = parts[idx + 2];
                if looks_like_domain(domain) && looks_like_local_part(user) {
                    accounts.insert(format!(
                        "{}@{}",
                        user.to_ascii_lowercase(),
                        domain.to_ascii_lowercase()
                    ));
                }
            }
        }

        if matches!(source, MigrationSource::Cyberpanel) && lower.ends_with("mailusers.json") {
            if let Ok(raw) = fs::read_to_string(file) {
                for token in raw.split('"') {
                    if token.contains('@') && token.contains('.') {
                        accounts.insert(token.to_ascii_lowercase());
                    }
                }
            }
        }
    }
    accounts.into_iter().collect()
}

fn discover_vhosts(source: &MigrationSource, tree: &ScannedTree) -> Vec<String> {
    let mut domains = BTreeSet::new();
    for file in &tree.files {
        let rel = file.to_string_lossy().replace('\\', "/");
        let lower = rel.to_ascii_lowercase();
        if lower.contains("/userdata/")
            || lower.contains("/vhosts/")
            || lower.contains("/public_html/")
        {
            for segment in rel.split('/') {
                if looks_like_domain(segment) {
                    domains.insert(segment.to_ascii_lowercase());
                }
            }
        }
        if matches!(source, MigrationSource::Cyberpanel)
            && lower.ends_with(".conf")
            && lower.contains("vhost")
        {
            for segment in rel.split('/') {
                if looks_like_domain(segment) {
                    domains.insert(segment.to_ascii_lowercase());
                }
            }
        }
    }
    domains.into_iter().collect()
}

fn looks_like_domain(value: &str) -> bool {
    let v = value.trim().to_ascii_lowercase();
    v.contains('.')
        && v.len() > 3
        && v.chars()
            .all(|c| c.is_ascii_alphanumeric() || c == '.' || c == '-')
}

fn looks_like_local_part(value: &str) -> bool {
    let v = value.trim();
    !v.is_empty()
        && v.chars()
            .all(|c| c.is_ascii_alphanumeric() || c == '.' || c == '_' || c == '-')
}

fn derive_db_name(path: &Path) -> String {
    let name = path
        .file_name()
        .and_then(|v| v.to_str())
        .unwrap_or("database.sql")
        .to_ascii_lowercase();
    let cleaned = name
        .trim_end_matches(".gz")
        .trim_end_matches(".sql")
        .chars()
        .filter(|c| c.is_ascii_alphanumeric() || *c == '_')
        .collect::<String>();
    if cleaned.is_empty() {
        format!("db_{}", now_ts())
    } else {
        cleaned
    }
}

fn convert_dump_to_plain_sql(input: &Path, output: &Path) -> Result<(), String> {
    let mut content = Vec::new();
    let input_name = input
        .file_name()
        .and_then(|v| v.to_str())
        .unwrap_or_default()
        .to_ascii_lowercase();
    if input_name.ends_with(".gz") {
        let file = fs::File::open(input).map_err(|e| format!("Dump acilamadi: {}", e))?;
        let mut gz = GzDecoder::new(file);
        gz.read_to_end(&mut content)
            .map_err(|e| format!("Gzip dump acilamadi: {}", e))?;
    } else {
        content = fs::read(input).map_err(|e| format!("Dump okunamadi: {}", e))?;
    }

    if let Some(parent) = output.parent() {
        fs::create_dir_all(parent).map_err(|e| format!("DB cikti dizini olusturulamadi: {}", e))?;
    }
    fs::write(output, content).map_err(|e| format!("DB cikti yazilamadi: {}", e))
}

fn build_email_plan(email_accounts: &[String]) -> Vec<EmailPlanEntry> {
    let mut dedup = HashSet::new();
    let mut plan = Vec::new();
    for address in email_accounts {
        let addr = address.trim().to_ascii_lowercase();
        if !addr.contains('@') || !dedup.insert(addr.clone()) {
            continue;
        }
        let mut parts = addr.split('@');
        let username = parts.next().unwrap_or_default().to_string();
        let domain = parts.next().unwrap_or_default().to_string();
        if username.is_empty() || domain.is_empty() {
            continue;
        }
        plan.push(EmailPlanEntry {
            address: addr,
            username,
            domain,
        });
    }
    plan
}

fn build_vhost_plan(vhosts: &[String], owner: &str) -> Vec<HashMap<&'static str, String>> {
    let mut out = Vec::new();
    for domain in vhosts {
        let mut item = HashMap::new();
        item.insert("domain", domain.clone());
        item.insert("owner", owner.to_string());
        item.insert("php_version", "8.3".to_string());
        out.push(item);
    }
    out
}

fn write_json_file<T: Serialize>(path: &Path, value: &T) -> Result<(), String> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| format!("Dizin olusturulamadi: {}", e))?;
    }
    let body =
        serde_json::to_string_pretty(value).map_err(|e| format!("JSON serialize hatasi: {}", e))?;
    fs::write(path, body).map_err(|e| format!("JSON yazilamadi: {}", e))
}

#[cfg(test)]
mod tests {
    use super::*;
    use flate2::write::GzEncoder;
    use flate2::Compression;
    use std::io::Write;
    use tar::Builder;

    #[test]
    fn analyze_detects_cpanel_artifacts() {
        let root = std::env::temp_dir().join(next_id("aurapanel-migration-test"));
        let source = root.join("source");
        let mysql_dir = source.join("mysql");
        let mail_dir = source
            .join("homedir")
            .join("mail")
            .join("example.com")
            .join("admin");
        let userdata_dir = source.join("userdata").join("example.com");
        fs::create_dir_all(&mysql_dir).expect("mysql dir");
        fs::create_dir_all(&mail_dir).expect("mail dir");
        fs::create_dir_all(&userdata_dir).expect("userdata dir");
        fs::write(mysql_dir.join("app.sql"), "CREATE TABLE t(id INT);").expect("dump");
        fs::write(mail_dir.join("quota"), "1024").expect("mail marker");
        fs::write(userdata_dir.join("main"), "domain: example.com").expect("vhost marker");

        let archive_path = root.join("cpmove-demo.tar.gz");
        let archive_file = fs::File::create(&archive_path).expect("archive create");
        let encoder = GzEncoder::new(archive_file, Compression::default());
        let mut builder = Builder::new(encoder);
        builder
            .append_dir_all("cpmove-demo", &source)
            .expect("append");
        let encoder = builder.into_inner().expect("builder finalize");
        encoder.finish().expect("gzip finalize");

        let analysis = MigrationManager::analyze_backup(&MigrationAnalyzeRequest {
            archive_path: archive_path.to_string_lossy().to_string(),
            source_type: None,
        })
        .expect("analyze");

        assert!(matches!(
            analysis.source_type,
            MigrationSource::Cpanel | MigrationSource::Unknown
        ));
        assert!(!analysis.mysql_dumps.is_empty());
        assert!(!analysis.email_accounts.is_empty());
        assert!(!analysis.vhost_candidates.is_empty());

        let _ = fs::remove_dir_all(&root);
    }
}
