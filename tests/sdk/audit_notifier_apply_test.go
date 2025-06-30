package sdk_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	"github.com/faciam-dev/gcfm/internal/customfield/notifier"
	sdk "github.com/faciam-dev/gcfm/sdk"
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

	mock.ExpectQuery("^SELECT table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, `unique`, `default`, validator FROM custom_fields ORDER BY table_name, column_name$").WillReturnRows(sqlmock.NewRows([]string{"table_name", "column_name", "data_type", "label_key", "widget", "placeholder_key", "nullable", "unique", "default", "validator"}))
	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO custom_fields").ExpectExec().WithArgs(
		sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
	).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	nt := &stubNotifier{}
	disable := false
	svc := sdk.New(sdk.ServiceConfig{Recorder: &audit.Recorder{DB: db, Driver: "mysql"}, Notifier: nt, PluginEnabled: &disable})
	yamlData := []byte("version: 0.4\nfields:\n  - table: posts\n    column: title\n    type: text\n")
	rep, err := svc.Apply(context.Background(), sdk.DBConfig{Driver: "sqlmock", DSN: "sqlmock_db"}, yamlData, sdk.ApplyOptions{Actor: "alice"})
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

	mock.ExpectQuery("^SELECT table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, `unique`, `default`, validator FROM custom_fields ORDER BY table_name, column_name$").WillReturnRows(sqlmock.NewRows([]string{"table_name", "column_name", "data_type", "label_key", "widget", "placeholder_key", "nullable", "unique", "default", "validator"}))

	nt := &stubNotifier{}
	disable := false
	svc := sdk.New(sdk.ServiceConfig{Recorder: &audit.Recorder{DB: db, Driver: "mysql"}, Notifier: nt, PluginEnabled: &disable})
	yamlData := []byte("version: 0.4\nfields:\n  - table: posts\n    column: title\n    type: text\n")
	_, err = svc.Apply(context.Background(), sdk.DBConfig{Driver: "sqlmock", DSN: "sqlmock_db2"}, yamlData, sdk.ApplyOptions{Actor: "alice", DryRun: true})
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
