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
	"github.com/faciam-dev/gcfm/internal/tenant"
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
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, username FROM gcfm_users WHERE tenant_id=? ORDER BY username LIMIT ? OFFSET ?")).
		WithArgs("t1", 50, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow(1, "alice").AddRow(2, "bob"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT ur.user_id, r.name FROM gcfm_user_roles ur JOIN gcfm_roles r ON ur.role_id=r.id WHERE ur.user_id IN (?,?)")).
		WithArgs(int64(1), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "name"}).
			AddRow(1, "admin"))
	h := &RBACHandler{DB: db, Driver: "mysql"}
	ctx := tenant.WithTenant(context.Background(), "t1")
	out, err := h.listUsers(ctx, &listUsersParams{})
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

func TestRBACHandler_putRoleMembers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT ur.user_id FROM gcfm_user_roles ur JOIN gcfm_users u ON ur.user_id=u.id WHERE ur.role_id=? AND u.tenant_id=?")).
		WithArgs(int64(1), "t1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(1).AddRow(2))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM gcfm_user_roles WHERE role_id=? AND user_id=?")).
		WithArgs(int64(1), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM gcfm_users WHERE id=? AND tenant_id=?")).
		WithArgs(int64(3), "t1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT IGNORE INTO gcfm_user_roles(user_id, role_id) VALUES(?, ?)")).
		WithArgs(int64(3), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	h := &RBACHandler{DB: db, Driver: "mysql"}
	in := &roleMembersInput{ID: 1}
	in.Body.UserIDs = []int64{2, 3}
	ctx := tenant.WithTenant(context.Background(), "t1")
	if _, err := h.putRoleMembers(ctx, in); err != nil {
		t.Fatalf("putRoleMembers: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestRBACHandler_addRolePolicy_duplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO gcfm_role_policies(role_id, path, method) VALUES(?,?,?)")).
		WithArgs(int64(1), "/foo", "GET").
		WillReturnError(errors.New("duplicate"))
	h := &RBACHandler{DB: db, Driver: "mysql"}
	in := &policyInput{ID: 1}
	in.Body.Path = "/foo"
	in.Body.Method = "GET"
	_, err = h.addRolePolicy(context.Background(), in)
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

func TestRBACHandler_deleteRolePolicy_idempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM gcfm_role_policies WHERE role_id=? AND path=? AND method=?")).
		WithArgs(int64(1), "/foo", "GET").
		WillReturnResult(sqlmock.NewResult(0, 0))
	h := &RBACHandler{DB: db, Driver: "mysql"}
	p := &policyParams{ID: 1, Path: "/foo", Method: "GET"}
	if _, err := h.deleteRolePolicy(context.Background(), p); err != nil {
		t.Fatalf("deleteRolePolicy: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
