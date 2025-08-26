ALTER TABLE monitored_databases
  DROP COLUMN schema_name,
  DROP COLUMN dsn;

DELETE FROM gcfm_registry_schema_version WHERE version=17;
