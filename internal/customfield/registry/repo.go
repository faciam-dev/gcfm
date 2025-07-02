package registry

import (
	"context"
	"database/sql"
)

// Repo provides helper database operations for metrics.
type Repo struct {
	DB     *sql.DB
	Driver string
}

// CountFieldsByTable returns the number of custom fields per table.
func (r *Repo) CountFieldsByTable(ctx context.Context) (map[string]int, error) {
	if r == nil || r.DB == nil {
		return nil, nil
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT table_name, COUNT(*) FROM gcfm_custom_fields GROUP BY table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make(map[string]int)
	for rows.Next() {
		var table string
		var n int
		if err := rows.Scan(&table, &n); err != nil {
			return nil, err
		}
		res[table] = n
	}
	return res, rows.Err()
}
