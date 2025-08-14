DELETE p FROM gcfm_role_policies p JOIN gcfm_roles r ON p.role_id=r.id WHERE r.name='editor' AND p.path='/v1/custom-fields' AND p.method='GET';
DELETE p FROM gcfm_role_policies p JOIN gcfm_roles r ON p.role_id=r.id WHERE r.name='editor' AND p.path='/v1/custom-fields' AND p.method='POST';
DELETE p FROM gcfm_role_policies p JOIN gcfm_roles r ON p.role_id=r.id WHERE r.name='editor' AND p.path='/v1/custom-fields' AND p.method='PUT';
DELETE p FROM gcfm_role_policies p JOIN gcfm_roles r ON p.role_id=r.id WHERE r.name='editor' AND p.path='/v1/custom-fields' AND p.method='DELETE';
DELETE p FROM gcfm_role_policies p JOIN gcfm_roles r ON p.role_id=r.id WHERE r.name='viewer' AND p.path='/v1/custom-fields' AND p.method='GET';
DELETE FROM gcfm_registry_schema_version WHERE version=22;
