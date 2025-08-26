DELETE FROM gcfm_role_policies p USING gcfm_roles r WHERE p.role_id=r.id AND r.name='editor' AND p.path='/v1/custom-fields' AND p.method='GET';
DELETE FROM gcfm_role_policies p USING gcfm_roles r WHERE p.role_id=r.id AND r.name='editor' AND p.path='/v1/custom-fields' AND p.method='POST';
DELETE FROM gcfm_role_policies p USING gcfm_roles r WHERE p.role_id=r.id AND r.name='editor' AND p.path='/v1/custom-fields' AND p.method='PUT';
DELETE FROM gcfm_role_policies p USING gcfm_roles r WHERE p.role_id=r.id AND r.name='editor' AND p.path='/v1/custom-fields' AND p.method='DELETE';
DELETE FROM gcfm_role_policies p USING gcfm_roles r WHERE p.role_id=r.id AND r.name='viewer' AND p.path='/v1/custom-fields' AND p.method='GET';
DELETE FROM gcfm_registry_schema_version WHERE version=18;
