package sdk

import (
	"context"
	"regexp"
	"testing"

	"database/sql"
	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/faciam-dev/gcfm/meta"
	"go.uber.org/zap"
)

// stubMeta implements meta.MetaStore with only ListFieldDefs used in tests.
type stubMeta struct{ defs []FieldDef }

func (m *stubMeta) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)              { return nil, nil }
func (m *stubMeta) UpsertFieldDefs(context.Context, *sql.Tx, []FieldDef) error            { return nil }
func (m *stubMeta) DeleteFieldDefs(context.Context, *sql.Tx, []FieldDef) error            { return nil }
func (m *stubMeta) ListFieldDefs(context.Context, string) ([]FieldDef, error)             { return m.defs, nil }
func (m *stubMeta) RecordScanResult(context.Context, *sql.Tx, meta.ScanResult) error      { return nil }
func (m *stubMeta) UpsertTarget(context.Context, *sql.Tx, meta.TargetRow, []string) error { return nil }
func (m *stubMeta) DeleteTarget(context.Context, *sql.Tx, string) error                   { return nil }
func (m *stubMeta) ListTargets(context.Context) ([]meta.TargetRowWithLabels, string, string, error) {
	return nil, "", "", nil
}
func (m *stubMeta) SetDefaultTarget(context.Context, *sql.Tx, string) error     { return nil }
func (m *stubMeta) BumpTargetsVersion(context.Context, *sql.Tx) (string, error) { return "", nil }

// TestListCustomFieldsMeta verifies ReadFromMeta returns definitions without hitting target DB.
func TestListCustomFieldsMeta(t *testing.T) {
	svc := &service{
		logger:     zap.NewNop().Sugar(),
		meta:       &stubMeta{defs: []FieldDef{{DBID: 1, TableName: "posts", ColumnName: "cf1", DataType: "text"}}},
		targets:    NewHotReloadRegistry(nil),
		readSource: ReadFromMeta,
	}
	defs, err := svc.ListCustomFields(context.Background(), 1, "posts")
	if err != nil || len(defs) != 1 || defs[0].ColumnName != "cf1" {
		t.Fatalf("unexpected defs: %+v, err: %v", defs, err)
	}
}

// TestReconcileRepair ensures missing fields are detected and inserted into the target.
func TestReconcileRepair(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	query := "SELECT db_id, table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, `unique`, has_default, default_value, validator FROM gcfm_custom_fields WHERE tenant_id=? AND db_id=? ORDER BY table_name, column_name"
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs("default", int64(1)).WillReturnRows(sqlmock.NewRows([]string{"db_id", "table_name", "column_name", "data_type", "label_key", "widget", "placeholder_key", "nullable", "unique", "has_default", "default_value", "validator"}))

	mock.ExpectBegin()
	mock.ExpectPrepare(regexp.QuoteMeta("INSERT INTO gcfm_custom_fields"))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO gcfm_custom_fields")).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	reg := NewHotReloadRegistry(&TargetConn{DB: db, Driver: "mysql", Schema: "", Dialect: driverDialect("mysql")})
	svc := &service{
		logger:  zap.NewNop().Sugar(),
		meta:    &stubMeta{defs: []FieldDef{{DBID: 1, TableName: "posts", ColumnName: "cf1", DataType: "text"}}},
		targets: reg,
	}

	rep, err := svc.ReconcileCustomFields(context.Background(), 1, "posts", true)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(rep.MissingInTarget) != 1 || rep.MissingInTarget[0].ColumnName != "cf1" {
		t.Fatalf("unexpected report: %+v", rep)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock: %v", err)
	}
}

// TestDiffFieldsDuplicateTarget ensures duplicate target definitions are all reported when mismatched.
func TestDiffFieldsDuplicateTarget(t *testing.T) {
	meta := []FieldDef{{DBID: 1, TableName: "posts", ColumnName: "cf1", DataType: "text"}}
	tgt := []FieldDef{
		{DBID: 1, TableName: "posts", ColumnName: "cf1", DataType: "int"},
		{DBID: 1, TableName: "posts", ColumnName: "cf1", DataType: "bool"},
	}
	rep := diffFields(meta, tgt)
	if len(rep.Mismatched) != 2 {
		t.Fatalf("expected 2 mismatches, got %d", len(rep.Mismatched))
	}
	if len(rep.MissingInMeta) != 0 || len(rep.MissingInTarget) != 0 {
		t.Fatalf("unexpected report: %+v", rep)
	}
}
