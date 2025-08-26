package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/faciam-dev/gcfm/pkg/registry"
)

// Scanner reads PostgreSQL metadata.
type Scanner struct {
	db *sql.DB
}

// NewScanner returns a new Scanner.
func NewScanner(db *sql.DB) *Scanner { return &Scanner{db: db} }

// Scan retrieves column metadata for the given schema.
func (s *Scanner) Scan(ctx context.Context, conf registry.DBConfig) ([]registry.FieldMeta, error) {
	const colQuery = `SELECT table_name, column_name, data_type, is_nullable, column_default
FROM information_schema.columns
WHERE table_schema = $1 AND table_name <> $2
ORDER BY table_name, ordinal_position`
	rows, err := s.db.QueryContext(ctx, colQuery, conf.Schema, conf.TablePrefix+"custom_fields")
	if err != nil {
		return nil, fmt.Errorf("query columns: %w", err)
	}
	defer rows.Close()

	var metas []registry.FieldMeta
	for rows.Next() {
		var (
			table, column, dataType, isNullable string
			def                                 sql.NullString
		)
		if err := rows.Scan(&table, &column, &dataType, &isNullable, &def); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		m := registry.FieldMeta{TableName: table, ColumnName: column, DataType: dataType}
		if isNullable == "YES" {
			m.Nullable = true
		}
		if def.Valid {
			m.HasDefault = true
			v := def.String
			m.Default = &v
		}
		metas = append(metas, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	const uniqQuery = `SELECT t.relname, i.relname, a.attname
FROM pg_index ix
JOIN pg_class t ON t.oid = ix.indrelid
JOIN pg_class i ON i.oid = ix.indexrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
JOIN unnest(ix.indkey) AS k(attnum) ON true
JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
WHERE n.nspname = $1 AND ix.indisunique`
	urows, err := s.db.QueryContext(ctx, uniqQuery, conf.Schema)
	if err != nil {
		return nil, fmt.Errorf("query unique indexes: %w", err)
	}
	defer urows.Close()

	type key struct{ table, index string }
	idx := make(map[key][]string)
	for urows.Next() {
		var tbl, idxName, col string
		if err := urows.Scan(&tbl, &idxName, &col); err != nil {
			return nil, fmt.Errorf("unique scan: %w", err)
		}
		k := key{tbl, idxName}
		idx[k] = append(idx[k], col)
	}
	if err := urows.Err(); err != nil {
		return nil, fmt.Errorf("unique rows: %w", err)
	}

	uniqueCols := make(map[string]map[string]struct{})
	for k, cols := range idx {
		if len(cols) == 1 && k.table != conf.TablePrefix+"custom_fields" {
			if uniqueCols[k.table] == nil {
				uniqueCols[k.table] = make(map[string]struct{})
			}
			uniqueCols[k.table][cols[0]] = struct{}{}
		}
	}
	for i := range metas {
		if cols, ok := uniqueCols[metas[i].TableName]; ok {
			if _, ok := cols[metas[i].ColumnName]; ok {
				metas[i].Unique = true
			}
		}
	}

	return metas, nil
}
