-- 既存環境と CI の両方で安全に通るように IF/EXISTS を付ける

-- ① tenant_id 追加（既にある環境もある）
ALTER TABLE gcfm_users
  ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';

-- ② UNIQUE(username) を外す（ない環境でも OK）
ALTER TABLE gcfm_users DROP CONSTRAINT IF EXISTS gcfm_users_username_key;

-- ③ UNIQUE(tenant_id, username) を張る（重複作成を回避）
ALTER TABLE gcfm_users
  ADD CONSTRAINT gcfm_users_tenant_username_key UNIQUE (tenant_id, username);

-- ④ 旧 role 列を user_roles に移す（列が無ければスキップ）
INSERT INTO gcfm_user_roles(user_id, role_id)
  SELECT u.id, r.id
    FROM gcfm_users u
    JOIN gcfm_roles r ON r.name = u.role
  ON CONFLICT DO NOTHING;

ALTER TABLE gcfm_users DROP COLUMN IF EXISTS role;

-- ⑤ レジストリ（存在する場合のみ追記）
INSERT INTO gcfm_registry_schema_version(version, semver)
  VALUES (15, '0.15')
ON CONFLICT DO NOTHING;
