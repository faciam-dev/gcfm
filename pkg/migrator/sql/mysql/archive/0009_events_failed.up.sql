CREATE TABLE IF NOT EXISTS gcfm_events_failed (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  payload JSON NOT NULL,
  attempts INT NOT NULL,
  last_error TEXT,
  inserted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (9, '0.9');
