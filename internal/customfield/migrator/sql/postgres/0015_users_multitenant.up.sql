ALTER TABLE gcfm_users ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE gcfm_users DROP CONSTRAINT gcfm_users_username_key;
ALTER TABLE gcfm_users ADD CONSTRAINT gcfm_users_tenant_username_key UNIQUE (tenant_id, username);
INSERT INTO gcfm_user_roles(user_id, role_id)
  SELECT u.id, r.id FROM gcfm_users u JOIN gcfm_roles r ON r.name=u.role
ON CONFLICT DO NOTHING;
ALTER TABLE gcfm_users DROP COLUMN role;
INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (15, '0.15');
