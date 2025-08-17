package sqlscan

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

// Scanner scans database schema information_schema for custom field metadata.
type Scanner struct {
	db    *sql.DB
	query string
}

// New creates a new SQL scanner with the given query template.
// The query should contain a placeholder for the schema argument and a
// %s verb for the table name to exclude.
func New(db *sql.DB, query string) *Scanner {
	return &Scanner{db: db, query: query}
}

// Scan retrieves field metadata using the provided DB configuration.
func (s *Scanner) Scan(ctx context.Context, conf registry.DBConfig) ([]registry.FieldMeta, error) {
	q := fmt.Sprintf(s.query, conf.TablePrefix+"custom_fields")
	rows, err := s.db.QueryContext(ctx, q, conf.Schema)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var metas []registry.FieldMeta
	for rows.Next() {
		var m registry.FieldMeta
		if err := rows.Scan(&m.TableName, &m.ColumnName, &m.DataType); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		metas = append(metas, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return metas, nil
}
