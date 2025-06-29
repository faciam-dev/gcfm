package unit_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/faciam-dev/gcfm/internal/customfield/migrator"
)

func TestMigratorUpDownTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	m := migrator.New()
	mock.ExpectQuery("SELECT MAX\\(version\\)").WillReturnError(fmt.Errorf("no such table"))
	mock.ExpectBegin()
	mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO registry_schema_version").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := m.Up(context.Background(), db, 1); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}

	mock.ExpectQuery("SELECT MAX\\(version\\)").WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(1))
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
