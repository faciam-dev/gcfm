package sqlmetastore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	metapkg "github.com/faciam-dev/gcfm/meta"
	"github.com/faciam-dev/gcfm/pkg/monitordb"
	"github.com/faciam-dev/gcfm/pkg/registry"
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
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf(`INSERT INTO %s (db_id, table_name, column_name, data_type, label_key, widget, widget_config, placeholder_key, nullable, "unique", has_default, default_value, validator, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NOW(),NOW()) ON CONFLICT (db_id, tenant_id, table_name, column_name) DO UPDATE SET data_type=EXCLUDED.data_type, label_key=EXCLUDED.label_key, widget=EXCLUDED.widget, widget_config=EXCLUDED.widget_config, placeholder_key=EXCLUDED.placeholder_key, nullable=EXCLUDED.nullable, "unique"=EXCLUDED."unique", has_default=EXCLUDED.has_default, default_value=EXCLUDED.default_value, validator=EXCLUDED.validator, updated_at=NOW()`, tbl))
	case "mysql":
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf("INSERT INTO %s (db_id, table_name, column_name, data_type, label_key, widget, widget_config, placeholder_key, nullable, `unique`, has_default, default_value, validator, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW()) ON DUPLICATE KEY UPDATE data_type=VALUES(data_type), label_key=VALUES(label_key), widget=VALUES(widget), widget_config=VALUES(widget_config), placeholder_key=VALUES(placeholder_key), nullable=VALUES(nullable), `unique`=VALUES(`unique`), has_default=VALUES(has_default), default_value=VALUES(default_value), validator=VALUES(validator), updated_at=NOW()", tbl))
	default:
		// Assume drivers using '?' placeholders and supporting ON CONFLICT.
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf(`INSERT INTO %s (db_id, table_name, column_name, data_type, label_key, widget, widget_config, placeholder_key, nullable, "unique", has_default, default_value, validator, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) ON CONFLICT (db_id, tenant_id, table_name, column_name) DO UPDATE SET data_type=excluded.data_type, label_key=excluded.label_key, widget=excluded.widget, widget_config=excluded.widget_config, placeholder_key=excluded.placeholder_key, nullable=excluded.nullable, "unique"=excluded."unique", has_default=excluded.has_default, default_value=excluded.default_value, validator=excluded.validator, updated_at=CURRENT_TIMESTAMP`, tbl))
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
		var widgetCfg any
		if m.Display != nil {
			labelKey = m.Display.LabelKey
			widget = m.Display.Widget
			placeholderKey = m.Display.PlaceholderKey
			if len(m.Display.WidgetConfig) > 0 {
				widgetCfg = string(m.Display.WidgetConfig)
			}
		}
		var defVal string
		if m.Default != nil {
			defVal = *m.Default
		}
		dbid := monitordb.NormalizeDBID(m.DBID)
		if _, err := stmt.ExecContext(ctx, dbid, m.TableName, m.ColumnName, m.DataType, labelKey, widget, widgetCfg, placeholderKey, m.Nullable, m.Unique, m.HasDefault, defVal, m.Validator); err != nil {
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
	tbl := s.table("scan_results")
	var (
		query string
		args  []interface{}
	)
	switch s.driver {
	case "postgres":
		query = fmt.Sprintf(`INSERT INTO %s (tenant_id, scan_id, status, started_at, finished_at, details) VALUES ($1, $2, $3, $4, $5, $6)`, tbl)
		args = []interface{}{res.TenantID, res.ScanID, res.Status, res.StartedAt, res.FinishedAt, res.Details}
	case "mysql":
		query = fmt.Sprintf("INSERT INTO %s (tenant_id, scan_id, status, started_at, finished_at, details) VALUES (?, ?, ?, ?, ?, ?)", tbl)
		args = []interface{}{res.TenantID, res.ScanID, res.Status, res.StartedAt, res.FinishedAt, res.Details}
	default:
		query = fmt.Sprintf(`INSERT INTO %s (tenant_id, scan_id, status, started_at, finished_at, details) VALUES (?, ?, ?, ?, ?, ?)`, tbl)
		args = []interface{}{res.TenantID, res.ScanID, res.Status, res.StartedAt, res.FinishedAt, res.Details}
	}
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, args...)
	} else {
		_, err = s.db.ExecContext(ctx, query, args...)
	}
	return err
}

