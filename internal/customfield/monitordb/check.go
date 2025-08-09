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
		err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND LOWER(table_name) = LOWER(?)`, table).Scan(&n)
		return n > 0, err
	case "postgres":
		if schema == "" {
			schema = "public"
		}
		var ok bool
		err := db.QueryRowContext(ctx, `SELECT to_regclass($1||'.'||$2) IS NOT NULL`, schema, table).Scan(&ok)
		return ok, err
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
