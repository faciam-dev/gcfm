CREATE TABLE IF NOT EXISTS gcfm_registry_schema_version (
    version INT PRIMARY KEY,
    semver VARCHAR(32) NOT NULL,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS gcfm_monitored_databases (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    name VARCHAR(255) NOT NULL,
    driver VARCHAR(16) NOT NULL,
    dsn VARCHAR(512) NOT NULL DEFAULT '',
    dsn_enc LONGBLOB,
    schema_name VARCHAR(64),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY gcfm_monitored_databases_tenant_name (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS gcfm_custom_fields (
    db_id BIGINT NOT NULL,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    table_name VARCHAR(255) NOT NULL,
    column_name VARCHAR(255) NOT NULL,
    data_type VARCHAR(255) NOT NULL,
    label_key VARCHAR(255),
    widget VARCHAR(50),
    placeholder_key VARCHAR(255),
    nullable BOOLEAN NOT NULL DEFAULT FALSE,
    `unique` BOOLEAN NOT NULL DEFAULT FALSE,
    has_default BOOLEAN NOT NULL DEFAULT FALSE,
    default_value TEXT,
    validator VARCHAR(64),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (db_id, tenant_id, table_name, column_name),
    CONSTRAINT fk_gcfm_custom_fields_db FOREIGN KEY (db_id) REFERENCES gcfm_monitored_databases(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS gcfm_events_failed (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    payload JSON NOT NULL,
    attempts INT NOT NULL,
    last_error TEXT,
    inserted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS gcfm_registry_snapshots (
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    semver VARCHAR(32) NOT NULL,
    yaml LONGBLOB NOT NULL,
    taken_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    author VARCHAR(64),
    UNIQUE KEY uq_snapshots_tenant_semver (tenant_id, semver)
);

CREATE TABLE IF NOT EXISTS gcfm_users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    username VARCHAR(64) NOT NULL,
    password_hash VARCHAR(256) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY gcfm_users_tenant_username (tenant_id, username)
);

CREATE TABLE IF NOT EXISTS gcfm_roles (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(64) UNIQUE NOT NULL,
    comment VARCHAR(128)
);

CREATE TABLE IF NOT EXISTS gcfm_user_roles (
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (user_id) REFERENCES gcfm_users(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES gcfm_roles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS gcfm_role_policies (
    role_id BIGINT NOT NULL,
    path VARCHAR(128) NOT NULL,
    method VARCHAR(8) NOT NULL,
    PRIMARY KEY (role_id, path, method),
    FOREIGN KEY (role_id) REFERENCES gcfm_roles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS gcfm_audit_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    actor VARCHAR(64),
    action VARCHAR(32),
    table_name VARCHAR(255),
    column_name VARCHAR(255),
    record_id VARCHAR(64),
    before_json JSON,
    after_json JSON,
    added_count INT DEFAULT 0,
    removed_count INT DEFAULT 0,
    change_count INT DEFAULT 0,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_gcfm_audit_tenant_time ON gcfm_audit_logs(tenant_id, applied_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS casbin_rule (
    id INT AUTO_INCREMENT PRIMARY KEY,
    ptype VARCHAR(100),
    v0 VARCHAR(100),
    v1 VARCHAR(100),
    v2 VARCHAR(100),
    v3 VARCHAR(100),
    v4 VARCHAR(100),
    v5 VARCHAR(100)
);

INSERT INTO gcfm_roles(id, name) VALUES (1,'admin') ON DUPLICATE KEY UPDATE name=VALUES(name);
INSERT INTO gcfm_roles(id, name) VALUES (2,'editor') ON DUPLICATE KEY UPDATE name=VALUES(name);
INSERT INTO gcfm_roles(id, name) VALUES (3,'viewer') ON DUPLICATE KEY UPDATE name=VALUES(name);

INSERT INTO gcfm_users(id, tenant_id, username, password_hash) VALUES (1,'default','admin','$2a$12$m6067tTF2aFUNYum/PPEeONElY.Ohk34KWBrvCNcYzs5nB0j.L/N.')
ON DUPLICATE KEY UPDATE password_hash=VALUES(password_hash);

INSERT INTO gcfm_user_roles(user_id, role_id) VALUES (1,1) ON DUPLICATE KEY UPDATE role_id=VALUES(role_id);

INSERT INTO gcfm_role_policies(role_id, path, method) VALUES
    (1,'/v1/*','GET'),
    (1,'/v1/*','POST'),
    (1,'/v1/*','PUT'),
    (1,'/v1/*','DELETE')
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases/*/tables', 'GET'
  FROM gcfm_roles r WHERE r.name='admin'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/snapshots', 'GET'
  FROM gcfm_roles r WHERE r.name='admin'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/snapshots', 'POST'
  FROM gcfm_roles r WHERE r.name='admin'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/snapshots/{ver}/apply', 'POST'
  FROM gcfm_roles r WHERE r.name='admin'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases', 'GET'
  FROM gcfm_roles r WHERE r.name='admin'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases', 'POST'
  FROM gcfm_roles r WHERE r.name='admin'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases/{id}', 'PUT'
  FROM gcfm_roles r WHERE r.name='admin'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases/{id}', 'DELETE'
  FROM gcfm_roles r WHERE r.name='admin'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases/{id}/scan', 'POST'
  FROM gcfm_roles r WHERE r.name='admin'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/snapshots', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/metadata/tables', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases/{id}/scan', 'POST'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases', 'GET'
  FROM gcfm_roles r WHERE r.name='viewer'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/metadata/tables', 'GET'
  FROM gcfm_roles r WHERE r.name='viewer'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields', 'POST'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields', 'PUT'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields', 'DELETE'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields', 'GET'
  FROM gcfm_roles r WHERE r.name='viewer'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields/:name', 'PUT'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields/:name', 'DELETE'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields/:name', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON DUPLICATE KEY UPDATE path=VALUES(path);

INSERT INTO casbin_rule (ptype,v0,v1,v2,v3,v4,v5) VALUES
    ('p','admin','*','*','*','*','*'),
    ('g','admin','admin','','','','')
ON DUPLICATE KEY UPDATE v0=VALUES(v0);

-- targets configuration tables
CREATE TABLE IF NOT EXISTS gcfm_targets (
  `key`          VARCHAR(255) PRIMARY KEY,
  driver       TEXT NOT NULL,
  dsn          TEXT NOT NULL,
  schema_name  VARCHAR(64) DEFAULT '',
  max_open_conns INT DEFAULT 0,
  max_idle_conns INT DEFAULT 0,
  conn_max_idle_ms BIGINT DEFAULT 0,
  conn_max_life_ms BIGINT DEFAULT 0,
  is_default   BOOLEAN DEFAULT FALSE,
  updated_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS gcfm_target_labels (
  `key`   VARCHAR(255) NOT NULL,
  label  VARCHAR(255) NOT NULL,
  PRIMARY KEY (`key`, label),
  CONSTRAINT fk_target_labels FOREIGN KEY (`key`) REFERENCES gcfm_targets(`key`) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS gcfm_target_config_version (
  id SMALLINT PRIMARY KEY DEFAULT 1,
  version VARCHAR(32) NOT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
INSERT INTO gcfm_target_config_version (id, version)
  VALUES (1, REPLACE(uuid(), '-', ''))
ON DUPLICATE KEY UPDATE version=version;

-- MySQL does not support partial indexes with a WHERE clause.
-- Use a functional index that yields a non-NULL value only when
-- `is_default` is TRUE to ensure only one default target exists.
CREATE UNIQUE INDEX gcfm_targets_one_default
  ON gcfm_targets ((CASE WHEN is_default THEN 1 END));

INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (1,'0.3');
