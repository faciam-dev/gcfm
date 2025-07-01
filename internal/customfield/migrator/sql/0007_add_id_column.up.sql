ALTER TABLE custom_fields DROP PRIMARY KEY;
ALTER TABLE custom_fields ADD COLUMN id BIGINT AUTO_INCREMENT PRIMARY KEY FIRST;
ALTER TABLE custom_fields ADD UNIQUE KEY idx_custom_fields_table_column (table_name, column_name);
INSERT INTO registry_schema_version(version, semver) VALUES (7, '0.7');
