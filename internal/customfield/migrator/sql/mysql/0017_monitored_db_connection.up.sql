ALTER TABLE monitored_databases
  ADD COLUMN IF NOT EXISTS driver ENUM('mysql','postgres') NOT NULL DEFAULT 'mysql',
  ADD COLUMN IF NOT EXISTS dsn VARCHAR(512) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS schema_name VARCHAR(64) NULL;

INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (17, '0.17');
