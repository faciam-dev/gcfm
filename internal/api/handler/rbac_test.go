package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faciam-dev/gcfm/internal/api/schema"
	huma "github.com/faciam-dev/gcfm/internal/huma"
)

func TestRBACHandler_listRoles(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, comment FROM gcfm_roles ORDER BY id")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "comment"}).AddRow(1, "admin", ""))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT role_id, path, method FROM gcfm_role_policies")).
		WillReturnRows(sqlmock.NewRows([]string{"role_id", "path", "method"}).AddRow(1, "/v1/foo", "GET"))
	h := &RBACHandler{DB: db, Driver: "mysql"}
	out, err := h.listRoles(context.Background(), nil)
	if err != nil {
		t.Fatalf("listRoles: %v", err)
	}
	if len(out.Body) != 1 || out.Body[0].Name != "admin" || len(out.Body[0].Policies) != 1 {
		t.Fatalf("unexpected result: %#v", out.Body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestRBACHandler_listUsers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT u.id, u.username, r.name FROM gcfm_users u LEFT JOIN gcfm_user_roles ur ON u.id=ur.user_id LEFT JOIN gcfm_roles r ON ur.role_id=r.id ORDER BY u.id")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "name"}).
			AddRow(1, "alice", sql.NullString{String: "admin", Valid: true}).
			AddRow(2, "bob", sql.NullString{Valid: false}))
	h := &RBACHandler{DB: db, Driver: "mysql"}
	out, err := h.listUsers(context.Background(), nil)
	if err != nil {
		t.Fatalf("listUsers: %v", err)
	}
	if len(out.Body) != 2 {
		t.Fatalf("unexpected len: %d", len(out.Body))
	}
	byName := map[string]schema.User{}
	for _, u := range out.Body {
		byName[u.Username] = u
	}
	a, ok := byName["alice"]
	if !ok || len(a.Roles) != 1 || a.Roles[0] != "admin" {
		t.Fatalf("unexpected alice: %#v", a)
	}
	b, ok := byName["bob"]
	if !ok || len(b.Roles) != 0 {
		t.Fatalf("unexpected bob: %#v", b)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestRBACHandler_createRole_duplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO gcfm_roles(name, comment) VALUES(?, ?)")).
		WithArgs("admin", nil).WillReturnError(errors.New("duplicate"))
	h := &RBACHandler{DB: db, Driver: "mysql"}
	in := &createRoleInput{}
	in.Body.Name = "admin"
	_, err = h.createRole(context.Background(), in)
	if err == nil {
		t.Fatalf("expected error")
	}
	se, ok := err.(huma.StatusError)
	if !ok || se.GetStatus() != http.StatusConflict {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestRBACHandler_deleteRole_referenced(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM gcfm_user_roles WHERE role_id=?")).
		WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	h := &RBACHandler{DB: db, Driver: "mysql"}
	_, err = h.deleteRole(context.Background(), &roleIDParam{ID: 1})
	if err == nil {
		t.Fatalf("expected error")
	}
	se, ok := err.(huma.StatusError)
	if !ok || se.GetStatus() != http.StatusConflict {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestRBACHandler_createDeleteRole(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO gcfm_roles(name, comment) VALUES(?, ?)")).
		WithArgs("dev", nil).WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM gcfm_user_roles WHERE role_id=?")).
		WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM gcfm_role_policies WHERE role_id=?")).
		WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM gcfm_roles WHERE id=?")).
		WithArgs(int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	h := &RBACHandler{DB: db, Driver: "mysql"}
	in := &createRoleInput{}
	in.Body.Name = "dev"
	out, err := h.createRole(context.Background(), in)
	if err != nil {
		t.Fatalf("createRole: %v", err)
	}
	if out.Body.ID != 2 || out.Body.Name != "dev" {
		t.Fatalf("unexpected out: %#v", out.Body)
	}
	if _, err := h.deleteRole(context.Background(), &roleIDParam{ID: 2}); err != nil {
		t.Fatalf("deleteRole: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
