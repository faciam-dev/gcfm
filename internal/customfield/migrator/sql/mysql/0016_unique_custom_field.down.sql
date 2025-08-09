DROP INDEX uq_gcfm_cf_tenant_db_tbl_col ON gcfm_custom_fields;

DELETE FROM gcfm_registry_schema_version WHERE version=16;
