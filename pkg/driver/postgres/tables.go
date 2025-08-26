package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/faciam-dev/gcfm/pkg/metadata"
)

// ListTables returns tables and comments from the given schema.
func ListTables(ctx context.Context, db *sql.DB, schema string) ([]metadata.Table, error) {
	const q = `SELECT table_name, obj_description((quote_ident(table_schema)||'.'||quote_ident(table_name))::regclass) AS comment FROM information_schema.tables WHERE table_schema=$1 AND table_type='BASE TABLE' ORDER BY table_name`
	rows, err := db.QueryContext(ctx, q, schema)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var tables []metadata.Table
	for rows.Next() {
		var t metadata.Table
		var comment sql.NullString
		if err := rows.Scan(&t.Name, &comment); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if comment.Valid {
			t.Comment = comment.String
		}
		tables = append(tables, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return tables, nil
}
