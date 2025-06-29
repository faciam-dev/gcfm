ALTER TABLE custom_fields DROP COLUMN placeholder_key;
ALTER TABLE custom_fields DROP COLUMN widget;
ALTER TABLE custom_fields DROP COLUMN label_key;
DELETE FROM registry_schema_version WHERE version=2;
