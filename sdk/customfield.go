package sdk

import (
	"context"
	"fmt"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	metapkg "github.com/faciam-dev/gcfm/meta"
	"github.com/faciam-dev/gcfm/pkg/monitordb"
)

//
// Custom field CRUD follows a two-phase flow:
//   1. Read/modify the target database using its own connection/transaction.
//   2. Persist metadata to the MetaDB in a separate transaction.
//
// Distributed transactions are not used. Audit logs and notifications (when
// enabled) are triggered only after the metadata transaction commits.

// ListCustomFields returns custom field metadata filtered by database and table.
func (s *service) ListCustomFields(ctx context.Context, dbID int64, table string) ([]registry.FieldMeta, error) {
	tgt, err := s.pickTarget(ctx)
	if err != nil {
		return nil, err
	}
	metas, err := registry.LoadSQLByDB(ctx, tgt.DB, registry.DBConfig{Driver: tgt.Driver, Schema: tgt.Schema}, "default", dbID)
	if err != nil {
		return nil, err
	}
	if table != "" {
		var filtered []registry.FieldMeta
		for _, m := range metas {
			if m.TableName == table {
				filtered = append(filtered, m)
			}
		}
		metas = filtered
	}
	return metas, nil
}

func (s *service) columnExists(ctx context.Context, tgt TargetConn, table, column string) (bool, error) {
	var q string
	switch tgt.Driver {
	case "postgres":
		q = `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=$1 AND table_name=$2 AND column_name=$3`
	case "mysql":
		q = `SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND COLUMN_NAME=?`
	default:
		return false, fmt.Errorf("unsupported driver: %s", tgt.Driver)
	}
	var count int
	err := tgt.DB.QueryRowContext(ctx, q, tgt.Schema, table, column).Scan(&count)
	return count > 0, err
}

// CreateCustomField adds a column to the target database and records metadata in
// the MetaDB. Target and meta operations are executed in separate transactions.
func (s *service) CreateCustomField(ctx context.Context, fm registry.FieldMeta) error {
	tgt, err := s.pickTarget(ctx)
	if err != nil {
		return err
	}
	exists, err := s.columnExists(ctx, tgt, fm.TableName, fm.ColumnName)
	if err != nil {
		return err
	}
	if !exists {
		if err := registry.AddColumnSQL(ctx, tgt.DB, tgt.Driver, fm.TableName, fm.ColumnName, fm.DataType, nil, nil, nil); err != nil {
			return err
		}
	}
	fm.DBID = monitordb.DefaultDBID
	return s.meta.UpsertFieldDefs(ctx, nil, []metapkg.FieldDef{fm})
}

// UpdateCustomField alters a column in the target database and synchronizes the
// meta store. Operations on the target and MetaDB are executed independently.
func (s *service) UpdateCustomField(ctx context.Context, fm registry.FieldMeta) error {
	tgt, err := s.pickTarget(ctx)
	if err != nil {
		return err
	}
	exists, err := s.columnExists(ctx, tgt, fm.TableName, fm.ColumnName)
	if err != nil {
		return err
	}
	if exists {
		if err := registry.ModifyColumnSQL(ctx, tgt.DB, tgt.Driver, fm.TableName, fm.ColumnName, fm.DataType, nil, nil, nil); err != nil {
			return err
		}
	} else {
		if err := registry.AddColumnSQL(ctx, tgt.DB, tgt.Driver, fm.TableName, fm.ColumnName, fm.DataType, nil, nil, nil); err != nil {
			return err
		}
	}
	fm.DBID = monitordb.DefaultDBID
	return s.meta.UpsertFieldDefs(ctx, nil, []metapkg.FieldDef{fm})
}

// DeleteCustomField drops a column from the target database and removes its
// metadata. Each database operation uses its own transaction with no attempt at
// distributed commits.
func (s *service) DeleteCustomField(ctx context.Context, table, column string) error {
	tgt, err := s.pickTarget(ctx)
	if err != nil {
		return err
	}
	if err := registry.DropColumnSQL(ctx, tgt.DB, tgt.Driver, table, column); err != nil {
		return err
	}
	fm := registry.FieldMeta{DBID: monitordb.DefaultDBID, TableName: table, ColumnName: column}
	return s.meta.DeleteFieldDefs(ctx, nil, []metapkg.FieldDef{fm})
}
