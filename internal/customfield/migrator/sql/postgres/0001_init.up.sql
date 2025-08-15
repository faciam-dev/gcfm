CREATE SCHEMA IF NOT EXISTS public;
SET search_path TO public;

CREATE TABLE IF NOT EXISTS gcfm_registry_schema_version (
    version INT PRIMARY KEY,
    semver TEXT NOT NULL,
    applied_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS gcfm_monitored_databases (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    name VARCHAR(255) NOT NULL,
    driver VARCHAR(16) NOT NULL,
    dsn TEXT NOT NULL DEFAULT '',
    dsn_enc BYTEA,
    schema_name VARCHAR(64),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS gcfm_custom_fields (
    db_id BIGINT NOT NULL REFERENCES gcfm_monitored_databases(id) ON DELETE CASCADE,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    table_name TEXT NOT NULL,
    column_name TEXT NOT NULL,
    data_type TEXT NOT NULL,
    label_key VARCHAR(255),
    widget VARCHAR(50),
    placeholder_key VARCHAR(255),
    nullable BOOLEAN NOT NULL DEFAULT FALSE,
    "unique" BOOLEAN NOT NULL DEFAULT FALSE,
    has_default BOOLEAN NOT NULL DEFAULT FALSE,
    default_value TEXT,
    validator VARCHAR(64),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (db_id, tenant_id, table_name, column_name)
);

CREATE TABLE IF NOT EXISTS gcfm_events_failed (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    attempts INT NOT NULL,
    last_error TEXT,
    inserted_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS gcfm_registry_snapshots (
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    id BIGSERIAL PRIMARY KEY,
    semver VARCHAR(32) NOT NULL,
    yaml BYTEA NOT NULL,
    taken_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    author VARCHAR(64),
    UNIQUE (tenant_id, semver)
);

CREATE TABLE IF NOT EXISTS gcfm_users (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    username VARCHAR(64) NOT NULL,
    password_hash VARCHAR(256) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, username)
);

CREATE TABLE IF NOT EXISTS gcfm_roles (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(64) UNIQUE NOT NULL,
    comment VARCHAR(128)
);

CREATE TABLE IF NOT EXISTS gcfm_user_roles (
    user_id BIGINT NOT NULL REFERENCES gcfm_users(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES gcfm_roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS gcfm_role_policies (
    role_id BIGINT NOT NULL REFERENCES gcfm_roles(id) ON DELETE CASCADE,
    path VARCHAR(128) NOT NULL,
    method VARCHAR(8) NOT NULL,
    PRIMARY KEY (role_id, path, method)
);

CREATE TABLE IF NOT EXISTS gcfm_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    actor VARCHAR(64),
    action VARCHAR(32),
    table_name TEXT,
    column_name TEXT,
    record_id TEXT,
    before_json JSONB,
    after_json JSONB,
    added_count INT DEFAULT 0,
    removed_count INT DEFAULT 0,
    change_count INT DEFAULT 0,
    applied_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_gcfm_audit_tenant_time ON gcfm_audit_logs(tenant_id, applied_at DESC, id DESC);

CREATE SCHEMA IF NOT EXISTS authz;
CREATE TABLE IF NOT EXISTS authz.casbin_rule (
    id SERIAL PRIMARY KEY,
    ptype VARCHAR(100),
    v0 VARCHAR(100),
    v1 VARCHAR(100),
    v2 VARCHAR(100),
    v3 VARCHAR(100),
    v4 VARCHAR(100),
    v5 VARCHAR(100)
);

INSERT INTO gcfm_roles(id, name) VALUES (1,'admin') ON CONFLICT (id) DO NOTHING;
INSERT INTO gcfm_roles(id, name) VALUES (2,'editor') ON CONFLICT (id) DO NOTHING;
INSERT INTO gcfm_roles(id, name) VALUES (3,'viewer') ON CONFLICT (id) DO NOTHING;

INSERT INTO gcfm_users(id, tenant_id, username, password_hash) VALUES (1,'default','admin','$2a$12$m6067tTF2aFUNYum/PPEeONElY.Ohk34KWBrvCNcYzs5nB0j.L/N.')
ON CONFLICT (id) DO UPDATE SET password_hash=EXCLUDED.password_hash;

INSERT INTO gcfm_user_roles(user_id, role_id) VALUES (1,1) ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method) VALUES
    (1,'/v1/*','GET'),
    (1,'/v1/*','POST'),
    (1,'/v1/*','PUT'),
    (1,'/v1/*','DELETE')
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases/*/tables', 'GET'
  FROM gcfm_roles r WHERE r.name='admin'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/snapshots', 'GET'
  FROM gcfm_roles r WHERE r.name='admin'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/snapshots', 'POST'
  FROM gcfm_roles r WHERE r.name='admin'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/snapshots/{ver}/apply', 'POST'
  FROM gcfm_roles r WHERE r.name='admin'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases', 'GET'
  FROM gcfm_roles r WHERE r.name='admin'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases', 'POST'
  FROM gcfm_roles r WHERE r.name='admin'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases/{id}', 'PUT'
  FROM gcfm_roles r WHERE r.name='admin'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases/{id}', 'DELETE'
  FROM gcfm_roles r WHERE r.name='admin'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases/{id}/scan', 'POST'
  FROM gcfm_roles r WHERE r.name='admin'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/snapshots', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/metadata/tables', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases/{id}/scan', 'POST'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/databases', 'GET'
  FROM gcfm_roles r WHERE r.name='viewer'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/metadata/tables', 'GET'
  FROM gcfm_roles r WHERE r.name='viewer'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields', 'POST'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields', 'PUT'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields', 'DELETE'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields', 'GET'
  FROM gcfm_roles r WHERE r.name='viewer'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields/:name', 'PUT'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields/:name', 'DELETE'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO gcfm_role_policies(role_id, path, method)
SELECT r.id, '/v1/custom-fields/:name', 'GET'
  FROM gcfm_roles r WHERE r.name='editor'
ON CONFLICT DO NOTHING;

INSERT INTO authz.casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES
    ('p','admin','*','*','*','*','*'),
    ('g','admin','admin','','','','')
ON CONFLICT DO NOTHING;

-- targets configuration tables
CREATE TABLE IF NOT EXISTS gcfm_targets (
  key          TEXT PRIMARY KEY,
  driver       TEXT NOT NULL,
  dsn          TEXT NOT NULL,
  schema_name  TEXT DEFAULT '',
  max_open_conns INT DEFAULT 0,
  max_idle_conns INT DEFAULT 0,
  conn_max_idle_ms BIGINT DEFAULT 0,
  conn_max_life_ms BIGINT DEFAULT 0,
  is_default   BOOLEAN DEFAULT FALSE,
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS gcfm_target_labels (
  key    TEXT NOT NULL REFERENCES gcfm_targets(key) ON DELETE CASCADE,
  label  TEXT NOT NULL,
  PRIMARY KEY (key, label)
);

CREATE TABLE IF NOT EXISTS gcfm_target_config_version (
  id SMALLINT PRIMARY KEY DEFAULT 1,
  version TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO gcfm_target_config_version (id, version)
  VALUES (1, gen_random_uuid()::text)
ON CONFLICT (id) DO NOTHING;

CREATE UNIQUE INDEX IF NOT EXISTS gcfm_targets_one_default
  ON gcfm_targets (is_default) WHERE is_default = TRUE;

INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (1,'0.3');
