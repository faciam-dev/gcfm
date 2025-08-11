ALTER TABLE gcfm_audit_logs
  ADD COLUMN added_count INT NOT NULL DEFAULT 0,
  ADD COLUMN removed_count INT NOT NULL DEFAULT 0,
  ADD COLUMN change_count INT NOT NULL DEFAULT 0;
INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (16, '0.16');
