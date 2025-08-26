package server

import (
	"context"
	"database/sql"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/faciam-dev/gcfm/internal/logger"
	"github.com/faciam-dev/gcfm/internal/rbac"
	"github.com/faciam-dev/gcfm/internal/server/roles"
	"github.com/faciam-dev/gcfm/pkg/tenant"
	"github.com/faciam-dev/goquent/orm/driver"
)

// initEnforcer creates a Casbin enforcer and loads policies from the database.
func initEnforcer(db *sql.DB, dialect driver.Dialect, tablePrefix string) (*casbin.Enforcer, error) {
	m := model.NewModel()
	m.AddDef("r", "r", "sub, obj, act")
	m.AddDef("p", "p", "sub, obj, act")
	m.AddDef("g", "g", "_, _")
	m.AddDef("e", "e", "some(where (p.eft == allow))")
	m.AddDef("m", "m", "g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && (r.act == p.act || p.act == \"*\")")
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, err
	}
	e.AddPolicy("admin", "/v1/*", "*")
	e.AddPolicy("admin", "/admin/*", "*")
	if db != nil {
		if err := rbac.Load(context.Background(), db, dialect, tablePrefix, e); err != nil {
			logger.L.Error("load rbac", "err", err)
		}
	}
	return e, nil
}

// roleResolver returns a function that resolves roles for a given user.
func roleResolver(db *sql.DB, dialect driver.Dialect, tablePrefix string) func(context.Context, string) ([]string, error) {
	return func(ctx context.Context, user string) ([]string, error) {
		tid := tenant.FromContext(ctx)
		return roles.OfUser(ctx, db, dialect, tablePrefix, user, tid)
	}
}
