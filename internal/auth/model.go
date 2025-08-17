package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

// User represents an application user.
type User struct {
	ID           uint64
	Username     string
	PasswordHash string
}

// UserRepo provides access to the gcfm_users table.
type UserRepo struct {
	DB          *sql.DB
	Dialect     driver.Dialect
	TablePrefix string
}

// GetByUsername returns a user by name within a tenant.
func (r *UserRepo) GetByUsername(ctx context.Context, tenantID, name string) (*User, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("repo not initialized")
	}
	tbl := r.TablePrefix + "users"
	q := query.New(r.DB, tbl, r.Dialect).
		Select("id", "username", "password_hash").
		Where("tenant_id", tenantID).
		Where("username", name).
		WithContext(ctx)
	var u User
	if err := q.First(&u); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// List returns all users within a tenant.
func (r *UserRepo) List(ctx context.Context, tenantID string) ([]User, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("repo not initialized")
	}
	tbl := r.TablePrefix + "users"
	q := query.New(r.DB, tbl, r.Dialect).
		Select("id", "username", "password_hash").
		Where("tenant_id", tenantID).
		WithContext(ctx)
	var users []User
	if err := q.Get(&users); err != nil {
		return nil, err
	}
	return users, nil
}

// GetRoles returns all role names associated with the given user ID.
func (r *UserRepo) GetRoles(ctx context.Context, userID uint64) ([]string, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("repo not initialized")
	}
	ur := r.TablePrefix + "user_roles"
	rolesTbl := r.TablePrefix + "roles"
	q := query.New(r.DB, ur+" as ur", r.Dialect).
		Select("r.name").
		Join(rolesTbl+" as r", "ur.role_id", "=", "r.id").
		Where("ur.user_id", userID).
		WithContext(ctx)
	var rows []struct{ Name string }
	if err := q.Get(&rows); err != nil {
		return nil, err
	}
	roles := make([]string, 0, len(rows))
	for _, r := range rows {
		roles = append(roles, strings.ToLower(r.Name))
	}
	return roles, nil
}
