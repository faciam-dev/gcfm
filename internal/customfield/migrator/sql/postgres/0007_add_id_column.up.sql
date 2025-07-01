ALTER TABLE custom_fields DROP CONSTRAINT custom_fields_pkey;
ALTER TABLE custom_fields ADD COLUMN id BIGSERIAL PRIMARY KEY;
CREATE UNIQUE INDEX idx_custom_fields_table_column ON custom_fields(table_name, column_name);
INSERT INTO registry_schema_version(version, semver) VALUES (7, '0.7');
