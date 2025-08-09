package monitordb

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
)

func TableExists(ctx context.Context, db *sql.DB, driver, schema, table string) (bool, error) {
	switch driver {
	case "mysql":
		var n int
		err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`, table).Scan(&n)
		return n > 0, err
	case "postgres":
		if schema == "" {
			schema = "public"
		}
		var n int
		err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2`, schema, table).Scan(&n)
		return n > 0, err
	default:
		return false, fmt.Errorf("unsupported driver: %s", driver)
	}
}

// HasDatabaseName returns true if DSN appears to include a database name.
func HasDatabaseName(driver, dsn string) bool {
	switch driver {
	case "mysql":
		idx := strings.LastIndex(dsn, "/")
		if idx == -1 {
			return false
		}
		rest := dsn[idx+1:]
		if rest == "" || strings.HasPrefix(rest, "?") {
			return false
		}
		return true
	case "postgres":
		if strings.Contains(dsn, "dbname=") {
			return true
		}
		u, err := url.Parse(dsn)
		if err == nil && strings.TrimPrefix(u.Path, "/") != "" {
			return true
		}
		return false
	default:
		return false
	}
}
