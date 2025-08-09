INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases/*/tables', 'GET'
  FROM gcfm_roles r WHERE r.name='admin'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (18, '0.18');
