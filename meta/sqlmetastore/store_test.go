package sqlmetastore

import (
	"context"
	"database/sql"
	"testing"
	"time"

	metapkg "github.com/faciam-dev/gcfm/meta"
	_ "github.com/mattn/go-sqlite3"
)

func newTestStore(t *testing.T) *SQLMetaStore {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	schema := `CREATE TABLE gcfm_custom_fields (
        db_id INTEGER,
        tenant_id TEXT DEFAULT 'default',
        table_name TEXT,
        column_name TEXT,
        data_type TEXT,
        label_key TEXT,
        widget TEXT,
        placeholder_key TEXT,
        nullable BOOLEAN,
        "unique" BOOLEAN,
        has_default BOOLEAN,
        default_value TEXT,
        validator TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (db_id, tenant_id, table_name, column_name)
    );
    CREATE TABLE gcfm_scan_results (
        tenant_id TEXT,
        scan_id TEXT,
        status TEXT,
        started_at TIMESTAMP,
        finished_at TIMESTAMP,
        details TEXT
    );
    CREATE TABLE gcfm_targets (
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
    CREATE TABLE gcfm_target_labels (
        key TEXT NOT NULL,
        label TEXT NOT NULL,
        PRIMARY KEY (key, label),
        FOREIGN KEY (key) REFERENCES gcfm_targets(key) ON DELETE CASCADE
    );
    CREATE TABLE gcfm_target_config_version (
        id INTEGER PRIMARY KEY,
        version TEXT NOT NULL,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    INSERT INTO gcfm_target_config_version(id, version) VALUES (1, 'init');
    CREATE UNIQUE INDEX gcfm_targets_one_default ON gcfm_targets(is_default) WHERE is_default;
    `
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	return NewSQLMetaStore(db, "sqlite3", "")
}

func TestSQLMetaStore_BeginTx(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	tx, err := store.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	_ = tx.Rollback()
}

func TestSQLMetaStore_UpsertAndList(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	def := metapkg.FieldDef{DBID: 1, TableName: "posts", ColumnName: "title", DataType: "text"}
	if err := store.UpsertFieldDefs(ctx, nil, []metapkg.FieldDef{def}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	def.DataType = "varchar"
	if err := store.UpsertFieldDefs(ctx, nil, []metapkg.FieldDef{def}); err != nil {
		t.Fatalf("Upsert update: %v", err)
	}
	defs, err := store.ListFieldDefs(ctx, "default")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].DataType != "varchar" {
		t.Fatalf("expected varchar, got %s", defs[0].DataType)
	}
}

func TestSQLMetaStore_RecordScanResult(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	res := metapkg.ScanResult{TenantID: "t1", ScanID: "s1", Status: "done", StartedAt: time.Now(), FinishedAt: time.Now(), Details: "{}"}
	if err := store.RecordScanResult(ctx, nil, res); err != nil {
		t.Fatalf("RecordScanResult: %v", err)
	}
	var cnt int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM gcfm_scan_results`).Scan(&cnt); err != nil {
		t.Fatalf("query: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected 1 row, got %d", cnt)
	}
}

func TestSQLMetaStore_Targets(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	rowA := metapkg.TargetRow{Key: "a", Driver: "sqlite3", DSN: "dsnA"}
	if err := store.UpsertTarget(ctx, nil, rowA, []string{"region=tokyo"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	rowB := metapkg.TargetRow{Key: "b", Driver: "sqlite3", DSN: "dsnB"}
	if err := store.UpsertTarget(ctx, nil, rowB, nil); err != nil {
		t.Fatalf("upsert b: %v", err)
	}
	if err := store.SetDefaultTarget(ctx, nil, "a"); err != nil {
		t.Fatalf("set default: %v", err)
	}
	ver1, err := store.BumpTargetsVersion(ctx, nil)
	if err != nil {
		t.Fatalf("bump version: %v", err)
	}
	rows, ver, def, err := store.ListTargets(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if ver != ver1 {
		t.Fatalf("version mismatch: %s vs %s", ver, ver1)
	}
	if def != "a" {
		t.Fatalf("default key: want a got %s", def)
	}
	if len(rows[0].Labels) == 0 {
		t.Fatalf("labels not loaded")
	}
	if err := store.DeleteTarget(ctx, nil, "b"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	ver2, err := store.BumpTargetsVersion(ctx, nil)
	if err != nil {
		t.Fatalf("bump2: %v", err)
	}
	if ver1 == ver2 {
		t.Fatalf("version not changed")
	}
	rows, _, _, err = store.ListTargets(ctx)
	if err != nil {
		t.Fatalf("list2: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
}
