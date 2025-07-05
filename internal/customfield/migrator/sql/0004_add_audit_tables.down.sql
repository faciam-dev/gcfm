DROP TABLE IF EXISTS gcfm_registry_snapshots;
DROP TABLE IF EXISTS gcfm_audit_logs;
DELETE FROM registry_schema_version WHERE version=4;
