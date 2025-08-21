package monitordb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

// Database represents a monitored database connection.
type Database struct {
	ID        int64
	TenantID  string
	Name      string
	Driver    string
	DSN       string
	DSNEnc    []byte
	CreatedAt time.Time
}

// Repo manages monitored database records.
type Repo struct {
	DB          *sql.DB
	Dialect     ormdriver.Dialect
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
	if r == nil || r.DB == nil {
		return 0, fmt.Errorf("repo not initialized")
	}
	tbl := r.table()
	data := map[string]any{
		"tenant_id": d.TenantID,
		"name":      d.Name,
		"driver":    d.Driver,
	}
	if d.DSN != "" {
		data["dsn"] = d.DSN
	}
	if len(d.DSNEnc) > 0 {
		data["dsn_enc"] = d.DSNEnc
	}
	q := query.New(r.DB, tbl, r.Dialect).WithContext(ctx)
	id, err := q.InsertGetId(data)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// List returns all monitored databases for a tenant.
func (r *Repo) List(ctx context.Context, tenant string) ([]Database, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("repo not initialized")
	}
	var res []Database
	q := query.New(r.DB, r.table(), r.Dialect).
		Select("id", "tenant_id", "name", "driver", "dsn", "dsn_enc", "created_at").
		Where("tenant_id", tenant).
		OrderBy("id", "asc").
		WithContext(ctx)
	if err := q.Get(&res); err != nil {
		return nil, err
	}
	return res, nil
}

// ListAll returns all monitored databases.
func (r *Repo) ListAll(ctx context.Context) ([]Database, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("repo not initialized")
	}
	var res []Database
	q := query.New(r.DB, r.table(), r.Dialect).
		Select("id", "tenant_id", "name", "driver", "dsn", "dsn_enc", "created_at").
		OrderBy("id", "asc").
		WithContext(ctx)
	if err := q.Get(&res); err != nil {
		return nil, err
	}
	return res, nil
}

// Get fetches a database by tenant and ID.
func (r *Repo) Get(ctx context.Context, tenant string, id int64) (Database, error) {
	if r == nil || r.DB == nil {
		return Database{}, fmt.Errorf("repo not initialized")
	}
	var d Database
	q := query.New(r.DB, r.table(), r.Dialect).
		Select("id", "tenant_id", "name", "driver", "dsn", "dsn_enc", "created_at").
		Where("tenant_id", tenant).
		Where("id", id).
		WithContext(ctx)
	if err := q.First(&d); err != nil {
		return d, err
	}
	return d, nil
}

// Update modifies an existing monitored database's attributes.
func (r *Repo) Update(ctx context.Context, tenant string, id int64, name, driver string, dsnEnc []byte) error {
	if r == nil || r.DB == nil {
		return fmt.Errorf("repo not initialized")
	}
	data := map[string]any{"name": name, "driver": driver, "dsn_enc": dsnEnc}
	q := query.New(r.DB, r.table(), r.Dialect).
		Where("tenant_id", tenant).
		Where("id", id).
		WithContext(ctx)
	_, err := q.Update(data)
	return err
}

// Delete removes a monitored database.
func (r *Repo) Delete(ctx context.Context, tenant string, id int64) error {
	if r == nil || r.DB == nil {
		return fmt.Errorf("repo not initialized")
	}
	q := query.New(r.DB, r.table(), r.Dialect).
		Where("tenant_id", tenant).
		Where("id", id).
		WithContext(ctx)
	_, err := q.Delete()
	return err
}
