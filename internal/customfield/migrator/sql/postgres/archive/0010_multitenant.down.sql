ALTER TABLE gcfm_audit_logs
  DROP CONSTRAINT gcfm_audit_logs_pkey,
  DROP COLUMN tenant_id,
  ADD PRIMARY KEY (id);

ALTER TABLE gcfm_custom_fields
  DROP CONSTRAINT gcfm_custom_fields_pkey,
  DROP COLUMN tenant_id,
  ADD PRIMARY KEY (table_name, column_name);

DELETE FROM gcfm_registry_schema_version WHERE version=10;
