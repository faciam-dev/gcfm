package handler

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"

	"github.com/faciam-dev/gcfm/internal/monitordb"
	"github.com/faciam-dev/gcfm/pkg/tenant"
)

func TestListHandlesPlainDSN(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	repo := &monitordb.Repo{DB: db, Dialect: ormdriver.MySQLDialect{}}

	rows := sqlmock.NewRows([]string{"id", "tenant_id", "name", "driver", "dsn", "dsn_enc", "created_at"}).
		AddRow(1, "t1", "db1", "mysql", "plain", []byte{}, time.Now())
	sqlStr, _, _ := query.New(db, "gcfm_monitored_databases", ormdriver.MySQLDialect{}).
		Select("id", "tenant_id", "name", "driver", "dsn", "dsn_enc", "created_at").
		Where("tenant_id", "t1").
		OrderBy("id", "asc").
		Build()
	mock.ExpectQuery(regexp.QuoteMeta(sqlStr)).WithArgs("t1").WillReturnRows(rows)

	h := &DatabaseHandler{Repo: repo}
	ctx := tenant.WithTenant(context.Background(), "t1")
	out, err := h.list(ctx, &struct{}{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out.Body) != 1 {
		t.Fatalf("expected 1 database, got %d", len(out.Body))
	}
	if out.Body[0].DSN != "" || out.Body[0].DSNEnc != "" {
		t.Fatalf("unexpected dsn fields: %#v", out.Body[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("db expectations: %v", err)
	}
}
