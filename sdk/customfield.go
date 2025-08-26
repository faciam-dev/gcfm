package sdk

import (
	"context"

	metapkg "github.com/faciam-dev/gcfm/meta"
	"github.com/faciam-dev/gcfm/pkg/monitordb"
	"github.com/faciam-dev/gcfm/pkg/registry"
)

//
// Custom field CRUD follows a two-phase flow:
//   1. Read/modify the target database using its own connection/transaction.
//   2. Persist metadata to the MetaDB in a separate transaction.
//
// Distributed transactions are not used. Audit logs and notifications (when
// enabled) are triggered only after the metadata transaction commits.

// listFromTarget loads custom field metadata from the target database.
func (s *service) listFromTarget(ctx context.Context, dbID int64, table string) ([]registry.FieldMeta, error) {
	dec, ok := s.resolveDecision(ctx)
	if !ok {
		return nil, ErrNoTarget
	}
	var metas []registry.FieldMeta
	err := s.RunWithTarget(ctx, dec, false, func(t TargetConn) error {
		cfg := registry.DBConfig{Driver: t.Driver, Schema: t.Schema, TablePrefix: registry.DefaultTablePrefix}
		m, e := registry.LoadSQLByDB(ctx, t.DB, cfg, "default", dbID)
		if e == nil {
			metas = m
		}
		return e
	})
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

// ListCustomFields returns custom field metadata filtered by database and table.
func (s *service) ListCustomFields(ctx context.Context, dbID int64, table string) ([]registry.FieldMeta, error) {
	switch s.readSource {
	case ReadFromMeta:
		return s.listFromMeta(ctx, dbID, table)
	case ReadAuto:
		defs, err := s.listFromTarget(ctx, dbID, table)
		if err == nil && len(defs) > 0 {
			return defs, nil
		}
		s.logger.Warn("target read empty/fail; fallback to meta", "dbID", dbID, "table", table, "err", err)
		return s.listFromMeta(ctx, dbID, table)
	case ReadFromTarget:
		fallthrough
	default:
		return s.listFromTarget(ctx, dbID, table)
	}
}

// listFromMeta loads custom field metadata from the MetaDB.
func (s *service) listFromMeta(ctx context.Context, dbID int64, table string) ([]registry.FieldMeta, error) {
	defs, err := s.meta.ListFieldDefs(ctx, "default")
	if err != nil {
		return nil, err
	}
	var filtered []registry.FieldMeta
	for _, d := range defs {
		if d.DBID != dbID {
			continue
		}
		if table != "" && d.TableName != table {
			continue
		}
		filtered = append(filtered, d)
	}
	return filtered, nil
}

func (s *service) addColumn(ctx context.Context, dec TargetDecision, fm registry.FieldMeta) error {
	return s.RunWithTarget(ctx, dec, true, func(t TargetConn) error {
		exists, err := registry.ColumnExists(ctx, t.DB, t.Dialect, t.Schema, fm.TableName, fm.ColumnName)
		if err != nil {
			return err
		}
		if !exists {
			return registry.AddColumnSQL(ctx, t.DB, t.Driver, fm.TableName, fm.ColumnName, fm.DataType, nil, nil, registry.UnifiedDefault{})
		}
		return nil
	})
}

func (s *service) upsertColumn(ctx context.Context, dec TargetDecision, fm registry.FieldMeta) error {
	return s.RunWithTarget(ctx, dec, true, func(t TargetConn) error {
		exists, err := registry.ColumnExists(ctx, t.DB, t.Dialect, t.Schema, fm.TableName, fm.ColumnName)
		if err != nil {
			return err
		}
		if exists {
			return registry.ModifyColumnSQL(ctx, t.DB, t.Driver, fm.TableName, fm.ColumnName, fm.DataType, nil, nil, registry.UnifiedDefault{})
		}
		return registry.AddColumnSQL(ctx, t.DB, t.Driver, fm.TableName, fm.ColumnName, fm.DataType, nil, nil, registry.UnifiedDefault{})
	})
}

func (s *service) dropColumn(ctx context.Context, dec TargetDecision, table, column string) error {
	return s.RunWithTarget(ctx, dec, true, func(t TargetConn) error {
		return registry.DropColumnSQL(ctx, t.DB, t.Driver, table, column)
	})
}

func (s *service) persistMeta(ctx context.Context, dec TargetDecision, fm registry.FieldMeta) error {
	fm.DBID = monitordb.DefaultDBID
	if err := s.meta.UpsertFieldDefs(ctx, nil, []metapkg.FieldDef{fm}); err != nil {
		return err
	}
	return s.RunWithTarget(ctx, dec, true, func(t TargetConn) error {
		return registry.UpsertFieldDefByDB(ctx, t.DB, t.Driver, registry.DefaultTablePrefix, "default", fm)
	})
}

func (s *service) removeMeta(ctx context.Context, dec TargetDecision, fm registry.FieldMeta) error {
	if err := s.meta.DeleteFieldDefs(ctx, nil, []metapkg.FieldDef{fm}); err != nil {
		return err
	}
	return s.RunWithTarget(ctx, dec, true, func(t TargetConn) error {
		return registry.DeleteFieldDefByDB(ctx, t.DB, t.Driver, registry.DefaultTablePrefix, fm)
	})
}

// CreateCustomField adds a column to the target database and records metadata in
// the MetaDB. Target and meta operations are executed in separate transactions.
func (s *service) CreateCustomField(ctx context.Context, fm registry.FieldMeta) error {
	dec, ok := s.resolveDecision(ctx)
	if !ok {
		return ErrNoTarget
	}
	if err := s.addColumn(ctx, dec, fm); err != nil {
		return err
	}
	return s.persistMeta(ctx, dec, fm)
}

// UpdateCustomField alters a column in the target database and synchronizes the
// meta store. Operations on the target and MetaDB are executed independently.
func (s *service) UpdateCustomField(ctx context.Context, fm registry.FieldMeta) error {
	dec, ok := s.resolveDecision(ctx)
	if !ok {
		return ErrNoTarget
	}
	if err := s.upsertColumn(ctx, dec, fm); err != nil {
		return err
	}
	return s.persistMeta(ctx, dec, fm)
}

// DeleteCustomField drops a column from the target database and removes its
// metadata. Each database operation uses its own transaction with no attempt at
// distributed commits.
func (s *service) DeleteCustomField(ctx context.Context, table, column string) error {
	dec, ok := s.resolveDecision(ctx)
	if !ok {
		return ErrNoTarget
	}
	if err := s.dropColumn(ctx, dec, table, column); err != nil {
		return err
	}
	fm := registry.FieldMeta{DBID: monitordb.DefaultDBID, TableName: table, ColumnName: column}
	return s.removeMeta(ctx, dec, fm)
}
