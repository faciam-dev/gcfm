DELETE p FROM gcfm_role_policies p JOIN gcfm_roles r ON p.role_id=r.id WHERE r.name='admin' AND p.path='/v1/snapshots' AND p.method='GET';
DELETE p FROM gcfm_role_policies p JOIN gcfm_roles r ON p.role_id=r.id WHERE r.name='admin' AND p.path='/v1/snapshots' AND p.method='POST';
DELETE p FROM gcfm_role_policies p JOIN gcfm_roles r ON p.role_id=r.id WHERE r.name='admin' AND p.path='/v1/snapshots/{ver}/apply' AND p.method='POST';
DELETE p FROM gcfm_role_policies p JOIN gcfm_roles r ON p.role_id=r.id WHERE r.name='admin' AND p.path='/v1/databases' AND p.method='GET';
DELETE p FROM gcfm_role_policies p JOIN gcfm_roles r ON p.role_id=r.id WHERE r.name='admin' AND p.path='/v1/databases' AND p.method='POST';
DELETE p FROM gcfm_role_policies p JOIN gcfm_roles r ON p.role_id=r.id WHERE r.name='editor' AND p.path='/v1/snapshots' AND p.method='GET';
DELETE p FROM gcfm_role_policies p JOIN gcfm_roles r ON p.role_id=r.id WHERE r.name='editor' AND p.path='/v1/databases' AND p.method='GET';
DELETE FROM gcfm_registry_schema_version WHERE version=21;
