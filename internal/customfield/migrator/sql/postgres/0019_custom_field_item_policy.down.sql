DELETE FROM gcfm_role_policies p USING gcfm_roles r WHERE p.role_id=r.id AND r.name='editor' AND p.path='/v1/custom-fields/:name' AND p.method='PUT';
DELETE FROM gcfm_role_policies p USING gcfm_roles r WHERE p.role_id=r.id AND r.name='editor' AND p.path='/v1/custom-fields/:name' AND p.method='DELETE';
DELETE FROM gcfm_role_policies p USING gcfm_roles r WHERE p.role_id=r.id AND r.name='editor' AND p.path='/v1/custom-fields/:name' AND p.method='GET';
DELETE FROM gcfm_registry_schema_version WHERE version=19;
