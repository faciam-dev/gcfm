ALTER TABLE gcfm_users ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE gcfm_users DROP INDEX username;
ALTER TABLE gcfm_users ADD UNIQUE INDEX gcfm_users_tenant_username (tenant_id, username);
INSERT INTO gcfm_user_roles(user_id, role_id)
  SELECT u.id, r.id FROM gcfm_users u JOIN gcfm_roles r ON r.name=u.role
ON DUPLICATE KEY UPDATE role_id=role_id;
ALTER TABLE gcfm_users DROP COLUMN role;
INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (19, '0.19');
