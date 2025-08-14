package monitordb

import (
	"context"
	"database/sql"
	"fmt"
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
	DB          *sql.DB
	Driver      string
	TablePrefix string
}

func (r *Repo) prefix() string {
	if r.TablePrefix != "" {
		return r.TablePrefix
	}
	return "gcfm_"
}

func (r *Repo) table() string {
	return r.prefix() + "monitored_databases"
}

// Create inserts a new monitored database and returns its ID.
func (r *Repo) Create(ctx context.Context, d Database) (int64, error) {
	var q string
	tbl := r.table()
	switch r.Driver {
	case "postgres":
		q = fmt.Sprintf(`INSERT INTO %s (tenant_id, name, driver, dsn_enc) VALUES ($1,$2,$3,$4) RETURNING id`, tbl)
		var id int64
		if err := r.DB.QueryRowContext(ctx, q, d.TenantID, d.Name, d.Driver, d.DSNEnc).Scan(&id); err != nil {
			return 0, err
		}
		return id, nil
	default:
		q = fmt.Sprintf(`INSERT INTO %s (tenant_id, name, driver, dsn_enc, created_at) VALUES (?,?,?,?, NOW())`, tbl)
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
	tbl := r.table()
	q := fmt.Sprintf(`SELECT id, tenant_id, name, driver, dsn_enc, created_at FROM %s WHERE tenant_id=? ORDER BY id`, tbl)
	if r.Driver == "postgres" {
		q = fmt.Sprintf(`SELECT id, tenant_id, name, driver, dsn_enc, created_at FROM %s WHERE tenant_id=$1 ORDER BY id`, tbl)
	}
	rows, err := r.DB.QueryContext(ctx, q, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Database
	for rows.Next() {
		var (
			d  Database
			ct any
		)
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.Driver, &d.DSNEnc, &ct); err != nil {
			return nil, err
		}
		t, err := parseSQLTime(ct)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}
		d.CreatedAt = t
		res = append(res, d)
	}
	return res, rows.Err()
}

// ListAll returns all monitored databases.
func (r *Repo) ListAll(ctx context.Context) ([]Database, error) {
	tbl := r.table()
	rows, err := r.DB.QueryContext(ctx, fmt.Sprintf(`SELECT id, tenant_id, name, driver, dsn_enc, created_at FROM %s ORDER BY id`, tbl))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Database
	for rows.Next() {
		var (
			d  Database
			ct any
		)
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.Driver, &d.DSNEnc, &ct); err != nil {
			return nil, err
		}
		t, err := parseSQLTime(ct)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}
		d.CreatedAt = t
		res = append(res, d)
	}
	return res, rows.Err()
}

// Get fetches a database by tenant and ID.
func (r *Repo) Get(ctx context.Context, tenant string, id int64) (Database, error) {
	tbl := r.table()
	q := fmt.Sprintf(`SELECT id, tenant_id, name, driver, dsn_enc, created_at FROM %s WHERE tenant_id=? AND id=?`, tbl)
	if r.Driver == "postgres" {
		q = fmt.Sprintf(`SELECT id, tenant_id, name, driver, dsn_enc, created_at FROM %s WHERE tenant_id=$1 AND id=$2`, tbl)
	}
	var (
		d  Database
		ct any
	)
	if err := r.DB.QueryRowContext(ctx, q, tenant, id).Scan(&d.ID, &d.TenantID, &d.Name, &d.Driver, &d.DSNEnc, &ct); err != nil {
		return d, err
	}
	t, err := parseSQLTime(ct)
	if err != nil {
		return d, fmt.Errorf("parse created_at: %w", err)
	}
	d.CreatedAt = t
	return d, nil
}

// Update modifies an existing monitored database's attributes.
func (r *Repo) Update(ctx context.Context, tenant string, id int64, name, driver string, dsnEnc []byte) error {
	tbl := r.table()
	q := fmt.Sprintf(`UPDATE %s SET name=?, driver=?, dsn_enc=? WHERE tenant_id=? AND id=?`, tbl)
	if r.Driver == "postgres" {
		q = fmt.Sprintf(`UPDATE %s SET name=$1, driver=$2, dsn_enc=$3 WHERE tenant_id=$4 AND id=$5`, tbl)
	}
	_, err := r.DB.ExecContext(ctx, q, name, driver, dsnEnc, tenant, id)
	return err
}

// Delete removes a monitored database.
func (r *Repo) Delete(ctx context.Context, tenant string, id int64) error {
	tbl := r.table()
	q := fmt.Sprintf(`DELETE FROM %s WHERE tenant_id=? AND id=?`, tbl)
	if r.Driver == "postgres" {
		q = fmt.Sprintf(`DELETE FROM %s WHERE tenant_id=$1 AND id=$2`, tbl)
	}
	_, err := r.DB.ExecContext(ctx, q, tenant, id)
	return err
}
