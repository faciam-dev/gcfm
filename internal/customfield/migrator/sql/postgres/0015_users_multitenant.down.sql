ALTER TABLE gcfm_users ADD COLUMN role VARCHAR(32) NOT NULL DEFAULT 'admin';
UPDATE gcfm_users u
SET role=r.name FROM gcfm_user_roles ur JOIN gcfm_roles r ON r.id=ur.role_id
WHERE u.id=ur.user_id;
ALTER TABLE gcfm_users DROP CONSTRAINT gcfm_users_tenant_username_key;
ALTER TABLE gcfm_users DROP COLUMN tenant_id;
ALTER TABLE gcfm_users ADD CONSTRAINT gcfm_users_username_key UNIQUE (username);
DELETE FROM gcfm_registry_schema_version WHERE version=15;
