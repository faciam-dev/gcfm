ALTER TABLE gcfm_custom_fields RENAME COLUMN "default" TO default_value;
ALTER TABLE gcfm_custom_fields ADD COLUMN has_default BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE gcfm_custom_fields SET has_default = TRUE WHERE default_value IS NOT NULL;
INSERT INTO registry_schema_version(version, semver) VALUES (7, '0.7');
