INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/snapshots', 'GET'
  FROM gcfm_roles r WHERE r.name='admin'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/snapshots', 'POST'
  FROM gcfm_roles r WHERE r.name='admin'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/snapshots/{ver}/apply', 'POST'
  FROM gcfm_roles r WHERE r.name='admin'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases', 'GET'
  FROM gcfm_roles r WHERE r.name='admin'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases', 'POST'
  FROM gcfm_roles r WHERE r.name='admin'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/snapshots', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases', 'GET'
  FROM gcfm_roles r WHERE r.name='viewer'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/metadata/tables', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/metadata/tables', 'GET'
  FROM gcfm_roles r WHERE r.name='viewer'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases/{id}/scan', 'POST'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (21, '0.21');
