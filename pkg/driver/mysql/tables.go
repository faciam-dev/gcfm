package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/faciam-dev/gcfm/pkg/metadata"
)

// ListTables returns tables and comments from the given schema.
func ListTables(ctx context.Context, db *sql.DB, schema string) ([]metadata.Table, error) {
	const q = `SELECT TABLE_NAME, TABLE_COMMENT FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=? ORDER BY TABLE_NAME`
	rows, err := db.QueryContext(ctx, q, schema)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var tables []metadata.Table
	for rows.Next() {
		var t metadata.Table
		if err := rows.Scan(&t.Name, &t.Comment); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		tables = append(tables, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return tables, nil
}
