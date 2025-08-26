ALTER TABLE gcfm_custom_fields
  ADD COLUMN IF NOT EXISTS db_id BIGINT;

UPDATE gcfm_custom_fields SET db_id = 1 WHERE db_id IS NULL;

ALTER TABLE gcfm_custom_fields
  ALTER COLUMN db_id SET NOT NULL;

ALTER TABLE gcfm_custom_fields
  ADD CONSTRAINT fk_gcfm_custom_fields_db FOREIGN KEY (db_id) REFERENCES monitored_databases(id),
  DROP CONSTRAINT IF EXISTS gcfm_custom_fields_pkey,
  ADD PRIMARY KEY (db_id, tenant_id, table_name, column_name);

INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (13, '0.13');
