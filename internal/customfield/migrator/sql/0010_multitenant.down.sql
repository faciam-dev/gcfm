ALTER TABLE gcfm_audit_logs
  DROP PRIMARY KEY,
  DROP COLUMN tenant_id;
ALTER TABLE gcfm_audit_logs
  ADD PRIMARY KEY (id);

ALTER TABLE gcfm_custom_fields
  DROP PRIMARY KEY,
  DROP COLUMN tenant_id;
ALTER TABLE gcfm_custom_fields
  ADD PRIMARY KEY (table_name, column_name);

DELETE FROM gcfm_registry_schema_version WHERE version=10;
