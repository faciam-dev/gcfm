ALTER TABLE gcfm_custom_fields
  ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE gcfm_custom_fields
  DROP CONSTRAINT gcfm_custom_fields_pkey,
  ADD PRIMARY KEY (tenant_id, table_name, column_name);

ALTER TABLE gcfm_audit_logs
  ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE gcfm_audit_logs
  DROP CONSTRAINT gcfm_audit_logs_pkey,
  ADD PRIMARY KEY (tenant_id, id);

INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (10, '0.10');
