package rbac

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/casbin/casbin/v2"
)

// Load fills the Casbin enforcer with policies and groupings from the database.
func Load(ctx context.Context, db *sql.DB, e *casbin.Enforcer) error {
	if db == nil || e == nil {
		return nil
	}
	rows, err := db.QueryContext(ctx, `SELECT r.name, p.path, p.method FROM gcfm_roles r JOIN gcfm_role_policies p ON r.id=p.role_id`)
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
	rows2, err := db.QueryContext(ctx, `SELECT ur.user_id, r.name FROM gcfm_user_roles ur JOIN gcfm_roles r ON ur.role_id=r.id`)
	if err != nil {
		return err
	}
	defer rows2.Close()
	for rows2.Next() {
		var uid int64
		var role string
		if err := rows2.Scan(&uid, &role); err != nil {
			return err
		}
		e.AddGroupingPolicy(fmt.Sprint(uid), role)
	}
	return rows2.Err()
}
