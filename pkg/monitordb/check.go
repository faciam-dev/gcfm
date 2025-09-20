package monitordb

import (
	"context"
	"database/sql"
	"net/url"
	"strings"

	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

func TableExists(ctx context.Context, db *sql.DB, dialect ormdriver.Dialect, schema, table string) (bool, error) {
	q := query.New(db, "information_schema.tables", dialect).
		SelectRaw("COUNT(*) as cnt").
		WhereRaw("LOWER(table_name) = LOWER(:t)", map[string]any{"t": table})
	switch dialect.(type) {
	case ormdriver.PostgresDialect:
		if schema == "" {
			schema = "public"
		}
		q.Where("table_schema", schema)
	case ormdriver.MySQLDialect:
		q.WhereRaw("table_schema = DATABASE()", nil)
	default:
		q.Where("table_schema", schema)
	}
	var res struct {
		Cnt int `db:"cnt"`
	}
	if err := q.WithContext(ctx).First(&res); err != nil {
		return false, err
	}
	return res.Cnt > 0, nil
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
	case "mongo":
		u, err := url.Parse(dsn)
		if err != nil {
			return false
		}
		if strings.TrimPrefix(u.Path, "/") != "" {
			return true
		}
		if u.Query().Get("authSource") != "" {
			return true
		}
		return false
	default:
		return false
	}
}
