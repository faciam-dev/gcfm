package monitordb

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestListParsesCreatedAt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	r := &Repo{DB: db, Driver: "mysql"}
	rows := sqlmock.NewRows([]string{"id", "tenant_id", "name", "driver", "dsn_enc", "created_at"}).
		AddRow(1, "t1", "db1", "mysql", []byte("enc"), []byte("2024-01-02 03:04:05"))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_id, name, driver, dsn_enc, created_at FROM gcfm_monitored_databases WHERE tenant_id=? ORDER BY id")).
		WithArgs("t1").WillReturnRows(rows)

	dbs, err := r.List(context.Background(), "t1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(dbs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(dbs))
	}
	if dbs[0].CreatedAt.IsZero() {
		t.Fatalf("expected CreatedAt to be set")
	}
}

func TestParseSQLTime(t *testing.T) {
	if _, err := parseSQLTime([]byte("2024-01-02 03:04:05")); err != nil {
		t.Fatalf("parseSQLTime: %v", err)
	}
}
