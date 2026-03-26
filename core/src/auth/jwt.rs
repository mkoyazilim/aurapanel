use jsonwebtoken::{encode, decode, Header, Validation, EncodingKey, DecodingKey};
use serde::{Deserialize, Serialize};
use std::time::{SystemTime, UNIX_EPOCH};

#[derive(Debug, Serialize, Deserialize)]
pub struct Claims {
    pub sub: String, // username or user_id
    pub role: String, // admin, reseller, user
    pub exp: usize,
}

const SECRET: &[u8] = b"aurapanel_super_secret_temporary_key_change_in_prod";

pub fn create_token(user_id: &str, role: &str) -> Result<String, jsonwebtoken::errors::Error> {
    let expiration = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("Time went backwards")
        .as_secs() + 24 * 3600; // 24 hours

    let claims = Claims {
        sub: user_id.to_string(),
        role: role.to_string(),
        exp: expiration as usize,
    };

    encode(&Header::default(), &claims, &EncodingKey::from_secret(SECRET))
}

pub fn verify_token(token: &str) -> Result<Claims, jsonwebtoken::errors::Error> {
    let validation = Validation::new(jsonwebtoken::Algorithm::HS256);
    let token_data = decode::<Claims>(token, &DecodingKey::from_secret(SECRET), &validation)?;
    Ok(token_data.claims)
}
