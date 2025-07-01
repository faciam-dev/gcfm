ALTER TABLE custom_fields DROP COLUMN has_default;
ALTER TABLE custom_fields RENAME COLUMN default_value TO "default";
DELETE FROM registry_schema_version WHERE version=7;
