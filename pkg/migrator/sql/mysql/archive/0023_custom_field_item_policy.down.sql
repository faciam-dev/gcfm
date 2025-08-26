DELETE p FROM gcfm_role_policies p JOIN gcfm_roles r ON p.role_id=r.id WHERE r.name='editor' AND p.path='/v1/custom-fields/:name' AND p.method='PUT';
DELETE p FROM gcfm_role_policies p JOIN gcfm_roles r ON p.role_id=r.id WHERE r.name='editor' AND p.path='/v1/custom-fields/:name' AND p.method='DELETE';
DELETE p FROM gcfm_role_policies p JOIN gcfm_roles r ON p.role_id=r.id WHERE r.name='editor' AND p.path='/v1/custom-fields/:name' AND p.method='GET';
DELETE FROM gcfm_registry_schema_version WHERE version=23;
