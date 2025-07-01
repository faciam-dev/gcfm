ALTER TABLE custom_fields CHANGE COLUMN `default` `default_value` TEXT DEFAULT NULL;
ALTER TABLE custom_fields ADD COLUMN has_default BOOLEAN NOT NULL DEFAULT FALSE AFTER default_value;
UPDATE custom_fields SET has_default = TRUE WHERE default_value IS NOT NULL;
INSERT INTO registry_schema_version(version, semver) VALUES (7, '0.7');
