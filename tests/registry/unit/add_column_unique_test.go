package unit_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

func TestAddColumnSQL_MySQL_Unique(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	add := "ALTER TABLE `posts` ADD COLUMN `email` VARCHAR(255) NOT NULL"
	uq := "ALTER TABLE `posts` ADD CONSTRAINT `posts_email_key` UNIQUE (`email`)"
	mock.ExpectExec(regexp.QuoteMeta(add)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(uq)).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := registry.AddColumnSQL(context.Background(), db, "mysql", "posts", "email", "varchar", boolPtr(false), boolPtr(true), registry.UnifiedDefault{}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
