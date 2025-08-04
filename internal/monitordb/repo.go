package monitordb

import (
	"context"
	"database/sql"
	"time"
)

// Database represents a monitored database connection.
type Database struct {
	ID        int64
	TenantID  string
	Name      string
	Driver    string
	DSNEnc    []byte
	CreatedAt time.Time
}

// Repo manages monitored database records.
type Repo struct {
	DB     *sql.DB
	Driver string
}

// Create inserts a new monitored database and returns its ID.
func (r *Repo) Create(ctx context.Context, d Database) (int64, error) {
	var q string
	switch r.Driver {
	case "postgres":
		q = `INSERT INTO monitored_databases (tenant_id, name, driver, dsn_enc) VALUES ($1,$2,$3,$4) RETURNING id`
		var id int64
		if err := r.DB.QueryRowContext(ctx, q, d.TenantID, d.Name, d.Driver, d.DSNEnc).Scan(&id); err != nil {
			return 0, err
		}
		return id, nil
	default:
		q = `INSERT INTO monitored_databases (tenant_id, name, driver, dsn_enc, created_at) VALUES (?,?,?,?, NOW())`
		res, err := r.DB.ExecContext(ctx, q, d.TenantID, d.Name, d.Driver, d.DSNEnc)
		if err != nil {
			return 0, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return 0, err
		}
		return id, nil
	}
}

// List returns all monitored databases for a tenant.
func (r *Repo) List(ctx context.Context, tenant string) ([]Database, error) {
	q := `SELECT id, tenant_id, name, driver, dsn_enc, created_at FROM monitored_databases WHERE tenant_id=? ORDER BY id`
	if r.Driver == "postgres" {
		q = `SELECT id, tenant_id, name, driver, dsn_enc, created_at FROM monitored_databases WHERE tenant_id=$1 ORDER BY id`
	}
	rows, err := r.DB.QueryContext(ctx, q, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Database
	for rows.Next() {
		var (
			d Database
			t sql.NullTime
		)
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.Driver, &d.DSNEnc, &t); err != nil {
			return nil, err
		}
		d.CreatedAt = t.Time
		res = append(res, d)
	}
	return res, rows.Err()
}

// ListAll returns all monitored databases.
func (r *Repo) ListAll(ctx context.Context) ([]Database, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, tenant_id, name, driver, dsn_enc, created_at FROM monitored_databases ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Database
	for rows.Next() {
		var (
			d Database
			t sql.NullTime
		)
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.Driver, &d.DSNEnc, &t); err != nil {
			return nil, err
		}
		d.CreatedAt = t.Time
		res = append(res, d)
	}
	return res, rows.Err()
}

// Get fetches a database by tenant and ID.
func (r *Repo) Get(ctx context.Context, tenant string, id int64) (Database, error) {
	q := `SELECT id, tenant_id, name, driver, dsn_enc, created_at FROM monitored_databases WHERE tenant_id=? AND id=?`
	if r.Driver == "postgres" {
		q = `SELECT id, tenant_id, name, driver, dsn_enc, created_at FROM monitored_databases WHERE tenant_id=$1 AND id=$2`
	}
	var (
		d Database
		t sql.NullTime
	)
	if err := r.DB.QueryRowContext(ctx, q, tenant, id).Scan(&d.ID, &d.TenantID, &d.Name, &d.Driver, &d.DSNEnc, &t); err != nil {
		return d, err
	}
	d.CreatedAt = t.Time
	return d, nil
}

// Delete removes a monitored database.
func (r *Repo) Delete(ctx context.Context, tenant string, id int64) error {
	q := `DELETE FROM monitored_databases WHERE tenant_id=? AND id=?`
	if r.Driver == "postgres" {
		q = `DELETE FROM monitored_databases WHERE tenant_id=$1 AND id=$2`
	}
	_, err := r.DB.ExecContext(ctx, q, tenant, id)
	return err
}
