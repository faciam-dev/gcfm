CREATE TABLE gcfm_roles (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(64) UNIQUE NOT NULL,
  comment VARCHAR(128)
);

CREATE TABLE gcfm_user_roles (
  user_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  PRIMARY KEY (user_id, role_id),
  FOREIGN KEY (user_id) REFERENCES gcfm_users(id) ON DELETE CASCADE,
  FOREIGN KEY (role_id) REFERENCES gcfm_roles(id) ON DELETE CASCADE
);

CREATE TABLE gcfm_role_policies (
  role_id BIGINT NOT NULL,
  path VARCHAR(128) NOT NULL,
  method VARCHAR(8) NOT NULL,
  PRIMARY KEY(role_id, path, method),
  FOREIGN KEY(role_id) REFERENCES gcfm_roles(id) ON DELETE CASCADE
);

INSERT INTO gcfm_roles(name) VALUES ('admin'),('editor'),('viewer');
INSERT INTO gcfm_user_roles(user_id, role_id)
  SELECT u.id, r.id
    FROM gcfm_users u, gcfm_roles r
   WHERE u.username='admin' AND r.name='admin';
INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (8, '0.8');
