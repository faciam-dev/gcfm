ALTER TABLE gcfm_custom_fields ADD COLUMN validator_plugin VARCHAR(100);
INSERT INTO registry_schema_version(version, semver) VALUES (3, '0.3');
