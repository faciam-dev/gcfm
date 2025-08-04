CREATE TABLE monitored_databases (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  name VARCHAR(64) UNIQUE,
  driver VARCHAR(16) NOT NULL,
  dsn_enc LONGBLOB NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (12, '0.12');
