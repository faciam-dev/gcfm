CREATE TABLE IF NOT EXISTS gcfm_audit_logs (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  actor VARCHAR(64) NOT NULL,
  action ENUM('add','update','delete') NOT NULL,
  table_name VARCHAR(128) NOT NULL,
  column_name VARCHAR(128) NOT NULL,
  before_json JSON NULL,
  after_json JSON NULL,
  applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS gcfm_registry_snapshots (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  version INT NOT NULL,
  semver VARCHAR(20) NOT NULL,
  yaml LONGBLOB NOT NULL,
  taken_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (4, '0.4');
