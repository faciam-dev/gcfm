ALTER TABLE gcfm_custom_fields DROP COLUMN validator_plugin;
DELETE FROM gcfm_registry_schema_version WHERE version=3;
