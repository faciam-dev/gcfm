package sdk

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func createTable(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	if _, err := db.Exec("CREATE TABLE " + name + " (id INTEGER)"); err != nil {
		t.Fatalf("create table %s: %v", name, err)
	}
}

func createScanResultTable(t *testing.T, db *sql.DB) {
	t.Helper()
	schema := `CREATE TABLE gcfm_scan_results (
        tenant_id TEXT,
        scan_id TEXT,
        status TEXT,
        started_at TIMESTAMP,
        finished_at TIMESTAMP,
        details TEXT
    );`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create meta table: %v", err)
	}
}

func TestNightlyScan(t *testing.T) {
	metaDB, _ := sql.Open("sqlite3", ":memory:")
	createScanResultTable(t, metaDB)
	targetA, _ := sql.Open("sqlite3", ":memory:")
	targetB, _ := sql.Open("sqlite3", ":memory:")
	createTable(t, targetA, "a1")
	createTable(t, targetB, "b1")

	svc := New(ServiceConfig{
		MetaDB:     metaDB,
		MetaDriver: "sqlite3",
		Targets: []TargetConfig{
			{Key: "tenant:A", DB: targetA, Driver: "sqlite3"},
			{Key: "tenant:B", DB: targetB, Driver: "sqlite3"},
		},
	}).(*service)

	if err := svc.NightlyScan(context.Background()); err != nil {
		t.Fatalf("NightlyScan: %v", err)
	}
	var cntA, cntB int
	if err := metaDB.QueryRow("SELECT COUNT(*) FROM gcfm_scan_results WHERE tenant_id='tenant:A'").Scan(&cntA); err != nil {
		t.Fatalf("cntA: %v", err)
	}
	if err := metaDB.QueryRow("SELECT COUNT(*) FROM gcfm_scan_results WHERE tenant_id='tenant:B'").Scan(&cntB); err != nil {
		t.Fatalf("cntB: %v", err)
	}
	if cntA != 1 || cntB != 1 {
		t.Fatalf("expected 1 scan result per tenant, got %d and %d", cntA, cntB)
	}
}
