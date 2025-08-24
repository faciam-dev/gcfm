package sdk

import (
	"context"
	"database/sql"
	"testing"

	metapkg "github.com/faciam-dev/gcfm/meta"
	_ "github.com/mattn/go-sqlite3"
)

func createMetaTable(t *testing.T, db *sql.DB) {
	t.Helper()
	schema := `CREATE TABLE gcfm_custom_fields (
        db_id INTEGER,
        tenant_id TEXT DEFAULT 'default',
        table_name TEXT,
        column_name TEXT,
        data_type TEXT,
        label_key TEXT,
        widget TEXT,
        widget_config TEXT,
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
}

func TestServiceMetaSeparation(t *testing.T) {
	metaDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	targetDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	createMetaTable(t, metaDB)
	createMetaTable(t, targetDB)
	svc := New(ServiceConfig{
		DB:         targetDB,
		Driver:     "sqlite3",
		MetaDB:     metaDB,
		MetaDriver: "sqlite3",
	}).(*service)
	ctx := context.Background()
	def := metapkg.FieldDef{DBID: 1, TableName: "posts", ColumnName: "title", DataType: "text"}
	if err := svc.meta.UpsertFieldDefs(ctx, nil, []metapkg.FieldDef{def}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	var metaCount, targetCount int
	if err := metaDB.QueryRow("SELECT COUNT(*) FROM gcfm_custom_fields").Scan(&metaCount); err != nil {
		t.Fatal(err)
	}
	if err := targetDB.QueryRow("SELECT COUNT(*) FROM gcfm_custom_fields").Scan(&targetCount); err != nil {
		t.Fatal(err)
	}
	if metaCount != 1 || targetCount != 0 {
		t.Fatalf("expected meta=1 target=0 got %d %d", metaCount, targetCount)
	}
}

func TestServiceMetaDefaultFallback(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	createMetaTable(t, db)
	svc := New(ServiceConfig{DB: db, Driver: "sqlite3"}).(*service)
	ctx := context.Background()
	def := metapkg.FieldDef{DBID: 1, TableName: "posts", ColumnName: "title", DataType: "text"}
	if err := svc.meta.UpsertFieldDefs(ctx, nil, []metapkg.FieldDef{def}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM gcfm_custom_fields").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}
}
