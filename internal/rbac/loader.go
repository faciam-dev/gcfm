package rbac

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/casbin/casbin/v2"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

// Load fills the Casbin enforcer with policies and groupings from the database.
func Load(ctx context.Context, db *sql.DB, dialect ormdriver.Dialect, prefix string, e *casbin.Enforcer) error {
	if db == nil || e == nil {
		return nil
	}
	roles := prefix + "roles"
	policies := prefix + "role_policies"
	q := query.New(db, roles+" as r", dialect).
		Select("r.name as role").
		Select("p.path").
		Select("p.method").
		Join(policies+" as p", "r.id", "=", "p.role_id").
		WithContext(ctx)

	var rows []map[string]any
	if err := q.GetMaps(&rows); err != nil {
		return err
	}
	for _, r := range rows {
		if _, err := e.AddPolicy(fmt.Sprint(r["role"]), fmt.Sprint(r["path"]), fmt.Sprint(r["method"])); err != nil {
			return err
		}
	}
	return loadGroupPolicies(ctx, db, dialect, prefix, e)
}

func loadGroupPolicies(ctx context.Context, db *sql.DB, dialect ormdriver.Dialect, prefix string, e *casbin.Enforcer) error {
	userRoles := prefix + "user_roles"
	roles := prefix + "roles"
	q := query.New(db, userRoles+" as ur", dialect).
		Select("ur.user_id as uid").
		Select("r.name as role").
		Join(roles+" as r", "ur.role_id", "=", "r.id").
		WithContext(ctx)

	var rows []map[string]any
	if err := q.GetMaps(&rows); err != nil {
		return err
	}
	for _, r := range rows {
		if _, err := e.AddGroupingPolicy(fmt.Sprint(r["uid"]), fmt.Sprint(r["role"])); err != nil {
			return err
		}
	}
	return nil
}

// ReloadEnforcer is a hook for reloading the Casbin enforcer after RBAC changes.
// Currently it is a no-op and can be overridden elsewhere.
func ReloadEnforcer(ctx context.Context, db *sql.DB) {}
