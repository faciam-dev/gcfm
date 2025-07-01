ALTER TABLE custom_fields DROP INDEX idx_custom_fields_table_column;
ALTER TABLE custom_fields DROP PRIMARY KEY;
ALTER TABLE custom_fields DROP COLUMN id;
ALTER TABLE custom_fields ADD PRIMARY KEY (table_name, column_name);
DELETE FROM registry_schema_version WHERE version=7;
