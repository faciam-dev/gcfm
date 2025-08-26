ALTER TABLE gcfm_users ADD COLUMN role VARCHAR(32) NOT NULL DEFAULT 'admin';
UPDATE gcfm_users u
  JOIN gcfm_user_roles ur ON ur.user_id=u.id
  JOIN gcfm_roles r ON r.id=ur.role_id
  SET u.role=r.name;
ALTER TABLE gcfm_users DROP INDEX gcfm_users_tenant_username;
ALTER TABLE gcfm_users DROP COLUMN tenant_id;
ALTER TABLE gcfm_users ADD UNIQUE INDEX username (username);
DELETE FROM gcfm_registry_schema_version WHERE version=19;
