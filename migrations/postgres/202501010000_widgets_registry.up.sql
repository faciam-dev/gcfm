CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Widgets Registry (DB-backed source of truth)
CREATE TABLE IF NOT EXISTS gcfm_widgets (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  version       TEXT NOT NULL,
  type          TEXT NOT NULL DEFAULT 'widget',
  scopes        TEXT[] NOT NULL DEFAULT ARRAY['system'],
  enabled       BOOLEAN NOT NULL DEFAULT TRUE,
  description   TEXT,
  capabilities  TEXT[] NOT NULL DEFAULT '{}',
  homepage      TEXT,
  meta          JSONB NOT NULL DEFAULT '{}'::jsonb,
  tenant_scope  TEXT NOT NULL DEFAULT 'system',
  tenants       TEXT[] NOT NULL DEFAULT '{}',
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS gcfm_widgets_updated_at_idx ON gcfm_widgets (updated_at DESC);
CREATE INDEX IF NOT EXISTS gcfm_widgets_tenant_scope_idx ON gcfm_widgets (tenant_scope);
CREATE INDEX IF NOT EXISTS gcfm_widgets_scopes_gin_idx ON gcfm_widgets USING GIN (scopes);
CREATE INDEX IF NOT EXISTS gcfm_widgets_capabilities_gin_idx ON gcfm_widgets USING GIN (capabilities);

CREATE OR REPLACE FUNCTION notify_widgets_changed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  PERFORM pg_notify('widgets_changed', COALESCE(NEW.id, OLD.id));
  RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS gcfm_widgets_notify_iud ON gcfm_widgets;
CREATE TRIGGER gcfm_widgets_notify_iud
AFTER INSERT OR UPDATE OR DELETE ON gcfm_widgets
FOR EACH ROW EXECUTE FUNCTION notify_widgets_changed();
