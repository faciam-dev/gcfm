ALTER TABLE gcfm_custom_fields
  ADD COLUMN IF NOT EXISTS db_id BIGINT NOT NULL REFERENCES monitored_databases(id);

UPDATE gcfm_custom_fields SET db_id = 1 WHERE db_id IS NULL;

ALTER TABLE gcfm_custom_fields
  DROP CONSTRAINT IF EXISTS gcfm_custom_fields_pkey,
  ADD PRIMARY KEY (db_id, tenant_id, table_name, column_name);

INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (13, '0.13');
