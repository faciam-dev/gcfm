package handler

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faciam-dev/gcfm/pkg/tenant"
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

func TestListReturnsValidator(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	h := &CustomFieldHandler{DB: db, Driver: "postgres", Dialect: ormdriver.PostgresDialect{}, TablePrefix: "gcfm_"}

	rows := sqlmock.NewRows([]string{"db_id", "table_name", "column_name", "data_type", "label_key", "widget", "widget_config", "placeholder_key", "nullable", "unique", "has_default", "default_value", "validator"}).
		AddRow(4, "tenant_confirm_tokens", "new1", "varchar", nil, nil, nil, nil, false, false, false, nil, "email")

	mock.ExpectQuery(`SELECT .* FROM "gcfm_custom_fields"`).
		WithArgs("default", int64(4)).
		WillReturnRows(rows)

	ctx := tenant.WithTenant(context.Background(), "default")
	out, err := h.list(ctx, &listParams{DBID: 4})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out.Body) != 1 || out.Body[0].Validator != "email" {
		t.Fatalf("unexpected result: %+v", out.Body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations were not met: %v", err)
	}
}
