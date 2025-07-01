ALTER TABLE gcfm_custom_fields DROP COLUMN has_default;
ALTER TABLE gcfm_custom_fields RENAME COLUMN default_value TO "default";
DELETE FROM gcfm_registry_schema_version WHERE version=7;
