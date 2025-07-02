package handler

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faciam-dev/gcfm/internal/api/schema"
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
