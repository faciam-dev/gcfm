ALTER TABLE monitored_databases
  DROP COLUMN IF EXISTS schema_name,
  DROP COLUMN IF EXISTS dsn,
  DROP COLUMN IF EXISTS driver;

DELETE FROM gcfm_registry_schema_version WHERE version=17;
