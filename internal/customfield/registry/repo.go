package registry

import (
	"context"
	"database/sql"

	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

// Repo provides helper database operations for metrics.
type Repo struct {
	DB          *sql.DB
	Dialect     ormdriver.Dialect
	TablePrefix string
}

// CountFieldsByTable returns the number of custom fields per table.
func (r *Repo) CountFieldsByTable(ctx context.Context) (map[string]int, error) {
	if r == nil || r.DB == nil {
		return nil, nil
	}
	tbl := r.TablePrefix + "custom_fields"
	q := query.New(r.DB, tbl, r.Dialect).
		Select("table_name").
		SelectRaw("COUNT(*) as cnt").
		GroupBy("table_name").
		WithContext(ctx)

	type row struct {
		Table string `db:"table_name"`
		Cnt   int    `db:"cnt"`
	}
	var rows []row
	if err := q.Get(&rows); err != nil {
		return nil, err
	}
	res := make(map[string]int, len(rows))
	for _, r := range rows {
		res[r.Table] = r.Cnt
	}
	return res, nil
}
