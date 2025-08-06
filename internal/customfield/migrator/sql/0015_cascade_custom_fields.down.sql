ALTER TABLE gcfm_custom_fields
  DROP FOREIGN KEY fk_gcfm_custom_fields_db,
  ADD CONSTRAINT fk_gcfm_custom_fields_db
      FOREIGN KEY (db_id) REFERENCES monitored_databases(id);
DELETE FROM gcfm_registry_schema_version WHERE version=15;
