ALTER TABLE gcfm_audit_logs
  ADD COLUMN IF NOT EXISTS added_count   INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS removed_count INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS change_count  INT NOT NULL DEFAULT 0;

INSERT INTO gcfm_registry_schema_version(version, semver)
  VALUES (16, '0.16')
ON CONFLICT DO NOTHING;
