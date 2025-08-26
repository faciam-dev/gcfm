ALTER TABLE gcfm_custom_fields
  DROP CONSTRAINT IF EXISTS gcfm_custom_fields_pkey,
  ADD PRIMARY KEY (tenant_id, table_name, column_name);
ALTER TABLE gcfm_custom_fields DROP COLUMN IF EXISTS db_id;
DELETE FROM gcfm_registry_schema_version WHERE version=13;
