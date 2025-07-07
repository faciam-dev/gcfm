INSERT INTO gcfm_custom_fields (table_name, column_name, data_type, validator)
VALUES
  ('posts', 'author_email', 'varchar(255)', 'email'),
  ('posts', 'rating', 'int', 'number');
INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (10, '0.10');
