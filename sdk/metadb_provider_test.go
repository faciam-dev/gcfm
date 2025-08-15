package sdk

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	metapkg "github.com/faciam-dev/gcfm/meta"
	"github.com/faciam-dev/gcfm/meta/sqlmetastore"
	_ "github.com/mattn/go-sqlite3"
)

func newTargetStore(t *testing.T) *sqlmetastore.SQLMetaStore {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	schema := `CREATE TABLE IF NOT EXISTS gcfm_targets (
        key TEXT PRIMARY KEY,
        driver TEXT NOT NULL,
        dsn TEXT NOT NULL,
        schema_name TEXT DEFAULT '',
        max_open_conns INT DEFAULT 0,
        max_idle_conns INT DEFAULT 0,
        conn_max_idle_ms BIGINT DEFAULT 0,
        conn_max_life_ms BIGINT DEFAULT 0,
        is_default BOOLEAN DEFAULT FALSE,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS gcfm_target_labels (
        key TEXT NOT NULL,
        label TEXT NOT NULL,
        PRIMARY KEY (key, label),
        FOREIGN KEY (key) REFERENCES gcfm_targets(key) ON DELETE CASCADE
    );
    CREATE TABLE IF NOT EXISTS gcfm_target_config_version (
        id INTEGER PRIMARY KEY,
        version TEXT NOT NULL,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    INSERT OR IGNORE INTO gcfm_target_config_version(id, version) VALUES (1, 'init');
    `
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	return sqlmetastore.NewSQLMetaStore(db, "sqlite3", "")
}

func TestMetaDBProvider_Fetch(t *testing.T) {
	store := newTargetStore(t)
	ctx := context.Background()
	if err := store.UpsertTarget(ctx, nil, metapkg.TargetRow{Key: "a", Driver: "sqlite3", DSN: "dsnA", IsDefault: true}, []string{"r=1"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := store.BumpTargetsVersion(ctx, nil); err != nil {
		t.Fatalf("bump: %v", err)
	}
	p := NewMetaDBProvider(store)
	cfgs, def, ver, err := p.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if def != "a" {
		t.Fatalf("default key: %s", def)
	}
	if len(cfgs) != 1 {
		t.Fatalf("cfg count: %d", len(cfgs))
	}
	if ver == "" {
		t.Fatalf("version empty")
	}
}
