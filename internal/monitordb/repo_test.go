package monitordb

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

func TestListParsesCreatedAt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	r := &Repo{DB: db, Dialect: ormdriver.MySQLDialect{}, Driver: "mysql"}
	rows := sqlmock.NewRows([]string{"id", "tenant_id", "name", "driver", "dsn", "dsn_enc", "created_at"}).
		AddRow(1, "t1", "db1", "mysql", "plain", []byte("enc"), time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC))

	sqlStr, _, _ := query.New(db, "gcfm_monitored_databases", ormdriver.MySQLDialect{}).
		Select("id", "tenant_id", "name", "driver", "dsn", "dsn_enc", "created_at").
		Where("tenant_id", "t1").
		OrderBy("id", "asc").
		Build()
	mock.ExpectQuery(regexp.QuoteMeta(sqlStr)).
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
