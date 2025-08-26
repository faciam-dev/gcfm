CREATE TABLE IF NOT EXISTS gcfm_events_failed (
  id SERIAL PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  payload JSONB NOT NULL,
  attempts INT NOT NULL,
  last_error TEXT,
  inserted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (9, '0.9');
