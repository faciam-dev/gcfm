ALTER TABLE gcfm_custom_fields
  DROP CONSTRAINT fk_gcfm_custom_fields_db;

ALTER TABLE gcfm_custom_fields
  ADD CONSTRAINT fk_gcfm_custom_fields_db
      FOREIGN KEY (db_id) REFERENCES monitored_databases(id)
      ON DELETE CASCADE;

INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (15, '0.15');