// UpsertTarget inserts or updates a target definition and its labels.
func (s *SQLMetaStore) UpsertTarget(ctx context.Context, tx *sql.Tx, t metapkg.TargetRow, labels []string) error {
	ownTx := false
	if tx == nil {
		var err error
		tx, err = s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		ownTx = true
	}

	tbl := s.table("targets")
	var stmt *sql.Stmt
	var err error
	switch s.driver {
	case "postgres":
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf(`INSERT INTO %s (key, driver, dsn, schema_name, max_open_conns, max_idle_conns, conn_max_idle_ms, conn_max_life_ms, is_default, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW()) ON CONFLICT (key) DO UPDATE SET driver=EXCLUDED.driver, dsn=EXCLUDED.dsn, schema_name=EXCLUDED.schema_name, max_open_conns=EXCLUDED.max_open_conns, max_idle_conns=EXCLUDED.max_idle_conns, conn_max_idle_ms=EXCLUDED.conn_max_idle_ms, conn_max_life_ms=EXCLUDED.conn_max_life_ms, is_default=EXCLUDED.is_default, updated_at=NOW()`, tbl))
	case "mysql":
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf(`INSERT INTO %s (key, driver, dsn, schema_name, max_open_conns, max_idle_conns, conn_max_idle_ms, conn_max_life_ms, is_default, updated_at) VALUES (?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP) ON DUPLICATE KEY UPDATE driver=VALUES(driver), dsn=VALUES(dsn), schema_name=VALUES(schema_name), max_open_conns=VALUES(max_open_conns), max_idle_conns=VALUES(max_idle_conns), conn_max_idle_ms=VALUES(conn_max_idle_ms), conn_max_life_ms=VALUES(conn_max_life_ms), is_default=VALUES(is_default), updated_at=CURRENT_TIMESTAMP`, tbl))
	default:
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf(`INSERT INTO %s (key, driver, dsn, schema_name, max_open_conns, max_idle_conns, conn_max_idle_ms, conn_max_life_ms, is_default, updated_at) VALUES (?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP) ON CONFLICT (key) DO UPDATE SET driver=excluded.driver, dsn=excluded.dsn, schema_name=excluded.schema_name, max_open_conns=excluded.max_open_conns, max_idle_conns=excluded.max_idle_conns, conn_max_idle_ms=excluded.conn_max_idle_ms, conn_max_life_ms=excluded.conn_max_life_ms, is_default=excluded.is_default, updated_at=CURRENT_TIMESTAMP`, tbl))
	}
	if err != nil {
		if ownTx {
			_ = tx.Rollback()
		}
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, t.Key, t.Driver, t.DSN, t.Schema, t.MaxOpenConns, t.MaxIdleConns, t.ConnMaxIdle.Milliseconds(), t.ConnMaxLife.Milliseconds(), t.IsDefault)
	if err != nil {
		if ownTx {
			_ = tx.Rollback()
		}
		return err
	}

	lblTbl := s.table("target_labels")
	var delQ, insQ string
	switch s.driver {
	case "postgres":
		delQ = fmt.Sprintf("DELETE FROM %s WHERE key=$1", lblTbl)
		insQ = fmt.Sprintf("INSERT INTO %s (key,label) VALUES ($1,$2)", lblTbl)
	case "mysql":
		delQ = fmt.Sprintf("DELETE FROM %s WHERE key=?", lblTbl)
		insQ = fmt.Sprintf("INSERT INTO %s (key,label) VALUES (?,?)", lblTbl)
	default:
		delQ = fmt.Sprintf("DELETE FROM %s WHERE key=?", lblTbl)
		insQ = fmt.Sprintf("INSERT INTO %s (key,label) VALUES (?,?)", lblTbl)
	}
	if _, err := tx.ExecContext(ctx, delQ, t.Key); err != nil {
		if ownTx {
			_ = tx.Rollback()
		}
		return err
	}
	if len(labels) > 0 {
		lstmt, err := tx.PrepareContext(ctx, insQ)
		if err != nil {
			if ownTx {
				_ = tx.Rollback()
			}
			return err
		}
		for _, lb := range labels {
			if _, err := lstmt.ExecContext(ctx, t.Key, lb); err != nil {
				lstmt.Close()
				if ownTx {
					_ = tx.Rollback()
				}
				return err
			}
		}
		lstmt.Close()
	}

	if ownTx {
		return tx.Commit()
	}
	return nil
}

// DeleteTarget removes a target definition.
func (s *SQLMetaStore) DeleteTarget(ctx context.Context, tx *sql.Tx, key string) error {
	ownTx := false
	if tx == nil {
		var err error
		tx, err = s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		ownTx = true
	}
	tbl := s.table("targets")
	var q string
	switch s.driver {
	case "postgres":
		q = fmt.Sprintf("DELETE FROM %s WHERE key=$1", tbl)
	case "mysql":
		q = fmt.Sprintf("DELETE FROM %s WHERE key=?", tbl)
	default:
		q = fmt.Sprintf("DELETE FROM %s WHERE key=?", tbl)
	}
	if _, err := tx.ExecContext(ctx, q, key); err != nil {
		if ownTx {
			_ = tx.Rollback()
		}
		return err
	}
	if ownTx {
		return tx.Commit()
	}
	return nil
}

