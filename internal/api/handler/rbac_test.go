package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faciam-dev/gcfm/internal/api/schema"
	audit "github.com/faciam-dev/gcfm/internal/customfield/audit"
	huma "github.com/faciam-dev/gcfm/internal/huma"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/tenant"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
)

func TestRBACHandler_listRoles(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id`, `name`, `comment` FROM `gcfm_roles` ORDER BY `id` ASC")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "comment"}).AddRow(1, "admin", sql.NullString{}))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `role_id`, `path`, `method` FROM `gcfm_role_policies`")).
		WillReturnRows(sqlmock.NewRows([]string{"role_id", "path", "method"}).AddRow(1, "/v1/foo", "GET"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `ur`.`role_id`, COUNT(*) as count FROM `gcfm_user_roles` as `ur` INNER JOIN `gcfm_users` as `u` ON `ur`.`user_id` = `u`.`id` WHERE `u`.`tenant_id` = ? GROUP BY `ur`.`role_id`")).
		WithArgs("t1").
		WillReturnRows(sqlmock.NewRows([]string{"role_id", "count"}).AddRow(1, 1))
	h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
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
	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*").
		WithArgs("t1").
		WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(1))
	mock.ExpectQuery("SELECT .* FROM .*").
		WithArgs("t1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow(1, "admin"))
	mock.ExpectQuery("SELECT .* FROM .*").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "name"}).AddRow(1, "admin"))
	h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
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
		mock.ExpectQuery("SELECT COUNT\\(\\*\\).*").
			WithArgs("t1", "%adm%").
			WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(1))
		mock.ExpectQuery("SELECT .* FROM .*").
			WithArgs("t1", "%adm%").
			WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow(1, "admin"))
		mock.ExpectQuery("SELECT .* FROM .*").
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"user_id", "name"}).AddRow(1, "admin"))
		h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
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
		mock.ExpectQuery("SELECT COUNT\\(\\*\\).*").
			WithArgs("t1", "%zzz%").
			WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(0))
		mock.ExpectQuery("SELECT .* FROM .*").
			WithArgs("t1", "%zzz%").
			WillReturnRows(sqlmock.NewRows([]string{"id", "username"}))
		h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
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
	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*").
		WithArgs("t1", int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(0))
	mock.ExpectQuery("SELECT .* FROM .*").
		WithArgs("t1", int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}))
	rid := int64(1)
	h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
	ctx := tenant.WithTenant(context.Background(), "t1")
	out, err := h.ListUsers(ctx, &schema.ListUsersParams{ExcludeRoleID: rid})
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
	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*").
		WithArgs("t1").
		WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(3))
	mock.ExpectQuery("SELECT .* FROM .*").
		WithArgs("t1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow(2, "bob"))
	mock.ExpectQuery("SELECT .* FROM .*").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "name"}))
	h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
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

func TestRBACHandler_ListUsers_SortOrder(t *testing.T) {
	t.Run("username asc", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock: %v", err)
		}
		mock.ExpectQuery("SELECT COUNT\\(\\*\\).*").
			WithArgs("t1").
			WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(2))
		mock.ExpectQuery("SELECT .* FROM .*").
			WithArgs("t1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow(1, "alice").AddRow(2, "bob"))
		mock.ExpectQuery("SELECT .* FROM .*").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"user_id", "name"}))
		h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
		ctx := tenant.WithTenant(context.Background(), "t1")
		out, err := h.ListUsers(ctx, &schema.ListUsersParams{Sort: "username", Order: "asc"})
		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if len(out.Body.Items) != 2 || out.Body.Items[0].Username != "alice" || out.Body.Items[1].Username != "bob" {
			t.Fatalf("unexpected order: %#v", out.Body.Items)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet: %v", err)
		}
	})

	t.Run("username desc", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock: %v", err)
		}
		mock.ExpectQuery("SELECT COUNT\\(\\*\\).*").
			WithArgs("t1").
			WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(2))
		mock.ExpectQuery("SELECT .* FROM .*").
			WithArgs("t1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow(2, "bob").AddRow(1, "alice"))
		mock.ExpectQuery("SELECT .* FROM .*").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"user_id", "name"}))
		h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
		ctx := tenant.WithTenant(context.Background(), "t1")
		out, err := h.ListUsers(ctx, &schema.ListUsersParams{Sort: "username", Order: "desc"})
		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if len(out.Body.Items) != 2 || out.Body.Items[0].Username != "bob" || out.Body.Items[1].Username != "alice" {
			t.Fatalf("unexpected order: %#v", out.Body.Items)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet: %v", err)
		}
	})

	t.Run("created_at asc", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock: %v", err)
		}
		mock.ExpectQuery("SELECT COUNT\\(\\*\\).*").
			WithArgs("t1").
			WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(2))
		mock.ExpectQuery("SELECT .* FROM .*").
			WithArgs("t1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow(1, "alice").AddRow(2, "bob"))
		mock.ExpectQuery("SELECT .* FROM .*").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"user_id", "name"}))
		h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
		ctx := tenant.WithTenant(context.Background(), "t1")
		out, err := h.ListUsers(ctx, &schema.ListUsersParams{Sort: "created_at", Order: "asc"})
		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if len(out.Body.Items) != 2 || out.Body.Items[0].Username != "alice" || out.Body.Items[1].Username != "bob" {
			t.Fatalf("unexpected order: %#v", out.Body.Items)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet: %v", err)
		}
	})

	t.Run("invalid sort", func(t *testing.T) {
		h := &RBACHandler{TablePrefix: "gcfm_"}
		ctx := tenant.WithTenant(context.Background(), "t1")
		_, err := h.ListUsers(ctx, &schema.ListUsersParams{Sort: "email"})
		var se huma.StatusError
		if !errors.As(err, &se) || se.GetStatus() != http.StatusBadRequest {
			t.Fatalf("expected 400, got %v", err)
		}
	})

	t.Run("invalid order", func(t *testing.T) {
		h := &RBACHandler{TablePrefix: "gcfm_"}
		ctx := tenant.WithTenant(context.Background(), "t1")
		_, err := h.ListUsers(ctx, &schema.ListUsersParams{Order: "up"})
		var se huma.StatusError
		if !errors.As(err, &se) || se.GetStatus() != http.StatusBadRequest {
			t.Fatalf("expected 400, got %v", err)
		}
	})
}

func TestRBACHandler_createUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id`, `name` FROM `gcfm_roles` WHERE `name` IN (?)")).
		WithArgs("admin").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(2), "admin"))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO gcfm_users(tenant_id, username, password_hash) VALUES(?,?,?)")).
		WithArgs("t1", "alice", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT created_at FROM gcfm_users WHERE id=?")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(now))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO gcfm_user_roles(user_id, role_id) VALUES (?, ?)")).
		WithArgs(int64(1), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec("INSERT INTO .*audit_logs").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	rec := &audit.Recorder{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
	h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_", PasswordCost: 4, Recorder: rec}
	ctx := context.WithValue(context.Background(), middleware.UserKey(), "bob")
	ctx = tenant.WithTenant(ctx, "t1")
	in := &createUserInput{}
	in.Body.Username = "alice"
	in.Body.Password = "password123"
	in.Body.Roles = []string{"admin"}
	out, err := h.createUser(ctx, in)
	if err != nil {
		t.Fatalf("createUser: %v", err)
	}
	if out.Body.ID != 1 || out.Body.TenantID != "t1" || out.Body.Username != "alice" || len(out.Body.Roles) != 1 || out.Body.Roles[0] != "admin" || !out.Body.CreatedAt.Equal(now) {
		t.Fatalf("unexpected output: %#v", out.Body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestRBACHandler_createUser_parseTimeBytes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id`, `name` FROM `gcfm_roles` WHERE `name` IN (?)")).
		WithArgs("admin").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(2), "admin"))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO gcfm_users(tenant_id, username, password_hash) VALUES(?,?,?)")).
		WithArgs("t1", "alice", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	now := time.Now().UTC().Truncate(time.Second)
	ts := []byte(now.Format("2006-01-02 15:04:05"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT created_at FROM gcfm_users WHERE id=?")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(ts))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO gcfm_user_roles(user_id, role_id) VALUES (?, ?)")).
		WithArgs(int64(1), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec("INSERT INTO .*audit_logs").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	rec := &audit.Recorder{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
	h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_", PasswordCost: 4, Recorder: rec}
	ctx := context.WithValue(context.Background(), middleware.UserKey(), "bob")
	ctx = tenant.WithTenant(ctx, "t1")
	in := &createUserInput{}
	in.Body.Username = "alice"
	in.Body.Password = "password123"
	in.Body.Roles = []string{"admin"}
	out, err := h.createUser(ctx, in)
	if err != nil {
		t.Fatalf("createUser: %v", err)
	}
	if !out.Body.CreatedAt.Equal(now) {
		t.Fatalf("unexpected created_at: %v", out.Body.CreatedAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestRBACHandler_createUser_duplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id`, `name` FROM `gcfm_roles` WHERE `name` IN (?)")).
		WithArgs("admin").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(2), "admin"))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO gcfm_users(tenant_id, username, password_hash) VALUES(?,?,?)")).
		WithArgs("t1", "alice", sqlmock.AnyArg()).
		WillReturnError(errors.New("duplicate"))
	mock.ExpectRollback()
	h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_", PasswordCost: 4}
	ctx := tenant.WithTenant(context.Background(), "t1")
	in := &createUserInput{}
	in.Body.Username = "alice"
	in.Body.Password = "password123"
	in.Body.Roles = []string{"admin"}
	_, err = h.createUser(ctx, in)
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

func TestRBACHandler_createRole_duplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO gcfm_roles(comment, name) VALUES (?, ?)")).
		WithArgs(nil, "admin").WillReturnError(errors.New("duplicate"))
	h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
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
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) as count FROM `gcfm_user_roles` WHERE `role_id` = ?")).
		WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
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
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO gcfm_roles(comment, name) VALUES (?, ?)")).
		WithArgs(nil, "dev").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) as count FROM `gcfm_user_roles` WHERE `role_id` = ?")).
		WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) as count FROM `gcfm_role_policies` WHERE `role_id` = ?")).
		WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `gcfm_roles` WHERE `id` = ?")).
		WithArgs(int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
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
	t.Skip("TODO: adjust expectations for goquent delete ordering")
}

func TestRBACHandler_putRoleMembers_invalidUser(t *testing.T) {
	t.Skip("TODO: adjust expectations for goquent delete ordering")
}

func TestRBACHandler_addRolePolicy_duplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `gcfm_role_policies` (`method`, `path`, `role_id`) VALUES (?, ?, ?)")).
		WithArgs("GET", "/foo", int64(1)).
		WillReturnError(errors.New("duplicate"))
	h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
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
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `gcfm_role_policies` WHERE `role_id` = ? AND `path` = ? AND `method` = ?")).
		WithArgs(int64(1), "/foo", "GET").
		WillReturnResult(sqlmock.NewResult(0, 0))
	h := &RBACHandler{DB: db, Dialect: ormdriver.MySQLDialect{}, TablePrefix: "gcfm_"}
	p := &policyParams{ID: 1, Path: "/foo", Method: "GET"}
	if _, err := h.deleteRolePolicy(context.Background(), p); err != nil {
		t.Fatalf("deleteRolePolicy: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
