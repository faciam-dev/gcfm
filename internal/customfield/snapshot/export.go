package snapshot

import (
	"context"
	"database/sql"

	"github.com/faciam-dev/gcfm/internal/api/schema"
)

const registryVersion = "0.3"

// Registry represents a minimal registry YAML structure.
type Registry struct {
	Version string         `yaml:"version"`
	Fields  []schema.Field `yaml:"fields"`
}

// ExportRegistry retrieves registry information for a tenant from the database.
func ExportRegistry(ctx context.Context, db *sql.DB, drv, tid string) (*Registry, error) {
	var query string
	switch drv {
	case "postgres":
		query = `SELECT table_name, column_name, data_type FROM gcfm_custom_fields WHERE tenant_id=$1 ORDER BY db_id, table_name, column_name`
	default:
		query = `SELECT table_name, column_name, data_type FROM gcfm_custom_fields WHERE tenant_id=? ORDER BY db_id, table_name, column_name`
	}
	rows, err := db.QueryContext(ctx, query, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var f []schema.Field
	for rows.Next() {
		var t, c, typ string
		if err := rows.Scan(&t, &c, &typ); err != nil {
			return nil, err
		}
		f = append(f, schema.Field{Table: t, Column: c, Type: typ})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &Registry{Version: registryVersion, Fields: f}, nil
}
