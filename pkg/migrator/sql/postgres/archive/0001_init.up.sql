-- 初期スキーマ（0015/0016 と衝突しないよう設計）
CREATE TABLE IF NOT EXISTS gcfm_custom_fields (
  table_name  VARCHAR(255) NOT NULL,
  column_name VARCHAR(255) NOT NULL,
  data_type   VARCHAR(255) NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (table_name, column_name)
);

-- users（role 列あり = 0015 で DROP される前提）
CREATE TABLE IF NOT EXISTS gcfm_users (
  id            BIGSERIAL PRIMARY KEY,
  username      TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  role          TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- UNIQUE(username) を constraint 名付きで作る（0015 で DROP できるように）
ALTER TABLE gcfm_users
  ADD CONSTRAINT gcfm_users_username_key UNIQUE (username);

-- roles
CREATE TABLE IF NOT EXISTS gcfm_roles (
  id   BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);

-- user_roles（0015 では tenant_id を持たない想定で OK）
CREATE TABLE IF NOT EXISTS gcfm_user_roles (
  user_id BIGINT NOT NULL REFERENCES gcfm_users(id) ON DELETE CASCADE,
  role_id BIGINT NOT NULL REFERENCES gcfm_roles(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, role_id)
);

-- 監査ログ（0016 で件数カラムが追加される）
CREATE TABLE IF NOT EXISTS gcfm_audit_logs (
  id           BIGSERIAL PRIMARY KEY,
  tenant_id    TEXT NOT NULL DEFAULT 'default',
  actor        TEXT NOT NULL,
  action       TEXT NOT NULL,
  table_name   TEXT NOT NULL,
  column_name  TEXT NOT NULL,
  before_json  JSONB,
  after_json   JSONB,
  applied_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- レジストリ（バージョン管理）
CREATE TABLE IF NOT EXISTS gcfm_registry_schema_version (
  id BIGSERIAL PRIMARY KEY,
  version  INT  NOT NULL,
  semver   TEXT NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
