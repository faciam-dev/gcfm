package rbac_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"

	"github.com/faciam-dev/gcfm/internal/rbac"
)

func TestLoad(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT r.name, p.path, p.method FROM gcfm_roles r JOIN gcfm_role_policies p ON r.id=p.role_id")).WillReturnRows(
		sqlmock.NewRows([]string{"name", "path", "method"}).AddRow("admin", "/v1/test", "GET"),
	)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT ur.user_id, r.name FROM gcfm_user_roles ur JOIN gcfm_roles r ON ur.role_id=r.id")).WillReturnRows(
		sqlmock.NewRows([]string{"user_id", "name"}).AddRow(1, "admin"),
	)
	m := model.NewModel()
	m.AddDef("r", "r", "sub, obj, act")
	m.AddDef("p", "p", "sub, obj, act")
	m.AddDef("g", "g", "_, _")
	m.AddDef("e", "e", "some(where (p.eft == allow))")
	m.AddDef("m", "m", "g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act")
	e, _ := casbin.NewEnforcer(m)
	if err := rbac.Load(context.Background(), db, e); err != nil {
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
