INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields/:name', 'PUT'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields/:name', 'DELETE'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields/:name', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_registry_schema_version(version, semver)
  VALUES (19, '0.19')
ON CONFLICT DO NOTHING;
