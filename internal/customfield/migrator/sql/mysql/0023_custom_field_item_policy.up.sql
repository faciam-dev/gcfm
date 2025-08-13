INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields/:name', 'PUT'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields/:name', 'DELETE'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields/:name', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (23, '0.23');
