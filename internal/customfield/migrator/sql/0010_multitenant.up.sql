ALTER TABLE gcfm_custom_fields
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE gcfm_custom_fields
  DROP PRIMARY KEY,
  ADD PRIMARY KEY (tenant_id, table_name, column_name);

ALTER TABLE gcfm_audit_logs
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE gcfm_audit_logs
  DROP PRIMARY KEY,
  ADD PRIMARY KEY (tenant_id, id);

INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (10, '0.10');
