package sdk

import (
	"context"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/customfield/registry/codec"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

func TestApplyEnsuresMonitoredDB(t *testing.T) {
	t.Setenv("CF_ENC_KEY", "0123456789abcdef0123456789abcdef")
	db, mock, err := sqlmock.NewWithDSN("apply_dsn")
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	scanSQL, _, _ := query.New(db, "gcfm_custom_fields", ormdriver.MySQLDialect{}).
		Select("db_id", "table_name", "column_name", "data_type", "label_key", "widget", "widget_config", "placeholder_key", "nullable", "unique", "has_default", "default_value", "validator").
		OrderByRaw("table_name, column_name").
		Build()
	mock.ExpectQuery(regexp.QuoteMeta(scanSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"db_id", "table_name", "column_name", "data_type", "label_key", "widget", "widget_config", "placeholder_key", "nullable", "unique", "has_default", "default_value", "validator"}))

	selSQL, _, _ := query.New(db, "gcfm_monitored_databases", ormdriver.MySQLDialect{}).
		Select("id").
		Where("id", int64(1)).
		Where("tenant_id", "default").
		Build()
	mock.ExpectQuery(regexp.QuoteMeta(selSQL)).
		WithArgs(int64(1), "default").
		WillReturnRows(sqlmock.NewRows([]string{}))

	mock.ExpectExec(regexp.QuoteMeta("INSERT IGNORE INTO `gcfm_monitored_databases` (id, tenant_id, name, driver, dsn, dsn_enc) VALUES (?,?,?,?, '', ?)")).
		WithArgs(int64(1), "default", "db_1", "mysql", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectBegin()
	prep := mock.ExpectPrepare(regexp.QuoteMeta("INSERT INTO gcfm_custom_fields"))
	prep.ExpectExec().
		WithArgs(int64(1), "posts", "cf1", "text", "", "text", "", "", false, false, false, "", "").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	svc := New(ServiceConfig{})
	data, err := codec.EncodeYAML([]registry.FieldMeta{{TableName: "posts", ColumnName: "cf1", DataType: "text"}})
	if err != nil {
		t.Fatalf("EncodeYAML: %v", err)
	}
	if _, err := svc.Apply(context.Background(), DBConfig{Driver: "sqlmock", DSN: "apply_dsn", TablePrefix: "gcfm_"}, data, ApplyOptions{}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
