package roles

import (
	"context"
	"database/sql"
	"fmt"
)

// OfUser returns role names for the given user within a tenant.
// The user parameter may be either a numeric ID or a username.
func OfUser(ctx context.Context, db *sql.DB, driver, prefix, user, tenantID string) ([]string, error) {
	if db == nil {
		return nil, nil
	}
	isID := true
	for _, c := range user {
		if c < '0' || c > '9' {
			isID = false
			break
		}
	}
	var q string
	ur := prefix + "user_roles"
	users := prefix + "users"
	rolesTbl := prefix + "roles"
	var args []any
	if driver == "mysql" {
		if isID {
			q = fmt.Sprintf("SELECT r.name FROM %s ur JOIN %s u ON u.id = ur.user_id JOIN %s r ON r.id = ur.role_id WHERE ur.user_id = ? AND u.tenant_id = ? ORDER BY r.name", ur, users, rolesTbl)
			args = []any{user, tenantID}
		} else {
			q = fmt.Sprintf("SELECT r.name FROM %s ur JOIN %s u ON u.id = ur.user_id JOIN %s r ON r.id = ur.role_id WHERE u.username = ? AND u.tenant_id = ? ORDER BY r.name", ur, users, rolesTbl)
			args = []any{user, tenantID}
		}
	} else {
		if isID {
			q = fmt.Sprintf("SELECT r.name FROM %s ur JOIN %s u ON u.id = ur.user_id JOIN %s r ON r.id = ur.role_id WHERE ur.user_id = $1 AND u.tenant_id = $2 ORDER BY r.name", ur, users, rolesTbl)
			args = []any{user, tenantID}
		} else {
			q = fmt.Sprintf("SELECT r.name FROM %s ur JOIN %s u ON u.id = ur.user_id JOIN %s r ON r.id = ur.role_id WHERE u.username = $1 AND u.tenant_id = $2 ORDER BY r.name", ur, users, rolesTbl)
			args = []any{user, tenantID}
		}
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		roles = append(roles, name)
	}
	return roles, rows.Err()
}
