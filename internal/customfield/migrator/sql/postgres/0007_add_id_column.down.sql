DROP INDEX IF EXISTS idx_custom_fields_table_column;
ALTER TABLE custom_fields DROP CONSTRAINT custom_fields_pkey;
ALTER TABLE custom_fields DROP COLUMN id;
ALTER TABLE custom_fields ADD PRIMARY KEY (table_name, column_name);
DELETE FROM registry_schema_version WHERE version=7;
