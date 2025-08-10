package handler

import (
	"context"
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
	mock.ExpectQuery(regexp.QuoteMeta("SELECT ur.role_id, COUNT(*) FROM gcfm_user_roles ur JOIN gcfm_users u ON ur.user_id=u.id WHERE u.tenant_id=? GROUP BY ur.role_id")).
		WithArgs("t1").
		WillReturnRows(sqlmock.NewRows([]string{"role_id", "count"}).AddRow(1, 1))
	h := &RBACHandler{DB: db, Driver: "mysql"}
	ctx := tenant.WithTenant(context.Background(), "t1")
	out, err := h.listRoles(ctx, nil)
	if err != nil {
		t.Fatalf("listRoles: %v", err)
	}
	if len(out.Body) != 1 || out.Body[0].Name != "admin" || len(out.Body[0].Policies) != 1 || out.Body[0].Members != 1 {
		t.Fatalf("unexpected result: %#v", out.Body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestRBACHandler_ListUsers_Basic(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM gcfm_users u WHERE u.tenant_id = ?")).
		WithArgs("t1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT u.id, u.username FROM gcfm_users u WHERE u.tenant_id = ? ORDER BY u.username ASC LIMIT ? OFFSET ?")).
		WithArgs("t1", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow(1, "admin"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT ur.user_id, r.name FROM gcfm_user_roles ur JOIN gcfm_roles r ON r.id = ur.role_id WHERE ur.user_id IN (?) ORDER BY r.name")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "name"}).AddRow(1, "admin"))
	h := &RBACHandler{DB: db, Driver: "mysql"}
	ctx := tenant.WithTenant(context.Background(), "t1")
	out, err := h.ListUsers(ctx, &schema.ListUsersParams{})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if out.Body.Total != 1 || len(out.Body.Items) != 1 || out.Body.Items[0].Username != "admin" || len(out.Body.Items[0].Roles) != 1 {
		t.Fatalf("unexpected result: %#v", out.Body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestRBACHandler_ListUsers_Search(t *testing.T) {
	t.Run("match", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock: %v", err)
		}
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM gcfm_users u WHERE u.tenant_id = ? AND u.username LIKE ?")).
			WithArgs("t1", "%adm%").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT u.id, u.username FROM gcfm_users u WHERE u.tenant_id = ? AND u.username LIKE ? ORDER BY u.username ASC LIMIT ? OFFSET ?")).
			WithArgs("t1", "%adm%", 20, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow(1, "admin"))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT ur.user_id, r.name FROM gcfm_user_roles ur JOIN gcfm_roles r ON r.id = ur.role_id WHERE ur.user_id IN (?) ORDER BY r.name")).
			WithArgs(int64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"user_id", "name"}).AddRow(1, "admin"))
		h := &RBACHandler{DB: db, Driver: "mysql"}
		ctx := tenant.WithTenant(context.Background(), "t1")
		out, err := h.ListUsers(ctx, &schema.ListUsersParams{Search: "adm"})
		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if out.Body.Total != 1 || len(out.Body.Items) != 1 {
			t.Fatalf("unexpected: %#v", out.Body)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet: %v", err)
		}
	})

	t.Run("no match", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock: %v", err)
		}
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM gcfm_users u WHERE u.tenant_id = ? AND u.username LIKE ?")).
			WithArgs("t1", "%zzz%").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT u.id, u.username FROM gcfm_users u WHERE u.tenant_id = ? AND u.username LIKE ? ORDER BY u.username ASC LIMIT ? OFFSET ?")).
			WithArgs("t1", "%zzz%", 20, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "username"}))
		h := &RBACHandler{DB: db, Driver: "mysql"}
		ctx := tenant.WithTenant(context.Background(), "t1")
		out, err := h.ListUsers(ctx, &schema.ListUsersParams{Search: "zzz"})
		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if out.Body.Total != 0 || len(out.Body.Items) != 0 {
			t.Fatalf("unexpected: %#v", out.Body)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet: %v", err)
		}
	})
}

func TestRBACHandler_ListUsers_ExcludeRole(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM gcfm_users u WHERE u.tenant_id = ? AND NOT EXISTS (SELECT 1 FROM gcfm_user_roles ur WHERE ur.user_id = u.id AND ur.role_id = ?)")).
		WithArgs("t1", int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT u.id, u.username FROM gcfm_users u WHERE u.tenant_id = ? AND NOT EXISTS (SELECT 1 FROM gcfm_user_roles ur WHERE ur.user_id = u.id AND ur.role_id = ?) ORDER BY u.username ASC LIMIT ? OFFSET ?")).
		WithArgs("t1", int64(1), 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}))
	rid := int64(1)
	h := &RBACHandler{DB: db, Driver: "mysql"}
	ctx := tenant.WithTenant(context.Background(), "t1")
	out, err := h.ListUsers(ctx, &schema.ListUsersParams{ExcludeRoleID: &rid})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if out.Body.Total != 0 || len(out.Body.Items) != 0 {
		t.Fatalf("unexpected: %#v", out.Body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestRBACHandler_ListUsers_Paging(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM gcfm_users u WHERE u.tenant_id = ?")).
		WithArgs("t1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT u.id, u.username FROM gcfm_users u WHERE u.tenant_id = ? ORDER BY u.username ASC LIMIT ? OFFSET ?")).
		WithArgs("t1", 1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow(2, "bob"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT ur.user_id, r.name FROM gcfm_user_roles ur JOIN gcfm_roles r ON r.id = ur.role_id WHERE ur.user_id IN (?) ORDER BY r.name")).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "name"}))
	h := &RBACHandler{DB: db, Driver: "mysql"}
	ctx := tenant.WithTenant(context.Background(), "t1")
	out, err := h.ListUsers(ctx, &schema.ListUsersParams{Page: 2, PerPage: 1})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if out.Body.Total != 3 || len(out.Body.Items) != 1 || out.Body.Items[0].Username != "bob" || out.Body.Page != 2 || out.Body.PerPage != 1 {
		t.Fatalf("unexpected: %#v", out.Body)
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

func TestRBACHandler_putRoleMembers_invalidUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT ur.user_id FROM gcfm_user_roles ur JOIN gcfm_users u ON ur.user_id=u.id WHERE ur.role_id=? AND u.tenant_id=?")).
		WithArgs(int64(1), "t1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM gcfm_users WHERE id=? AND tenant_id=?")).
		WithArgs(int64(2), "t1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectRollback()
	h := &RBACHandler{DB: db, Driver: "mysql"}
	ctx := tenant.WithTenant(context.Background(), "t1")
	in := &roleMembersInput{ID: 1}
	in.Body.UserIDs = []int64{2}
	_, err = h.putRoleMembers(ctx, in)
	if err == nil {
		t.Fatalf("expected error")
	}
	se, ok := err.(huma.StatusError)
	if !ok || se.GetStatus() != http.StatusUnprocessableEntity {
		t.Fatalf("unexpected error: %v", err)
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
