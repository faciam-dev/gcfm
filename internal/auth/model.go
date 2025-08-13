package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
	Driver      string
	TablePrefix string
}

// GetByUsername returns a user by name within a tenant.
func (r *UserRepo) GetByUsername(ctx context.Context, tenantID, name string) (*User, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("repo not initialized")
	}
	var q string
	tbl := r.TablePrefix + "users"
	switch r.Driver {
	case "postgres":
		q = fmt.Sprintf("SELECT id, username, password_hash FROM %s WHERE tenant_id=$1 AND username=$2", tbl)
		row := r.DB.QueryRowContext(ctx, q, tenantID, name)
		var u User
		if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
		return &u, nil
	default:
		q = fmt.Sprintf("SELECT id, username, password_hash FROM %s WHERE tenant_id=? AND username=?", tbl)
		row := r.DB.QueryRowContext(ctx, q, tenantID, name)
		var u User
		if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
		return &u, nil
	}
}

// List returns all users within a tenant.
func (r *UserRepo) List(ctx context.Context, tenantID string) ([]User, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("repo not initialized")
	}
	var q string
	tbl := r.TablePrefix + "users"
	switch r.Driver {
	case "postgres":
		q = fmt.Sprintf("SELECT id, username, password_hash FROM %s WHERE tenant_id=$1", tbl)
	default:
		q = fmt.Sprintf("SELECT id, username, password_hash FROM %s WHERE tenant_id=?", tbl)
	}
	rows, err := r.DB.QueryContext(ctx, q, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetRoles returns all role names associated with the given user ID.
func (r *UserRepo) GetRoles(ctx context.Context, userID uint64) ([]string, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("repo not initialized")
	}
	var q string
	ur := r.TablePrefix + "user_roles"
	rolesTbl := r.TablePrefix + "roles"
	switch r.Driver {
	case "postgres":
		q = fmt.Sprintf("SELECT r.name FROM %s ur JOIN %s r ON ur.role_id=r.id WHERE ur.user_id=$1", ur, rolesTbl)
	default:
		q = fmt.Sprintf("SELECT r.name FROM %s ur JOIN %s r ON ur.role_id=r.id WHERE ur.user_id=?", ur, rolesTbl)
	}
	rows, err := r.DB.QueryContext(ctx, q, userID)
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
		roles = append(roles, strings.ToLower(name))
	}
	return roles, rows.Err()
}
