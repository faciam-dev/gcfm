ALTER TABLE gcfm_custom_fields DROP COLUMN validator;
ALTER TABLE gcfm_custom_fields DROP COLUMN "default";
ALTER TABLE gcfm_custom_fields DROP COLUMN "unique";
ALTER TABLE gcfm_custom_fields DROP COLUMN nullable;
DELETE FROM gcfm_registry_schema_version WHERE version=6;
