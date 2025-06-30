package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUserRepoList(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	repo := &UserRepo{DB: db, Driver: "postgres"}
	rows := sqlmock.NewRows([]string{"id", "username", "password_hash", "role"}).
		AddRow(1, "alice", "hash", "admin").
		AddRow(2, "bob", "hash2", "user")
	mock.ExpectQuery("^SELECT id, username, password_hash, role FROM users$").WillReturnRows(rows)
	users, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
	if len(users) != 2 || users[0].Username != "alice" || users[1].Role != "user" {
		t.Fatalf("unexpected users: %#v", users)
	}
}

func TestUserRepoListNotInit(t *testing.T) {
	repo := &UserRepo{}
	if _, err := repo.List(context.Background()); err == nil {
		t.Fatalf("expected error for uninitialized repo")
	}
}

func TestUserRepoListQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	repo := &UserRepo{DB: db, Driver: "postgres"}
	mock.ExpectQuery("^SELECT id, username, password_hash, role FROM users$").WillReturnError(errors.New("bad"))
	if _, err := repo.List(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUserRepoListScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	repo := &UserRepo{DB: db, Driver: "postgres"}
	rows := sqlmock.NewRows([]string{"id", "username", "password_hash", "role"}).
		AddRow("bad", "alice", "hash", "admin")
	mock.ExpectQuery("^SELECT id, username, password_hash, role FROM users$").WillReturnRows(rows)
	if _, err := repo.List(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}
