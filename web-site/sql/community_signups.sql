CREATE DATABASE IF NOT EXISTS aurapanel_community
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

USE aurapanel_community;

CREATE TABLE IF NOT EXISTS community_signups (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  full_name VARCHAR(120) NOT NULL,
  email VARCHAR(190) NOT NULL,
  company VARCHAR(190) NOT NULL DEFAULT '',
  role VARCHAR(50) NOT NULL,
  focus TEXT NOT NULL,
  source_page VARCHAR(190) NOT NULL DEFAULT '',
  user_agent VARCHAR(255) NOT NULL DEFAULT '',
  ip_hash CHAR(64) NOT NULL,
  status VARCHAR(30) NOT NULL DEFAULT 'new',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uniq_email_role (email, role),
  KEY idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
