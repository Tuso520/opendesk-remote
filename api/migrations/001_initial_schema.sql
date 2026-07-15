CREATE TABLE IF NOT EXISTS users (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  email VARCHAR(255) NOT NULL UNIQUE,
  username VARCHAR(120) NOT NULL UNIQUE,
  display_name VARCHAR(255) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  status ENUM('active','disabled','locked') NOT NULL DEFAULT 'active',
  mfa_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  source ENUM('local','oidc','ldap') NOT NULL DEFAULT 'local',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  last_login_at TIMESTAMP NULL
);

CREATE TABLE IF NOT EXISTS user_groups (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(255) NOT NULL UNIQUE,
  description TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_group_members (
  user_id BIGINT NOT NULL,
  group_id BIGINT NOT NULL,
  PRIMARY KEY (user_id, group_id),
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (group_id) REFERENCES user_groups(id)
);

CREATE TABLE IF NOT EXISTS devices (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  rustdesk_id VARCHAR(128) NOT NULL UNIQUE,
  name VARCHAR(255) NOT NULL,
  alias VARCHAR(255),
  owner_user_id BIGINT NULL,
  status ENUM('online','offline','disabled') NOT NULL DEFAULT 'offline',
  platform VARCHAR(64),
  client_version VARCHAR(64),
  opendesk_client_version VARCHAR(64),
  last_ip VARCHAR(64),
  last_seen_at TIMESTAMP NULL,
  registered_at TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  FOREIGN KEY (owner_user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS device_groups (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(255) NOT NULL UNIQUE,
  description TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS device_group_members (
  device_id BIGINT NOT NULL,
  group_id BIGINT NOT NULL,
  PRIMARY KEY (device_id, group_id),
  FOREIGN KEY (device_id) REFERENCES devices(id),
  FOREIGN KEY (group_id) REFERENCES device_groups(id)
);

CREATE TABLE IF NOT EXISTS api_tokens (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(255) NOT NULL,
  token_hash CHAR(64) NOT NULL UNIQUE,
  scopes JSON NOT NULL,
  user_id BIGINT NULL,
  expires_at TIMESTAMP NULL,
  last_used_at TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  revoked_at TIMESTAMP NULL,
  FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS sessions (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  token_hash CHAR(64) NOT NULL UNIQUE,
  ip VARCHAR(64),
  user_agent TEXT,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  revoked_at TIMESTAMP NULL,
  FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS relays (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(255) NOT NULL,
  region VARCHAR(120) NOT NULL,
  host VARCHAR(255) NOT NULL,
  port INT NOT NULL DEFAULT 21117,
  ws_port INT NOT NULL DEFAULT 21119,
  public_key_fingerprint VARCHAR(128),
  status ENUM('active','degraded','offline','disabled') NOT NULL DEFAULT 'active',
  max_bandwidth_mbps INT NULL,
  current_sessions INT NOT NULL DEFAULT 0,
  last_health_at TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS relay_grants (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  grant_id VARCHAR(128) NOT NULL UNIQUE,
  user_id BIGINT NULL,
  controller_device_id BIGINT NULL,
  target_device_id BIGINT NULL,
  target_rustdesk_id VARCHAR(128) NULL,
  relay_id BIGINT NULL,
  allowed_relays JSON NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  nonce VARCHAR(128) NOT NULL,
  status ENUM('issued','used','expired','revoked') NOT NULL DEFAULT 'issued',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  used_at TIMESTAMP NULL,
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (controller_device_id) REFERENCES devices(id),
  FOREIGN KEY (target_device_id) REFERENCES devices(id),
  FOREIGN KEY (relay_id) REFERENCES relays(id)
);

CREATE TABLE IF NOT EXISTS access_rules (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  subject_type ENUM('user','user_group') NOT NULL,
  subject_id BIGINT NOT NULL,
  target_type ENUM('device','device_group') NOT NULL,
  target_id BIGINT NOT NULL,
  effect ENUM('allow','deny') NOT NULL,
  priority INT NOT NULL DEFAULT 0,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_access_subject (subject_type, subject_id),
  INDEX idx_access_target (target_type, target_id)
);

CREATE TABLE IF NOT EXISTS control_roles (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(255) NOT NULL UNIQUE,
  description TEXT,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS control_role_permissions (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  role_id BIGINT NOT NULL,
  permission_key VARCHAR(120) NOT NULL,
  mode ENUM('use_client_settings','enable','disable') NOT NULL,
  FOREIGN KEY (role_id) REFERENCES control_roles(id)
);

CREATE TABLE IF NOT EXISTS strategies (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(255) NOT NULL UNIQUE,
  description TEXT,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  settings_json JSON NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS strategy_assignments (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  strategy_id BIGINT NOT NULL,
  target_type ENUM('device','user','device_group') NOT NULL,
  target_id BIGINT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (strategy_id) REFERENCES strategies(id)
);

CREATE TABLE IF NOT EXISTS audit_events (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  actor_user_id BIGINT NULL,
  actor_type ENUM('user','system','api_token') NOT NULL,
  action VARCHAR(160) NOT NULL,
  resource_type VARCHAR(160) NOT NULL,
  resource_id VARCHAR(160) NULL,
  ip VARCHAR(64),
  user_agent TEXT,
  metadata_json JSON,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_audit_action (action),
  INDEX idx_audit_resource (resource_type, resource_id),
  INDEX idx_audit_created (created_at)
);

CREATE TABLE IF NOT EXISTS connection_logs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  session_id BIGINT NULL,
  controller_user_id BIGINT NULL,
  controller_device_id BIGINT NULL,
  target_device_id BIGINT NULL,
  connection_type ENUM('direct','relay','websocket') NOT NULL,
  relay_id BIGINT NULL,
  started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ended_at TIMESTAMP NULL,
  status ENUM('started','ended','failed','denied') NOT NULL,
  deny_reason VARCHAR(255) NULL,
  metadata_json JSON
);

CREATE TABLE IF NOT EXISTS file_transfer_logs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  connection_log_id BIGINT NULL,
  direction ENUM('upload','download') NOT NULL,
  filename_hash CHAR(64) NOT NULL,
  size_bytes BIGINT NOT NULL,
  status VARCHAR(80) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (connection_log_id) REFERENCES connection_logs(id)
);

CREATE TABLE IF NOT EXISTS build_profiles (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(255) NOT NULL,
  app_name VARCHAR(255) NOT NULL,
  vendor VARCHAR(255),
  bundle_id VARCHAR(255) NOT NULL,
  product_name VARCHAR(255),
  description TEXT,
  server_config_json JSON NOT NULL,
  branding_json JSON NOT NULL,
  policy_json JSON NOT NULL,
  platforms_json JSON NOT NULL,
  signing_json JSON NOT NULL,
  created_by BIGINT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  FOREIGN KEY (created_by) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS build_jobs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  profile_id BIGINT NOT NULL,
  platform VARCHAR(64) NOT NULL,
  status ENUM('queued','running','succeeded','failed','canceled') NOT NULL DEFAULT 'queued',
  runner VARCHAR(120),
  log_path TEXT,
  error_message TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  started_at TIMESTAMP NULL,
  finished_at TIMESTAMP NULL,
  FOREIGN KEY (profile_id) REFERENCES build_profiles(id)
);

CREATE TABLE IF NOT EXISTS build_artifacts (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  build_job_id BIGINT NOT NULL,
  platform VARCHAR(64) NOT NULL,
  filename VARCHAR(255) NOT NULL,
  local_path TEXT NOT NULL,
  sha256 CHAR(64) NOT NULL,
  size_bytes BIGINT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (build_job_id) REFERENCES build_jobs(id)
);
