package sdk

import (
	"context"
	"fmt"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

// ListCustomFields returns custom field metadata filtered by table.
func (s *service) ListCustomFields(ctx context.Context, table string) ([]registry.FieldMeta, error) {
	if s.db == nil {
		return nil, fmt.Errorf("db not set")
	}
	metas, err := registry.LoadSQL(ctx, s.db, registry.DBConfig{Driver: s.driver, Schema: s.schema})
	if err != nil {
		return nil, err
	}
	if table != "" {
		filtered := metas[:0]
		for _, m := range metas {
			if m.TableName == table {
				filtered = append(filtered, m)
			}
		}
		metas = filtered
	}
	return metas, nil
}

func (s *service) columnExists(ctx context.Context, table, column string) (bool, error) {
	var q string
	switch s.driver {
	case "postgres":
		q = `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=$1 AND table_name=$2 AND column_name=$3`
	case "mysql":
		q = `SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND COLUMN_NAME=?`
	default:
		return false, fmt.Errorf("unsupported driver: %s", s.driver)
	}
	var count int
	err := s.db.QueryRowContext(ctx, q, s.schema, table, column).Scan(&count)
	return count > 0, err
}

func (s *service) CreateCustomField(ctx context.Context, fm registry.FieldMeta) error {
	if s.db == nil {
		return fmt.Errorf("db not set")
	}
	exists, err := s.columnExists(ctx, fm.TableName, fm.ColumnName)
	if err != nil {
		return err
	}
	if !exists {
		if err := registry.AddColumnSQL(ctx, s.db, s.driver, fm.TableName, fm.ColumnName, fm.DataType, nil, nil, nil); err != nil {
			return err
		}
	}
	return registry.UpsertSQL(ctx, s.db, s.driver, []registry.FieldMeta{fm})
}

func (s *service) UpdateCustomField(ctx context.Context, fm registry.FieldMeta) error {
	if s.db == nil {
		return fmt.Errorf("db not set")
	}
	exists, err := s.columnExists(ctx, fm.TableName, fm.ColumnName)
	if err != nil {
		return err
	}
	if exists {
		if err := registry.ModifyColumnSQL(ctx, s.db, s.driver, fm.TableName, fm.ColumnName, fm.DataType, nil, nil, nil); err != nil {
			return err
		}
	} else {
		if err := registry.AddColumnSQL(ctx, s.db, s.driver, fm.TableName, fm.ColumnName, fm.DataType, nil, nil, nil); err != nil {
			return err
		}
	}
	return registry.UpsertSQL(ctx, s.db, s.driver, []registry.FieldMeta{fm})
}

func (s *service) DeleteCustomField(ctx context.Context, table, column string) error {
	if s.db == nil {
		return fmt.Errorf("db not set")
	}
	if err := registry.DropColumnSQL(ctx, s.db, s.driver, table, column); err != nil {
		return err
	}
	fm := registry.FieldMeta{TableName: table, ColumnName: column}
	return registry.DeleteSQL(ctx, s.db, s.driver, []registry.FieldMeta{fm})
}
