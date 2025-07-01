ALTER TABLE gcfm_custom_fields DROP COLUMN has_default;
ALTER TABLE gcfm_custom_fields CHANGE COLUMN `default_value` `default` TEXT DEFAULT NULL;
DELETE FROM gcfm_registry_schema_version WHERE version=7;
