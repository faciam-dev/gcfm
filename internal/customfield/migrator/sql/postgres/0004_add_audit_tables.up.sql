CREATE TABLE IF NOT EXISTS gcfm_audit_logs (
  id BIGSERIAL PRIMARY KEY,
  actor VARCHAR(64) NOT NULL,
  action VARCHAR(10) NOT NULL CHECK (action IN ('add','update','delete')),
  table_name VARCHAR(128) NOT NULL,
  column_name VARCHAR(128) NOT NULL,
  before_json JSONB NULL,
  after_json JSONB NULL,
  applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS gcfm_registry_snapshots (
  id BIGSERIAL PRIMARY KEY,
  version INT NOT NULL,
  semver VARCHAR(20) NOT NULL,
  yaml BYTEA NOT NULL,
  taken_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO registry_schema_version(version, semver) VALUES (4, '0.4');
