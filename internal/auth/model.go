package auth

import (
	"context"
	"database/sql"
	"fmt"
)

// User represents an application user.
type User struct {
	ID           uint64
	Username     string
	PasswordHash string
	Role         string
}

// UserRepo provides access to the users table.
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
		q = `SELECT id, username, password_hash, role FROM users WHERE username=$1`
	default:
		q = `SELECT id, username, password_hash, role FROM users WHERE username=?`
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
		q = `SELECT id, username, password_hash, role FROM users`
	default:
		q = `SELECT id, username, password_hash, role FROM users`
	}
	rows, err := r.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var us []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role); err != nil {
			return nil, err
		}
		us = append(us, u)
	}
	return us, rows.Err()
}
