package snapshot

import (
	"context"
	"database/sql"

	"github.com/faciam-dev/gcfm/pkg/schema"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

const registryVersion = "0.3"

// Registry represents a minimal registry YAML structure.
type Registry struct {
	Version string         `yaml:"version"`
	Fields  []schema.Field `yaml:"fields"`
}

// ExportRegistry retrieves registry information for a tenant from the database.
func ExportRegistry(ctx context.Context, db *sql.DB, dialect ormdriver.Dialect, prefix, tid string) (*Registry, error) {
	tbl := prefix + "custom_fields"
	q := query.New(db, tbl, dialect).
		Select("table_name", "column_name", "data_type").
		Where("tenant_id", tid).
		OrderBy("db_id", "asc").
		OrderBy("table_name", "asc").
		OrderBy("column_name", "asc").
		WithContext(ctx)
	var rows []struct {
		TableName  string `db:"table_name"`
		ColumnName string `db:"column_name"`
		DataType   string `db:"data_type"`
	}
	if err := q.Get(&rows); err != nil {
		return nil, err
	}
	f := make([]schema.Field, 0, len(rows))
	for _, r := range rows {
		f = append(f, schema.Field{Table: r.TableName, Column: r.ColumnName, Type: r.DataType})
	}
	return &Registry{Version: registryVersion, Fields: f}, nil
}
