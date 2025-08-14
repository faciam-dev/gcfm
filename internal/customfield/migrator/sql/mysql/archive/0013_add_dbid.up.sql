SET @stmt := (
  SELECT IF(
    EXISTS (
      SELECT 1 FROM information_schema.COLUMNS
      WHERE table_schema = DATABASE()
        AND table_name = 'gcfm_custom_fields'
        AND column_name = 'db_id'
    ),
    'SELECT 1',
    'ALTER TABLE gcfm_custom_fields ADD COLUMN db_id BIGINT NULL'
  )
);
PREPARE s FROM @stmt;
EXECUTE s;
DEALLOCATE PREPARE s;

-- Step 2: Set db_id to 1 for all existing rows
UPDATE gcfm_custom_fields SET db_id = 1 WHERE db_id IS NULL;

-- Step 3: Alter column to NOT NULL and add foreign key constraint if not already present
SET @fk := (
  SELECT CONSTRAINT_NAME
  FROM information_schema.KEY_COLUMN_USAGE
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'gcfm_custom_fields'
    AND COLUMN_NAME = 'db_id'
    AND REFERENCED_TABLE_NAME = 'monitored_databases'
);

SET @stmt := (
  SELECT IF(
    @fk IS NOT NULL,
    'SELECT 1',
    'ALTER TABLE gcfm_custom_fields MODIFY db_id BIGINT NOT NULL, ADD CONSTRAINT fk_gcfm_custom_fields_db FOREIGN KEY (db_id) REFERENCES monitored_databases(id)'
  )
);
PREPARE s FROM @stmt;
EXECUTE s;
DEALLOCATE PREPARE s;

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
