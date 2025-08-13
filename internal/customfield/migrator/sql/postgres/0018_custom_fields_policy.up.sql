INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields', 'POST'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields', 'PUT'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields', 'DELETE'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields', 'GET'
  FROM gcfm_roles r WHERE r.name='viewer'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_registry_schema_version(version, semver)
  VALUES (18, '0.18')
ON CONFLICT DO NOTHING;
