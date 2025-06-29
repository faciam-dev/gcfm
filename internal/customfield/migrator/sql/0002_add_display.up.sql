ALTER TABLE custom_fields ADD COLUMN label_key VARCHAR(255);
ALTER TABLE custom_fields ADD COLUMN widget VARCHAR(50);
ALTER TABLE custom_fields ADD COLUMN placeholder_key VARCHAR(255);
INSERT INTO registry_schema_version(version, semver) VALUES (2, '0.2');
