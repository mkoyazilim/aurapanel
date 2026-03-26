use totp_rs::{Algorithm, TOTP, Secret};
use anyhow::Result;

pub fn generate_totp_secret(account_name: &str) -> Result<(String, String)> {
    let secret = Secret::generate_secret();
    let totp = TOTP::new(
        Algorithm::SHA1,
        6,
        1,
        30,
        secret.to_bytes().unwrap(),
        Some("AuraPanel".to_string()),
        account_name.to_string(),
    )?;

    let qr_code = totp.get_qr_base64()?;
    let secret_str = secret.to_encoded().to_string();

    Ok((secret_str, qr_code))
}

pub fn verify_totp(secret_str: &str, token: &str) -> Result<bool> {
    let secret = Secret::Encoded(secret_str.to_string());
    let totp = TOTP::new(
        Algorithm::SHA1,
        6,
        1,
        30,
        secret.to_bytes().unwrap(),
        Some("AuraPanel".to_string()),
        "".to_string(), // Account name is not needed for verification
    )?;
    
    Ok(totp.check_current(token)?)
}
