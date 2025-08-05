ALTER TABLE gcfm_custom_fields
  ADD COLUMN IF NOT EXISTS db_id BIGINT NOT NULL REFERENCES monitored_databases(id);

UPDATE gcfm_custom_fields SET db_id = 1 WHERE db_id IS NULL;

SET @pk := (
  SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index)
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'gcfm_custom_fields'
    AND index_name = 'PRIMARY'
);

SET @stmt := IF(@pk = 'db_id,tenant_id,table_name,column_name',
  'SELECT 1',
  'ALTER TABLE gcfm_custom_fields DROP PRIMARY KEY, ADD PRIMARY KEY (db_id, tenant_id, table_name, column_name)'
);

PREPARE s FROM @stmt;
EXECUTE s;
DEALLOCATE PREPARE s;

INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (13, '0.13');
