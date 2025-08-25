package unit_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

func TestAddColumnSQL_MySQL_DefaultExpr(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	add := "ALTER TABLE `posts` ADD COLUMN `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP"
	mock.ExpectExec(regexp.QuoteMeta(add)).WillReturnResult(sqlmock.NewResult(0, 1))
	d := registry.UnifiedDefault{Mode: "expression", Raw: "CURRENT_TIMESTAMP"}
	if err := registry.AddColumnSQL(context.Background(), db, "mysql", "posts", "created_at", "DATETIME", nil, nil, d); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestAddColumnSQL_Postgres_DefaultExpr(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	add := "ALTER TABLE \"posts\" ADD COLUMN \"created_at\" TIMESTAMP DEFAULT CURRENT_TIMESTAMP"
	mock.ExpectExec(regexp.QuoteMeta(add)).WillReturnResult(sqlmock.NewResult(0, 1))
	d := registry.UnifiedDefault{Mode: "expression", Raw: "CURRENT_TIMESTAMP"}
	if err := registry.AddColumnSQL(context.Background(), db, "postgres", "posts", "created_at", "TIMESTAMP", nil, nil, d); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestAddColumnSQL_MySQL_DefaultExprOnUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	add := "ALTER TABLE `posts` ADD COLUMN `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP"
	mock.ExpectExec(regexp.QuoteMeta(add)).WillReturnResult(sqlmock.NewResult(0, 1))
	d := registry.UnifiedDefault{Mode: "expression", Raw: "CURRENT_TIMESTAMP", OnUpdate: true}
	if err := registry.AddColumnSQL(context.Background(), db, "mysql", "posts", "updated_at", "DATETIME", nil, nil, d); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestAddColumnSQL_MySQL_MapExpressionDate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	add := "ALTER TABLE `posts` ADD COLUMN `published` DATE DEFAULT CURRENT_DATE"
	mock.ExpectExec(regexp.QuoteMeta(add)).WillReturnResult(sqlmock.NewResult(0, 1))
	d := registry.UnifiedDefault{Mode: "expression", Raw: "CURRENT_TIMESTAMP", OnUpdate: true}
	if err := registry.AddColumnSQL(context.Background(), db, "mysql", "posts", "published", "DATE", nil, nil, d); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
