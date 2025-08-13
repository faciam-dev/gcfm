package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

type Scanner struct {
	db *sql.DB
}

func NewScanner(db *sql.DB) *Scanner {
	return &Scanner{db: db}
}

func (s *Scanner) Scan(ctx context.Context, conf registry.DBConfig) ([]registry.FieldMeta, error) {
	tbl := conf.TablePrefix + "custom_fields"
	q := fmt.Sprintf("SELECT TABLE_NAME, COLUMN_NAME, DATA_TYPE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME != '%s' ORDER BY TABLE_NAME, ORDINAL_POSITION", tbl)
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
