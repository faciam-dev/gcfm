CREATE TABLE IF NOT EXISTS gcfm_users (
  id BIGSERIAL PRIMARY KEY,
  username VARCHAR(64) UNIQUE NOT NULL,
  password_hash VARCHAR(256) NOT NULL,
  role VARCHAR(32) NOT NULL DEFAULT 'admin',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO gcfm_users (username,password_hash) VALUES
('admin', '$2a$12$DqM2suIU0/DuGrx3BwYI.O7rB6ig84yYI6FqdtYYdlcYNSeNBYxe');
INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (5, '0.5');
