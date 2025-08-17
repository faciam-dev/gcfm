package roles

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

// OfUser returns role names for the given user within a tenant.
// The user parameter may be either a numeric ID or a username.
func OfUser(ctx context.Context, db *sql.DB, dialect driver.Dialect, prefix, user, tenantID string) ([]string, error) {
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
	ur := prefix + "user_roles"
	users := prefix + "users"
	rolesTbl := prefix + "roles"

	q := query.New(db, ur+" ur", dialect).
		Select("r.name").
		Join(users+" u", "ur.user_id", "=", "u.id").
		Join(rolesTbl+" r", "ur.role_id", "=", "r.id").
		Where("u.tenant_id", tenantID)

	if isID {
		uid, err := strconv.ParseUint(user, 10, 64)
		if err != nil {
			return nil, err
		}
		q = q.Where("ur.user_id", uid)
	} else {
		q = q.Where("u.username", user)
	}

	q = q.OrderBy("r.name", "asc").WithContext(ctx)

	var rows []struct{ Name string }
	if err := q.Get(&rows); err != nil {
		return nil, err
	}
	roles := make([]string, 0, len(rows))
	for _, r := range rows {
		roles = append(roles, r.Name)
	}
	return roles, nil
}
