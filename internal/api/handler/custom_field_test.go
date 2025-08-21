package handler

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
)

func TestGetFieldWithoutDBID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	h := &CustomFieldHandler{DB: db, Dialect: ormdriver.PostgresDialect{}, Driver: "postgres"}

	mock.ExpectQuery(`SELECT "db_id", "data_type" FROM "custom_fields".*`).
		WithArgs("my_table", "my_column").
		WillReturnRows(sqlmock.NewRows([]string{"db_id", "data_type"}).AddRow(2, "text"))

	meta, err := h.getField(context.Background(), nil, "my_table", "my_column")
	if err != nil {
		t.Fatalf("getField returned error: %v", err)
	}
	if meta == nil || meta.DBID != 2 {
		t.Fatalf("expected DBID 2, got %#v", meta)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations were not met: %v", err)
	}
}
