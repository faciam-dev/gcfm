package sqlmetastore

import (
	"context"
	"database/sql"
	"testing"

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
    );`
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
