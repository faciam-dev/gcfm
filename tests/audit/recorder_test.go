package audit_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
)

func TestRecorderWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	rec := &audit.Recorder{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
	old := &registry.FieldMeta{TableName: "posts", ColumnName: "title", DataType: "text"}
	newm := &registry.FieldMeta{TableName: "posts", ColumnName: "title", DataType: "varchar"}
	mock.ExpectExec("INSERT INTO .*audit_logs").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := rec.Write(context.Background(), "alice", old, newm); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestRecorderWriteAction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	rec := &audit.Recorder{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
	mock.ExpectExec("INSERT INTO .*audit_logs").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := rec.WriteAction(context.Background(), "bob", "snapshot", "1.0.0", "+1 -0"); err != nil {
		t.Fatalf("write action: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
