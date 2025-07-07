package unit_test

import (
	"context"
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
	for i := 0; i < 3; i++ {
		mock.ExpectExec(".*").WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectQuery("SELECT MAX\\(version\\)").WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(nil))
	mock.ExpectBegin()
	for i := 0; i < 4; i++ {
		mock.ExpectExec(".*").WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectCommit()
	if err := m.Up(context.Background(), db, 1); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}

	for i := 0; i < 2; i++ {
		mock.ExpectExec(".*").WillReturnResult(sqlmock.NewResult(0, 0))
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
