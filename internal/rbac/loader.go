package rbac

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/casbin/casbin/v2"
)

// Load fills the Casbin enforcer with policies and groupings from the database.
func Load(ctx context.Context, db *sql.DB, prefix string, e *casbin.Enforcer) error {
	if db == nil || e == nil {
		return nil
	}
	roles := prefix + "roles"
	policies := prefix + "role_policies"
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT r.name, p.path, p.method FROM %s r JOIN %s p ON r.id=p.role_id", roles, policies))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var role, path, method string
		if err := rows.Scan(&role, &path, &method); err != nil {
			return err
		}
		e.AddPolicy(role, path, method)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return loadGroupPolicies(ctx, db, prefix, e)
}

func loadGroupPolicies(ctx context.Context, db *sql.DB, prefix string, e *casbin.Enforcer) error {
	userRoles := prefix + "user_roles"
	roles := prefix + "roles"
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT ur.user_id, r.name FROM %s ur JOIN %s r ON ur.role_id=r.id", userRoles, roles))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var uid int64
		var role string
		if err := rows.Scan(&uid, &role); err != nil {
			return err
		}
		e.AddGroupingPolicy(fmt.Sprint(uid), role)
	}
	return rows.Err()
}

// ReloadEnforcer is a hook for reloading the Casbin enforcer after RBAC changes.
// Currently it is a no-op and can be overridden elsewhere.
func ReloadEnforcer(ctx context.Context, db *sql.DB) {}
