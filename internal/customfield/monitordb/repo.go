package monitordb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	if err != nil {
		return Record{}, err
	}
	if rec.DSN == "" {
		// attempt legacy column fallback
		var host, user, pass, dbname sql.NullString
		var port sql.NullInt64
		_ = db.QueryRowContext(ctx,
			`SELECT host, username, password, database_name, port FROM monitored_databases WHERE id=? AND tenant_id=?`,
			id, tenant).Scan(&host, &user, &pass, &dbname, &port)
		if host.Valid && user.Valid && dbname.Valid && port.Valid {
			rec.Driver = "mysql"
			rec.DSN = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true", user.String, pass.String, host.String, port.Int64, dbname.String)
		}
		if rec.DSN == "" {
			return Record{}, fmt.Errorf("monitored database (id=%d) has empty DSN; run migration 0017 and set dsn", id)
		}
	}
	if rec.Driver == "" {
		rec.Driver = "mysql"
	}
	return rec, nil
}
