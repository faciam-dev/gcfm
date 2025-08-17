package sdk

import (
	"context"
	"fmt"
	"time"

	metapkg "github.com/faciam-dev/gcfm/meta"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

// NightlyScan enumerates tables across all registered targets and records the
// results in the MetaDB. Each target is scanned independently and results are
// stored using a MetaDB transaction.
func (s *service) NightlyScan(ctx context.Context) error {
	for key, tgt := range s.targets.Snapshot() {
		tables, err := listTables(ctx, tgt)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		tx, err := s.meta.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		for _, tb := range tables {
			res := metapkg.ScanResult{
				TenantID:   key,
				ScanID:     tb,
				Status:     "ok",
				StartedAt:  time.Now(),
				FinishedAt: time.Now(),
			}
			if err := s.meta.RecordScanResult(ctx, tx, res); err != nil {
				if rbErr := tx.Rollback(); rbErr != nil {
					fmt.Printf("rollback failed for tenant %s: %v\n", key, rbErr)
				}
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func listTables(ctx context.Context, tgt TargetConn) ([]string, error) {
	switch tgt.Driver {
	case "postgres", "mysql":
		q := query.New(tgt.DB, "information_schema.tables", tgt.Dialect).
			Select("table_name")
		switch tgt.Dialect.(type) {
		case ormdriver.PostgresDialect:
			schema := tgt.Schema
			if schema == "" {
				schema = "public"
			}
			q.Where("table_schema", schema)
		case ormdriver.MySQLDialect:
			if tgt.Schema != "" {
				q.Where("table_schema", tgt.Schema)
			} else {
				q.WhereRaw("table_schema = DATABASE()", nil)
			}
		}
		type row struct {
			Name string `db:"table_name"`
		}
		var rows []row
		if err := q.WithContext(ctx).Get(&rows); err != nil {
			return nil, err
		}
		tables := make([]string, len(rows))
		for i, r := range rows {
			tables[i] = r.Name
		}
		return tables, nil
	case "sqlite3":
		rows, err := tgt.DB.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table'`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var tables []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			tables = append(tables, name)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return tables, nil
	default:
		return nil, fmt.Errorf("unsupported driver: %s", tgt.Driver)
	}
}
