CREATE UNIQUE INDEX IF NOT EXISTS uq_gcfm_cf_tenant_db_tbl_col
  ON gcfm_custom_fields(tenant_id, db_id, table_name, column_name);

INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (16, '0.16');
