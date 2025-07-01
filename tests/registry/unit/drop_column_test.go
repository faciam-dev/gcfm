package unit_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

func TestDropColumnSQL_MySQLColumnExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM INFORMATION_SCHEMA.COLUMNS").
		WithArgs("posts", "age").
		WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(1))
	mock.ExpectExec("ALTER TABLE `posts` DROP COLUMN `age`").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := registry.DropColumnSQL(context.Background(), db, "mysql", "posts", "age"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestDropColumnSQL_MySQLColumnNotExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM INFORMATION_SCHEMA.COLUMNS").
		WithArgs("posts", "age").
		WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(0))

	if err := registry.DropColumnSQL(context.Background(), db, "mysql", "posts", "age"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
