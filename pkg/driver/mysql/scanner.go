package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/faciam-dev/gcfm/pkg/registry"
)

// Scanner reads MySQL metadata.
type Scanner struct {
	db *sql.DB
}

// NewScanner returns a new Scanner.
func NewScanner(db *sql.DB) *Scanner {
	return &Scanner{db: db}
}

// Scan retrieves column metadata for the given schema.
func (s *Scanner) Scan(ctx context.Context, conf registry.DBConfig) ([]registry.FieldMeta, error) {
	const colQuery = `SELECT TABLE_NAME, COLUMN_NAME, DATA_TYPE, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT
FROM information_schema.columns
WHERE table_schema = ? AND table_name != ?
ORDER BY TABLE_NAME, ORDINAL_POSITION`
	rows, err := s.db.QueryContext(ctx, colQuery, conf.Schema, conf.TablePrefix+"custom_fields")
	if err != nil {
		return nil, fmt.Errorf("query columns: %w", err)
	}
	defer rows.Close()

	var metas []registry.FieldMeta
	storeKind := registry.DefaultStoreKindForDriver(conf.Driver)
	for rows.Next() {
		var (
			table, column, dataType, columnType, isNullable string
			def                                             sql.NullString
		)
		if err := rows.Scan(&table, &column, &dataType, &columnType, &isNullable, &def); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		physical := columnType
		if physical == "" {
			physical = dataType
		}
		m := registry.FieldMeta{
			TableName:    table,
			ColumnName:   column,
			DataType:     dataType,
			StoreKind:    storeKind,
			Kind:         registry.GuessSQLKind(dataType),
			PhysicalType: registry.SQLPhysicalType(conf.Driver, physical),
		}
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

	const uniqQuery = `SELECT TABLE_NAME, INDEX_NAME, COLUMN_NAME
FROM information_schema.statistics
WHERE table_schema = ? AND NON_UNIQUE = 0`
	urows, err := s.db.QueryContext(ctx, uniqQuery, conf.Schema)
	if err != nil {
		return nil, fmt.Errorf("unique index query: %w", err)
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
