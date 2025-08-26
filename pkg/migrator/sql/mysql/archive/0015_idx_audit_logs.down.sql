DROP INDEX idx_gcfm_audit_tenant_time ON gcfm_audit_logs;

DELETE FROM gcfm_registry_schema_version WHERE version=15;
