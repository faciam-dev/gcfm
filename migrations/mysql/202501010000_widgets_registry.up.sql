-- Widgets Registry (DB-backed source of truth)
CREATE TABLE IF NOT EXISTS gcfm_widgets (
  id            VARCHAR(255) PRIMARY KEY,
  name          TEXT NOT NULL,
  version       VARCHAR(64) NOT NULL,
  type          VARCHAR(32) NOT NULL DEFAULT 'widget',
  scopes        JSON NOT NULL DEFAULT '["system"]',
  enabled       BOOLEAN NOT NULL DEFAULT TRUE,
  description   TEXT,
  capabilities  JSON NOT NULL DEFAULT '[]',
  homepage      TEXT,
  meta          JSON NOT NULL DEFAULT '{}',
  tenant_scope  VARCHAR(16) NOT NULL DEFAULT 'system',
  tenants       JSON NOT NULL DEFAULT '[]',
  updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE INDEX gcfm_widgets_updated_at_idx ON gcfm_widgets (updated_at);
CREATE INDEX gcfm_widgets_tenant_scope_idx ON gcfm_widgets (tenant_scope);
