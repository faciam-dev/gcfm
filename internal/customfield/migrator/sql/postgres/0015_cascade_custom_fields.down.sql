ALTER TABLE gcfm_custom_fields
  DROP CONSTRAINT fk_gcfm_custom_fields_db;

ALTER TABLE gcfm_custom_fields
  ADD CONSTRAINT fk_gcfm_custom_fields_db
      FOREIGN KEY (db_id) REFERENCES monitored_databases(id);

DELETE FROM gcfm_registry_schema_version WHERE version=15;
