package unit_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

func TestModifyColumnSQL_DropUnique_Postgres(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	stmt := `ALTER TABLE "posts" ALTER COLUMN "email" TYPE varchar(255), DROP CONSTRAINT IF EXISTS "posts_email_key"`
	mock.ExpectExec(regexp.QuoteMeta(stmt)).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := registry.ModifyColumnSQL(context.Background(), db, "postgres", "posts", "email", "varchar(255)", nil, boolPtr(false), nil); err != nil {
		t.Fatalf("modify: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestModifyColumnSQL_DropUnique_MySQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	modify := "ALTER TABLE `posts` MODIFY COLUMN `email` varchar(255)"
	mock.ExpectExec(regexp.QuoteMeta(modify)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM INFORMATION_SCHEMA.STATISTICS").
		WithArgs("posts", "posts_email_key").
		WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(1))
	drop := "ALTER TABLE `posts` DROP INDEX `posts_email_key`"
	mock.ExpectExec(regexp.QuoteMeta(drop)).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := registry.ModifyColumnSQL(context.Background(), db, "mysql", "posts", "email", "varchar(255)", nil, boolPtr(false), nil); err != nil {
		t.Fatalf("modify: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestModifyColumnSQL_DropUnique_MySQL_NoIndex(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	modify := "ALTER TABLE `posts` MODIFY COLUMN `email` varchar(255)"
	mock.ExpectExec(regexp.QuoteMeta(modify)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM INFORMATION_SCHEMA.STATISTICS").
		WithArgs("posts", "posts_email_key").
		WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(0))
	if err := registry.ModifyColumnSQL(context.Background(), db, "mysql", "posts", "email", "varchar(255)", nil, boolPtr(false), nil); err != nil {
		t.Fatalf("modify: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestModifyColumnSQL_AddUnique_MySQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	modify := "ALTER TABLE `posts` MODIFY COLUMN `email` varchar(255)"
	add := "ALTER TABLE `posts` ADD CONSTRAINT `posts_email_key` UNIQUE (`email`)"
	mock.ExpectExec(regexp.QuoteMeta(modify)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(add)).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := registry.ModifyColumnSQL(context.Background(), db, "mysql", "posts", "email", "varchar(255)", nil, boolPtr(true), nil); err != nil {
		t.Fatalf("modify: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
