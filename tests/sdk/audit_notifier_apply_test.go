package sdk_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	"github.com/faciam-dev/gcfm/internal/customfield/notifier"
	sdk "github.com/faciam-dev/gcfm/sdk"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

type stubNotifier struct{ diffs []notifier.DiffReport }

func (s *stubNotifier) Emit(ctx context.Context, d notifier.DiffReport) error {
	s.diffs = append(s.diffs, d)
	return nil
}

func TestApplyHooks(t *testing.T) {
	db, mock, err := sqlmock.NewWithDSN("sqlmock_db")
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	t.Setenv("CF_ENC_KEY", "0123456789abcdef0123456789abcdef")

	mock.ExpectQuery("SELECT .* FROM .*custom_fields").WillReturnRows(sqlmock.NewRows([]string{"db_id", "table_name", "column_name", "data_type", "label_key", "widget", "widget_config", "placeholder_key", "nullable", "unique", "has_default", "default_value", "validator"}))
	sqlStr, _, _ := query.New(db, "gcfm_monitored_databases", ormdriver.MySQLDialect{}).
		Select("id").
		Where("id", 1).
		Where("tenant_id", "default").
		Build()
	mock.ExpectQuery(regexp.QuoteMeta(sqlStr)).WithArgs(1, "default").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec("INSERT IGNORE INTO `gcfm_monitored_databases`").WithArgs(1, "default", "db_1", "mysql", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO gcfm_custom_fields").ExpectExec().WithArgs(
		1,
		"posts", "title", "text",
		sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		sqlmock.AnyArg(),
	).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	nt := &stubNotifier{}
	disable := false
	svc := sdk.New(sdk.ServiceConfig{Recorder: &audit.Recorder{DB: db, Dialect: ormdriver.MySQLDialect{}}, Notifier: nt, PluginEnabled: &disable})
	yamlData := []byte("version: 0.4\nfields:\n  - table: posts\n    column: title\n    type: text\n")
	rep, err := svc.Apply(context.Background(), sdk.DBConfig{Driver: "sqlmock", DSN: "sqlmock_db", TablePrefix: "gcfm_"}, yamlData, sdk.ApplyOptions{Actor: "alice"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if rep.Added != 1 {
		t.Fatalf("diff added=%d", rep.Added)
	}
	if len(nt.diffs) != 1 || nt.diffs[0].Added != 1 {
		t.Fatalf("notifier called: %#v", nt.diffs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("db expectations: %v", err)
	}
}

func TestApplyHooksDryRun(t *testing.T) {
	db, mock, err := sqlmock.NewWithDSN("sqlmock_db2")
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	t.Setenv("CF_ENC_KEY", "0123456789abcdef0123456789abcdef")
	mock.ExpectQuery("SELECT .* FROM .*custom_fields").WillReturnRows(sqlmock.NewRows([]string{"db_id", "table_name", "column_name", "data_type", "label_key", "widget", "widget_config", "placeholder_key", "nullable", "unique", "has_default", "default_value", "validator"}))

	nt := &stubNotifier{}
	disable := false
	svc := sdk.New(sdk.ServiceConfig{Recorder: &audit.Recorder{DB: db, Dialect: ormdriver.MySQLDialect{}}, Notifier: nt, PluginEnabled: &disable})
	yamlData := []byte("version: 0.4\nfields:\n  - table: posts\n    column: title\n    type: text\n")
	_, err = svc.Apply(context.Background(), sdk.DBConfig{Driver: "sqlmock", DSN: "sqlmock_db2", TablePrefix: "gcfm_"}, yamlData, sdk.ApplyOptions{Actor: "alice", DryRun: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(nt.diffs) != 0 {
		t.Fatalf("notifier called: %#v", nt.diffs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("db expectations: %v", err)
	}
}
