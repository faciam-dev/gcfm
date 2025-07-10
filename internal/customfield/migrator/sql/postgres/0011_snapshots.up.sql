DROP TABLE IF EXISTS gcfm_registry_snapshots;
CREATE TABLE gcfm_registry_snapshots (
  tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
  id BIGSERIAL PRIMARY KEY,
  semver VARCHAR(32) NOT NULL,
  yaml BYTEA NOT NULL,
  taken_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  author VARCHAR(64)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_snapshots_tenant_semver ON gcfm_registry_snapshots(tenant_id, semver);
INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (11, '0.11');
