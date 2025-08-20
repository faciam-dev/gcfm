package registry

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
)

func TestColumnExistsMySQLUsesDatabaseFunction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*table_schema = DATABASE\\(\\)").
		WithArgs("my_table", "my_column").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))

	exists, err := ColumnExists(context.Background(), db, ormdriver.MySQLDialect{}, "", "my_table", "my_column")
	if err != nil {
		t.Fatalf("ColumnExists returned error: %v", err)
	}
	if !exists {
		t.Fatalf("expected column to exist")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations were not met: %v", err)
	}
}

func TestColumnExistsPostgresDefaultsToPublicSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*\"table_schema\" = \\$3").
		WithArgs("my_table", "my_column", "public").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))

	exists, err := ColumnExists(context.Background(), db, ormdriver.PostgresDialect{}, "", "my_table", "my_column")
	if err != nil {
		t.Fatalf("ColumnExists returned error: %v", err)
	}
	if !exists {
		t.Fatalf("expected column to exist")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations were not met: %v", err)
	}
}
