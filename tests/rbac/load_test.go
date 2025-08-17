package rbac_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"

	"github.com/faciam-dev/gcfm/internal/rbac"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
)

func TestLoad(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectQuery("SELECT .*gcfm_roles.*gcfm_role_policies").WillReturnRows(
		sqlmock.NewRows([]string{"role", "path", "method"}).AddRow("admin", "/v1/test", "GET"),
	)
	mock.ExpectQuery("SELECT .*gcfm_user_roles.*gcfm_roles").WillReturnRows(
		sqlmock.NewRows([]string{"uid", "role"}).AddRow(int64(1), "admin"),
	)
	m := model.NewModel()
	m.AddDef("r", "r", "sub, obj, act")
	m.AddDef("p", "p", "sub, obj, act")
	m.AddDef("g", "g", "_, _")
	m.AddDef("e", "e", "some(where (p.eft == allow))")
	m.AddDef("m", "m", "g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act")
	e, _ := casbin.NewEnforcer(m)
	if err := rbac.Load(context.Background(), db, ormdriver.MySQLDialect{}, "gcfm_", e); err != nil {
		t.Fatalf("load: %v", err)
	}
	ok, _ := e.Enforce("1", "/v1/test", "GET")
	if !ok {
		t.Fatalf("policy not enforced")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
