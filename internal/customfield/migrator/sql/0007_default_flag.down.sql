ALTER TABLE custom_fields DROP COLUMN has_default;
ALTER TABLE custom_fields CHANGE COLUMN `default_value` `default` TEXT DEFAULT NULL;
DELETE FROM registry_schema_version WHERE version=7;
