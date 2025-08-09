package monitordb

import (
	"context"
	"database/sql"
	"errors"
)

type Record struct {
	ID     int64
	Driver string
	DSN    string
	Schema string // postgres schema, empty for MySQL
}

var ErrNotFound = errors.New("monitored database not found")

func GetByID(ctx context.Context, db *sql.DB, tenant string, id int64) (Record, error) {
	var rec Record
	// if monitored_databases table lacks tenant_id, remove tenant condition accordingly
	err := db.QueryRowContext(ctx,
		`SELECT id, driver, dsn, COALESCE(schema_name,'') FROM monitored_databases WHERE id=? AND tenant_id=?`,
		id, tenant).Scan(&rec.ID, &rec.Driver, &rec.DSN, &rec.Schema)
	if err == sql.ErrNoRows {
		return Record{}, ErrNotFound
	}
	return rec, err
}
