DROP TABLE IF EXISTS registry_snapshots;
DROP TABLE IF EXISTS audit_logs;
DELETE FROM registry_schema_version WHERE version=4;
