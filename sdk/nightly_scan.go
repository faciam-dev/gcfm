package sdk

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	metapkg "github.com/faciam-dev/gcfm/meta"
)

// NightlyScan enumerates tables across all registered targets and records the
// results in the MetaDB. Each target is scanned independently and results are
// stored using a MetaDB transaction.
func (s *service) NightlyScan(ctx context.Context) error {
	return s.targets.ForEach(func(key string, tgt TargetConn) error {
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
		return tx.Commit()
	})
}

func listTables(ctx context.Context, tgt TargetConn) ([]string, error) {
	var (
		rows *sql.Rows
		err  error
	)
	switch tgt.Driver {
	case "postgres":
		rows, err = tgt.DB.QueryContext(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema=$1`, tgt.Schema)
	case "mysql":
		rows, err = tgt.DB.QueryContext(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema=?`, tgt.Schema)
	case "sqlite3":
		rows, err = tgt.DB.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table'`)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", tgt.Driver)
	}
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
}
