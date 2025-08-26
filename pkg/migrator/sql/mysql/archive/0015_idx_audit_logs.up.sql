CREATE INDEX idx_gcfm_audit_tenant_time
  ON gcfm_audit_logs(tenant_id, applied_at DESC, id DESC);

INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (15, '0.15');