// ListTargets returns all targets with labels, along with version and default key.
func (s *SQLMetaStore) ListTargets(ctx context.Context) ([]metapkg.TargetRowWithLabels, string, string, error) {
	tbl := s.table("targets")
	lblTbl := s.table("target_labels")
	verTbl := s.table("target_config_version")

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT t.key, t.driver, t.dsn, t.schema_name, t.max_open_conns, t.max_idle_conns, t.conn_max_idle_ms, t.conn_max_life_ms, t.is_default, l.label FROM %s t LEFT JOIN %s l ON t.key=l.key ORDER BY t.key`, tbl, lblTbl))
	if err != nil {
		return nil, "", "", err
	}
	defer rows.Close()

	m := make(map[string]*metapkg.TargetRowWithLabels)
	order := make([]string, 0)
	for rows.Next() {
		var r metapkg.TargetRowWithLabels
		var label sql.NullString
		if err := rows.Scan(&r.Key, &r.Driver, &r.DSN, &r.Schema, &r.MaxOpenConns, &r.MaxIdleConns, &r.ConnMaxIdle, &r.ConnMaxLife, &r.IsDefault, &label); err != nil {
			return nil, "", "", err
		}
		r.ConnMaxIdle = time.Duration(r.ConnMaxIdle) * time.Millisecond
		r.ConnMaxLife = time.Duration(r.ConnMaxLife) * time.Millisecond
		if existing, ok := m[r.Key]; ok {
			if label.Valid {
				existing.Labels = append(existing.Labels, label.String)
			}
		} else {
			if label.Valid {
				r.Labels = []string{label.String}
			}
			m[r.Key] = &r
			order = append(order, r.Key)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", "", err
	}
	var res []metapkg.TargetRowWithLabels
	for _, k := range order {
		res = append(res, *m[k])
	}

	// fetch version
	var ver string
	if err := s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT version FROM %s WHERE id=1", verTbl)).Scan(&ver); err != nil {
		return nil, "", "", err
	}
	// fetch default key
	var def sql.NullString
	if err := s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT key FROM %s WHERE is_default=true LIMIT 1", tbl)).Scan(&def); err != nil && err != sql.ErrNoRows {
		return nil, "", "", err
	}
	return res, ver, def.String, nil
}

// SetDefaultTarget marks the given key as default.
func (s *SQLMetaStore) SetDefaultTarget(ctx context.Context, tx *sql.Tx, key string) error {
	ownTx := false
	if tx == nil {
		var err error
		tx, err = s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		ownTx = true
	}
	tbl := s.table("targets")
	// clear existing default
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET is_default=false WHERE is_default=true", tbl)); err != nil {
		if ownTx {
			_ = tx.Rollback()
		}
		return err
	}
	// set new default
	var q string
	switch s.driver {
	case "postgres":
		q = fmt.Sprintf("UPDATE %s SET is_default=true WHERE key=$1", tbl)
	case "mysql":
		q = fmt.Sprintf("UPDATE %s SET is_default=true WHERE key=?", tbl)
	default:
		q = fmt.Sprintf("UPDATE %s SET is_default=true WHERE key=?", tbl)
	}
	if _, err := tx.ExecContext(ctx, q, key); err != nil {
		if ownTx {
			_ = tx.Rollback()
		}
		return err
	}
	if ownTx {
		return tx.Commit()
	}
	return nil
}

// BumpTargetsVersion updates and returns a new configuration version.
func (s *SQLMetaStore) BumpTargetsVersion(ctx context.Context, tx *sql.Tx) (string, error) {
	ownTx := false
	if tx == nil {
		var err error
		tx, err = s.db.BeginTx(ctx, nil)
		if err != nil {
			return "", err
		}
		ownTx = true
	}
	tbl := s.table("target_config_version")
	ver := strings.ReplaceAll(uuid.NewString(), "-", "")
	var q string
	switch s.driver {
	case "postgres":
		q = fmt.Sprintf("UPDATE %s SET version=$1, updated_at=NOW() WHERE id=1", tbl)
	case "mysql":
		q = fmt.Sprintf("UPDATE %s SET version=?, updated_at=CURRENT_TIMESTAMP WHERE id=1", tbl)
	default:
		q = fmt.Sprintf("UPDATE %s SET version=?, updated_at=CURRENT_TIMESTAMP WHERE id=1", tbl)
	}
	if _, err := tx.ExecContext(ctx, q, ver); err != nil {
		if ownTx {
			_ = tx.Rollback()
		}
		return "", err
	}
	if ownTx {
		if err := tx.Commit(); err != nil {
			return "", err
		}
	}
	return ver, nil
}
