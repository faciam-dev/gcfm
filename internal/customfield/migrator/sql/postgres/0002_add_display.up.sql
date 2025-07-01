ALTER TABLE gcfm_custom_fields ADD COLUMN label_key VARCHAR(255);
ALTER TABLE gcfm_custom_fields ADD COLUMN widget VARCHAR(50);
ALTER TABLE gcfm_custom_fields ADD COLUMN placeholder_key VARCHAR(255);
INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (2, '0.2');
