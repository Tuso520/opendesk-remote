CREATE TABLE IF NOT EXISTS login_logs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT NULL,
  email VARCHAR(255) NOT NULL,
  username VARCHAR(120),
  display_name VARCHAR(255),
  status ENUM('succeeded','failed','denied') NOT NULL,
  failure_reason VARCHAR(120),
  ip VARCHAR(64),
  user_agent TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id),
  INDEX idx_login_logs_email_created_at (email, created_at),
  INDEX idx_login_logs_status_created_at (status, created_at),
  INDEX idx_login_logs_created_at (created_at)
);
