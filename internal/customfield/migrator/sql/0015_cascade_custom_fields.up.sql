ALTER TABLE gcfm_custom_fields
  DROP FOREIGN KEY fk_gcfm_custom_fields_db,
  ADD CONSTRAINT fk_gcfm_custom_fields_db
      FOREIGN KEY (db_id) REFERENCES monitored_databases(id)
      ON DELETE CASCADE;
INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (15, '0.15');
