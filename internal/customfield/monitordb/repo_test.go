package monitordb

import (
	"context"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

func TestEnsureExistsInsertsPlaceholder(t *testing.T) {
	t.Setenv("CF_ENC_KEY", "0123456789abcdef0123456789abcdef")
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	sqlStr, _, _ := query.New(db, "gcfm_monitored_databases", ormdriver.MySQLDialect{}).
		Select("id").
		Where("id", int64(1)).
		Where("tenant_id", "default").
		Build()
	mock.ExpectQuery(regexp.QuoteMeta(sqlStr)).
		WithArgs(int64(1), "default").
		WillReturnRows(sqlmock.NewRows([]string{}))

	mock.ExpectExec(regexp.QuoteMeta("INSERT IGNORE INTO gcfm_monitored_databases (id, tenant_id, name, driver, dsn, dsn_enc) VALUES (?,?,?,?, '', ?)")).
		WithArgs(int64(1), "default", "db_1", "mysql", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := EnsureExists(context.Background(), db, ormdriver.MySQLDialect{}, "", "default", 1); err != nil {
		t.Fatalf("EnsureExists: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
