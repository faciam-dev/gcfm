ALTER TABLE gcfm_audit_logs
  DROP COLUMN change_count,
  DROP COLUMN removed_count,
  DROP COLUMN added_count;
DELETE FROM gcfm_registry_schema_version WHERE version=16;
