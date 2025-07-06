package unit_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/faciam-dev/gcfm/internal/customfield/migrator"
)

func TestMigratorUpDownTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	m := migrator.New("")
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS registry_schema_version").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT 1 FROM registry_schema_version WHERE version=0").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("^INSERT INTO registry_schema_version").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT MAX\\(version\\) FROM registry_schema_version").WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO gcfm_registry_schema_version").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := m.Up(context.Background(), db, 1); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS registry_schema_version").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT 1 FROM registry_schema_version WHERE version=0").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(1))
	mock.ExpectQuery("SELECT MAX\\(version\\) FROM registry_schema_version").WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectExec("DROP TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	if err := m.Down(context.Background(), db, 0); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
