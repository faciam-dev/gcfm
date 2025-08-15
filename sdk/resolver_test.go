package sdk

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func createCFTable(t *testing.T, db *sql.DB) {
	t.Helper()
	schema := `CREATE TABLE custom_fields (
       db_id INTEGER,
       tenant_id TEXT,
       table_name TEXT,
       column_name TEXT,
       data_type TEXT,
       label_key TEXT,
       widget TEXT,
       placeholder_key TEXT,
       nullable BOOLEAN NOT NULL DEFAULT 0,
       "unique" BOOLEAN NOT NULL DEFAULT 0,
       has_default BOOLEAN NOT NULL DEFAULT 0,
       default_value TEXT,
       validator TEXT
   );`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create table: %v", err)
	}
}

func TestTenantResolver(t *testing.T) {
	aDB, _ := sql.Open("sqlite3", ":memory:")
	bDB, _ := sql.Open("sqlite3", ":memory:")
	createCFTable(t, aDB)
	createCFTable(t, bDB)
	// insert different rows
	aDB.Exec(`INSERT INTO custom_fields (db_id, tenant_id, table_name, column_name, data_type, nullable, "unique", has_default) VALUES (1, 'default', 'posts', 'a_col', 'text', 1, 0, 0)`)
	bDB.Exec(`INSERT INTO custom_fields (db_id, tenant_id, table_name, column_name, data_type, nullable, "unique", has_default) VALUES (1, 'default', 'posts', 'b_col', 'text', 1, 0, 0)`)

	svc := New(ServiceConfig{
		Targets: []TargetConfig{
			{Key: "tenant:a", DB: aDB, Driver: "sqlite3"},
			{Key: "tenant:b", DB: bDB, Driver: "sqlite3"},
		},
		TargetResolver: TenantResolverFromPrefix("tenant:"),
	}).(*service)

	ctxA := WithTenantID(context.Background(), "a")
	metasA, err := svc.ListCustomFields(ctxA, 1, "")
	if err != nil {
		t.Fatalf("list a: %v", err)
	}
	if len(metasA) != 1 || metasA[0].ColumnName != "a_col" {
		t.Fatalf("unexpected metasA: %+v", metasA)
	}

	ctxB := WithTenantID(context.Background(), "b")
	metasB, err := svc.ListCustomFields(ctxB, 1, "")
	if err != nil {
		t.Fatalf("list b: %v", err)
	}
	if len(metasB) != 1 || metasB[0].ColumnName != "b_col" {
		t.Fatalf("unexpected metasB: %+v", metasB)
	}
}

func TestPickTargetFallback(t *testing.T) {
	db, _ := sql.Open("sqlite3", ":memory:")
	svc := New(ServiceConfig{DB: db, Driver: "sqlite3", TargetResolver: func(ctx context.Context) (string, bool) {
		return "missing", true
	}}).(*service)
	if _, err := svc.pickTarget(context.Background()); err != nil {
		t.Fatalf("fallback to default failed: %v", err)
	}

	svc = New(ServiceConfig{TargetResolver: func(ctx context.Context) (string, bool) {
		return "", false
	}}).(*service)
	if _, err := svc.pickTarget(context.Background()); err == nil {
		t.Fatalf("expected error when no target")
	}
}
