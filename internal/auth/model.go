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
	Role         string
}

// UserRepo provides access to the gcfm_users table.
type UserRepo struct {
	DB     *sql.DB
	Driver string
}

// GetByUsername returns a user by name.
func (r *UserRepo) GetByUsername(ctx context.Context, name string) (*User, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("repo not initialized")
	}
	var q string
	switch r.Driver {
	case "postgres":
		q = `SELECT id, username, password_hash, role FROM gcfm_users WHERE username=$1`
	default:
		q = `SELECT id, username, password_hash, role FROM gcfm_users WHERE username=?`
	}
	row := r.DB.QueryRowContext(ctx, q, name)
	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// List returns all users and their roles.
func (r *UserRepo) List(ctx context.Context) ([]User, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("repo not initialized")
	}
	var q string
	switch r.Driver {
	case "postgres":
		q = `SELECT id, username, password_hash, role FROM gcfm_users`
	default:
		q = `SELECT id, username, password_hash, role FROM gcfm_users`
	}
	rows, err := r.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role); err != nil {
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
	switch r.Driver {
	case "postgres":
		q = `SELECT r.name FROM gcfm_user_roles ur JOIN gcfm_roles r ON ur.role_id=r.id WHERE ur.user_id=$1`
	default:
		q = `SELECT r.name FROM gcfm_user_roles ur JOIN gcfm_roles r ON ur.role_id=r.id WHERE ur.user_id=?`
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
