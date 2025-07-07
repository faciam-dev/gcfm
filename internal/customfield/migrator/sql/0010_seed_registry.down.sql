DELETE FROM gcfm_custom_fields WHERE table_name='posts' AND column_name='author_email';
DELETE FROM gcfm_custom_fields WHERE table_name='posts' AND column_name='rating';
DELETE FROM gcfm_registry_schema_version WHERE version=10;
