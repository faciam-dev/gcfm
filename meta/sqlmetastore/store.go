package sqlmetastore

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	metapkg "github.com/faciam-dev/gcfm/meta"
	"github.com/faciam-dev/gcfm/pkg/monitordb"
)

// SQLMetaStore implements MetaStore using an SQL database.
type SQLMetaStore struct {
	db     *sql.DB
	driver string
	schema string
}

// NewSQLMetaStore initializes a SQLMetaStore with the given connection.
func NewSQLMetaStore(db *sql.DB, driver, schema string) *SQLMetaStore {
	return &SQLMetaStore{db: db, driver: driver, schema: schema}
}

// BeginTx starts a transaction using the underlying database.
func (s *SQLMetaStore) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, opts)
}

// table returns a fully qualified table name for metadata tables.
func (s *SQLMetaStore) table(name string) string {
	tbl := "gcfm_" + name
	if s.schema != "" {
		return fmt.Sprintf("%s.%s", s.schema, tbl)
	}
	return tbl
}

// UpsertFieldDefs stores or updates field definitions.
func (s *SQLMetaStore) UpsertFieldDefs(ctx context.Context, tx *sql.Tx, defs []metapkg.FieldDef) error {
	if len(defs) == 0 {
		return nil
	}
	ownTx := false
	if tx == nil {
		var err error
		tx, err = s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		ownTx = true
	}
	tbl := s.table("custom_fields")
	var (
		stmt *sql.Stmt
		err  error
	)
	switch s.driver {
	case "postgres":
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf(`INSERT INTO %s (db_id, table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, "unique", has_default, default_value, validator, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW(),NOW()) ON CONFLICT (db_id, tenant_id, table_name, column_name) DO UPDATE SET data_type=EXCLUDED.data_type, label_key=EXCLUDED.label_key, widget=EXCLUDED.widget, placeholder_key=EXCLUDED.placeholder_key, nullable=EXCLUDED.nullable, "unique"=EXCLUDED."unique", has_default=EXCLUDED.has_default, default_value=EXCLUDED.default_value, validator=EXCLUDED.validator, updated_at=NOW()`, tbl))
	case "mysql":
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf("INSERT INTO %s (db_id, table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, `unique`, has_default, default_value, validator, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW()) ON DUPLICATE KEY UPDATE data_type=VALUES(data_type), label_key=VALUES(label_key), widget=VALUES(widget), placeholder_key=VALUES(placeholder_key), nullable=VALUES(nullable), `unique`=VALUES(`unique`), has_default=VALUES(has_default), default_value=VALUES(default_value), validator=VALUES(validator), updated_at=NOW()", tbl))
	default:
		// Assume drivers using '?' placeholders and supporting ON CONFLICT.
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf(`INSERT INTO %s (db_id, table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, "unique", has_default, default_value, validator, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) ON CONFLICT (db_id, tenant_id, table_name, column_name) DO UPDATE SET data_type=excluded.data_type, label_key=excluded.label_key, widget=excluded.widget, placeholder_key=excluded.placeholder_key, nullable=excluded.nullable, "unique"=excluded."unique", has_default=excluded.has_default, default_value=excluded.default_value, validator=excluded.validator, updated_at=CURRENT_TIMESTAMP`, tbl))
	}
	if err != nil {
		if ownTx {
			_ = tx.Rollback()
		}
		return err
	}
	defer func() { _ = stmt.Close() }()

	for _, m := range defs {
		var labelKey, widget, placeholderKey string
		if m.Display != nil {
			labelKey = m.Display.LabelKey
			widget = m.Display.Widget
			placeholderKey = m.Display.PlaceholderKey
		}
		var defVal string
		if m.Default != nil {
			defVal = *m.Default
		}
		dbid := monitordb.NormalizeDBID(m.DBID)
		if _, err := stmt.ExecContext(ctx, dbid, m.TableName, m.ColumnName, m.DataType, labelKey, widget, placeholderKey, m.Nullable, m.Unique, m.HasDefault, defVal, m.Validator); err != nil {
			if ownTx {
				_ = tx.Rollback()
			}
			return err
		}
	}
	if ownTx {
		return tx.Commit()
	}
	return nil
}

// DeleteFieldDefs removes field definitions.
func (s *SQLMetaStore) DeleteFieldDefs(ctx context.Context, tx *sql.Tx, defs []metapkg.FieldDef) error {
	if len(defs) == 0 {
		return nil
	}
	ownTx := false
	if tx == nil {
		var err error
		tx, err = s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		ownTx = true
	}
	tbl := s.table("custom_fields")
	var (
		stmt *sql.Stmt
		err  error
	)
	switch s.driver {
	case "postgres":
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE db_id=$1 AND table_name=$2 AND column_name=$3`, tbl))
	case "mysql":
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE db_id=? AND table_name=? AND column_name=?`, tbl))
	default:
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE db_id=? AND table_name=? AND column_name=?`, tbl))
	}
	if err != nil {
		if ownTx {
			_ = tx.Rollback()
		}
		return err
	}
	defer func() { _ = stmt.Close() }()
	for _, m := range defs {
		dbid := monitordb.NormalizeDBID(m.DBID)
		if _, err := stmt.ExecContext(ctx, dbid, m.TableName, m.ColumnName); err != nil {
			if ownTx {
				_ = tx.Rollback()
			}
			return err
		}
	}
	if ownTx {
		return tx.Commit()
	}
	return nil
}

// ListFieldDefs returns field definitions for the given tenant.
func (s *SQLMetaStore) ListFieldDefs(ctx context.Context, tenantID string) ([]metapkg.FieldDef, error) {
	tbl := s.table("custom_fields")
	var (
		query string
	)
	switch s.driver {
	case "postgres":
		query = fmt.Sprintf(`SELECT db_id, table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, "unique", has_default, default_value, validator FROM %s WHERE tenant_id=$1 ORDER BY table_name, column_name`, tbl)
	case "mysql":
		query = fmt.Sprintf("SELECT db_id, table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, `unique`, has_default, default_value, validator FROM %s WHERE tenant_id=? ORDER BY table_name, column_name", tbl)
	default:
		query = fmt.Sprintf(`SELECT db_id, table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, "unique", has_default, default_value, validator FROM %s WHERE tenant_id=? ORDER BY table_name, column_name`, tbl)
	}
	rows, err := s.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var res []metapkg.FieldDef
	for rows.Next() {
		var m metapkg.FieldDef
		var labelKey, widget, placeholderKey sql.NullString
		var defVal, validator sql.NullString
		var hasDefault bool
		if err := rows.Scan(&m.DBID, &m.TableName, &m.ColumnName, &m.DataType, &labelKey, &widget, &placeholderKey, &m.Nullable, &m.Unique, &hasDefault, &defVal, &validator); err != nil {
			return nil, err
		}
		if labelKey.Valid || widget.Valid || placeholderKey.Valid {
			m.Display = &registry.DisplayMeta{LabelKey: labelKey.String, Widget: widget.String, PlaceholderKey: placeholderKey.String}
		}
		m.HasDefault = hasDefault
		if defVal.Valid {
			v := defVal.String
			m.Default = &v
		}
		if validator.Valid {
			m.Validator = validator.String
		}
		res = append(res, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

// RecordScanResult persists scan results.
func (s *SQLMetaStore) RecordScanResult(ctx context.Context, tx *sql.Tx, res metapkg.ScanResult) error {
	// TODO: implement using s.schema and s.driver
	return nil
}
