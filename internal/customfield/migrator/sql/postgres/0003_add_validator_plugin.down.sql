ALTER TABLE gcfm_custom_fields DROP COLUMN validator_plugin;
DELETE FROM registry_schema_version WHERE version=3;
