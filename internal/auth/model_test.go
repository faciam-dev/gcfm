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
	repo := &UserRepo{DB: db, Driver: "postgres", TablePrefix: "gcfm_"}
	rows := sqlmock.NewRows([]string{"id", "username", "password_hash"}).
		AddRow(1, "alice", "hash").
		AddRow(2, "bob", "hash2")
	mock.ExpectQuery("^SELECT id, username, password_hash FROM gcfm_users WHERE tenant_id=\\$1$").
		WithArgs("t1").WillReturnRows(rows)
	users, err := repo.List(context.Background(), "t1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
	if len(users) != 2 || users[0].Username != "alice" || users[1].Username != "bob" {
		t.Fatalf("unexpected users: %#v", users)
	}
}

func TestUserRepoListNotInit(t *testing.T) {
	repo := &UserRepo{}
	if _, err := repo.List(context.Background(), "t1"); err == nil {
		t.Fatalf("expected error for uninitialized repo")
	}
}

func TestUserRepoListQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	repo := &UserRepo{DB: db, Driver: "postgres", TablePrefix: "gcfm_"}
	mock.ExpectQuery("^SELECT id, username, password_hash FROM gcfm_users WHERE tenant_id=\\$1$").
		WithArgs("t1").WillReturnError(errors.New("bad"))
	if _, err := repo.List(context.Background(), "t1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUserRepoListScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	repo := &UserRepo{DB: db, Driver: "postgres", TablePrefix: "gcfm_"}
	rows := sqlmock.NewRows([]string{"id", "username", "password_hash"}).
		AddRow("bad", "alice", "hash")
	mock.ExpectQuery("^SELECT id, username, password_hash FROM gcfm_users WHERE tenant_id=\\$1$").
		WithArgs("t1").WillReturnRows(rows)
	if _, err := repo.List(context.Background(), "t1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUserRepoGetRoles(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	repo := &UserRepo{DB: db, Driver: "postgres", TablePrefix: "gcfm_"}
	mock.ExpectQuery("^SELECT r.name FROM gcfm_user_roles ur JOIN gcfm_roles r ON ur.role_id=r.id WHERE ur.user_id=\\$1$").WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("admin").AddRow("viewer"))
	roles, err := repo.GetRoles(context.Background(), 1)
	if err != nil {
		t.Fatalf("get roles: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
	if len(roles) != 2 || roles[0] != "admin" || roles[1] != "viewer" {
		t.Fatalf("unexpected roles: %v", roles)
	}
}
