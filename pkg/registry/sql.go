package registry

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/faciam-dev/gcfm/pkg/monitordb"
	pkgutil "github.com/faciam-dev/gcfm/pkg/util"
	"github.com/faciam-dev/goquent/orm/query"
)

func LoadSQL(ctx context.Context, db *sql.DB, conf DBConfig) ([]FieldMeta, error) {
	dialect := pkgutil.DialectFromDriver(conf.Driver)
	tbl := TableName(conf.TablePrefix, "custom_fields")
	q := query.New(db, tbl, dialect).
		Select("db_id", "table_name", "column_name", "data_type", "label_key", "widget", "widget_config", "placeholder_key", "nullable", "unique", "has_default", "default_value", "validator").
		OrderByRaw("table_name, column_name").
		WithContext(ctx)

	type row struct {
		DBID         int64          `db:"db_id"`
		TableName    string         `db:"table_name"`
		ColumnName   string         `db:"column_name"`
		DataType     string         `db:"data_type"`
		LabelKey     sql.NullString `db:"label_key"`
		Widget       sql.NullString `db:"widget"`
		WidgetConfig sql.NullString `db:"widget_config"`
		Placeholder  sql.NullString `db:"placeholder_key"`
		Nullable     bool           `db:"nullable"`
		Unique       bool           `db:"unique"`
		HasDefault   bool           `db:"has_default"`
		DefaultValue sql.NullString `db:"default_value"`
		Validator    sql.NullString `db:"validator"`
	}

	var rows []row
	if err := q.Get(&rows); err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	metas := make([]FieldMeta, 0, len(rows))
	for _, r := range rows {
		m := FieldMeta{
			DBID:       r.DBID,
			TableName:  r.TableName,
			ColumnName: r.ColumnName,
			DataType:   r.DataType,
			Nullable:   r.Nullable,
			Unique:     r.Unique,
			HasDefault: r.HasDefault,
		}
		if r.LabelKey.Valid || r.Widget.Valid || r.Placeholder.Valid || r.WidgetConfig.Valid {
			dm := DisplayMeta{LabelKey: r.LabelKey.String, Widget: r.Widget.String, PlaceholderKey: r.Placeholder.String}
			if r.WidgetConfig.Valid {
				dm.WidgetConfig = json.RawMessage(r.WidgetConfig.String)
			}
			m.Display = &dm
		}
		if r.DefaultValue.Valid {
			v := r.DefaultValue.String
			m.Default = &v
		}
		if r.Validator.Valid {
			m.Validator = r.Validator.String
		}
		metas = append(metas, m)
	}
	return metas, nil
}

// LoadSQLByTenant is like LoadSQL but filters by tenant ID.
func LoadSQLByTenant(ctx context.Context, db *sql.DB, conf DBConfig, tenant string) ([]FieldMeta, error) {
	dialect := pkgutil.DialectFromDriver(conf.Driver)
	tbl := TableName(conf.TablePrefix, "custom_fields")
	q := query.New(db, tbl, dialect).
		Select("db_id", "table_name", "column_name", "data_type", "label_key", "widget", "widget_config", "placeholder_key", "nullable", "unique", "has_default", "default_value", "validator").
		Where("tenant_id", tenant).
		OrderByRaw("table_name, column_name").
		WithContext(ctx)

	type row struct {
		DBID         int64          `db:"db_id"`
		TableName    string         `db:"table_name"`
		ColumnName   string         `db:"column_name"`
		DataType     string         `db:"data_type"`
		LabelKey     sql.NullString `db:"label_key"`
		Widget       sql.NullString `db:"widget"`
		WidgetConfig sql.NullString `db:"widget_config"`
		Placeholder  sql.NullString `db:"placeholder_key"`
		Nullable     bool           `db:"nullable"`
		Unique       bool           `db:"unique"`
		HasDefault   bool           `db:"has_default"`
		DefaultValue sql.NullString `db:"default_value"`
		Validator    sql.NullString `db:"validator"`
	}

	var rows []row
	if err := q.Get(&rows); err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	metas := make([]FieldMeta, 0, len(rows))
	for _, r := range rows {
		m := FieldMeta{
			DBID:       r.DBID,
			TableName:  r.TableName,
			ColumnName: r.ColumnName,
			DataType:   r.DataType,
			Nullable:   r.Nullable,
			Unique:     r.Unique,
			HasDefault: r.HasDefault,
		}
		if r.LabelKey.Valid || r.Widget.Valid || r.Placeholder.Valid || r.WidgetConfig.Valid {
			dm := DisplayMeta{LabelKey: r.LabelKey.String, Widget: r.Widget.String, PlaceholderKey: r.Placeholder.String}
			if r.WidgetConfig.Valid {
				dm.WidgetConfig = json.RawMessage(r.WidgetConfig.String)
			}
			m.Display = &dm
		}
		if r.DefaultValue.Valid {
			v := r.DefaultValue.String
			m.Default = &v
		}
		if r.Validator.Valid {
			m.Validator = r.Validator.String
		}
		metas = append(metas, m)
	}
	return metas, nil
}

// LoadSQLByDB filters by tenant and database ID.
func LoadSQLByDB(ctx context.Context, db *sql.DB, conf DBConfig, tenant string, dbID int64) ([]FieldMeta, error) {
	dialect := pkgutil.DialectFromDriver(conf.Driver)
	tbl := TableName(conf.TablePrefix, "custom_fields")
	q := query.New(db, tbl, dialect).
		Select("db_id", "table_name", "column_name", "data_type", "label_key", "widget", "widget_config", "placeholder_key", "nullable", "unique", "has_default", "default_value", "validator").
		Where("tenant_id", tenant).
		Where("db_id", dbID).
		OrderByRaw("table_name, column_name").
		WithContext(ctx)

	type row struct {
		DBID         int64          `db:"db_id"`
		TableName    string         `db:"table_name"`
		ColumnName   string         `db:"column_name"`
		DataType     string         `db:"data_type"`
		LabelKey     sql.NullString `db:"label_key"`
		Widget       sql.NullString `db:"widget"`
		WidgetConfig sql.NullString `db:"widget_config"`
		Placeholder  sql.NullString `db:"placeholder_key"`
		Nullable     bool           `db:"nullable"`
		Unique       bool           `db:"unique"`
		HasDefault   bool           `db:"has_default"`
		DefaultValue sql.NullString `db:"default_value"`
		Validator    sql.NullString `db:"validator"`
	}

	var rows []row
	if err := q.Get(&rows); err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	metas := make([]FieldMeta, 0, len(rows))
	for _, r := range rows {
		m := FieldMeta{
			DBID:       r.DBID,
			TableName:  r.TableName,
			ColumnName: r.ColumnName,
			DataType:   r.DataType,
			Nullable:   r.Nullable,
			Unique:     r.Unique,
			HasDefault: r.HasDefault,
		}
		if r.LabelKey.Valid || r.Widget.Valid || r.Placeholder.Valid || r.WidgetConfig.Valid {
			dm := DisplayMeta{LabelKey: r.LabelKey.String, Widget: r.Widget.String, PlaceholderKey: r.Placeholder.String}
			if r.WidgetConfig.Valid {
				dm.WidgetConfig = json.RawMessage(r.WidgetConfig.String)
			}
			m.Display = &dm
		}
		if r.DefaultValue.Valid {
			v := r.DefaultValue.String
			m.Default = &v
		}
		if r.Validator.Valid {
			m.Validator = r.Validator.String
		}
		metas = append(metas, m)
	}
	return metas, nil
}

func UpsertSQL(ctx context.Context, db *sql.DB, driver, tablePrefix string, metas []FieldMeta) error {
	if len(metas) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	tbl := TableName(tablePrefix, "custom_fields")
	var stmt *sql.Stmt
	switch driver {
	case "postgres":
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf(`INSERT INTO %s (db_id, table_name, column_name, data_type, label_key, widget, widget_config, placeholder_key, nullable, "unique", has_default, default_value, validator, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13, NOW(), NOW()) ON CONFLICT (db_id, tenant_id, table_name, column_name) DO UPDATE SET data_type=EXCLUDED.data_type, label_key=EXCLUDED.label_key, widget=EXCLUDED.widget, widget_config=EXCLUDED.widget_config, placeholder_key=EXCLUDED.placeholder_key, nullable=EXCLUDED.nullable, "unique"=EXCLUDED."unique", has_default=EXCLUDED.has_default, default_value=EXCLUDED.default_value, validator=EXCLUDED.validator, updated_at=NOW()`, tbl))
	case "mysql":
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf("INSERT INTO %s (db_id, table_name, column_name, data_type, label_key, widget, widget_config, placeholder_key, nullable, `unique`, has_default, default_value, validator, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW()) ON DUPLICATE KEY UPDATE data_type=VALUES(data_type), label_key=VALUES(label_key), widget=VALUES(widget), widget_config=VALUES(widget_config), placeholder_key=VALUES(placeholder_key), nullable=VALUES(nullable), `unique`=VALUES(`unique`), has_default=VALUES(has_default), default_value=VALUES(default_value), validator=VALUES(validator), updated_at=NOW()", tbl))
	default:
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback: %v: unsupported driver: %s", rbErr, driver)
		}
		return fmt.Errorf("unsupported driver: %s", driver)
	}
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback: %v: prepare: %w", rbErr, err)
		}
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, m := range metas {
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
		var def string
		if m.Default != nil {
			def = *m.Default
		}
		dbid := monitordb.NormalizeDBID(m.DBID)
		if _, err := stmt.ExecContext(ctx, dbid, m.TableName, m.ColumnName, m.DataType, labelKey, widget, widgetCfg, placeholderKey, m.Nullable, m.Unique, m.HasDefault, def, m.Validator); err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				return fmt.Errorf("rollback: %v: exec: %w", rbErr, err)
			}
			return fmt.Errorf("exec: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// UpsertSQLByTenant inserts or updates fields for a specific tenant and returns inserted/updated counts.
func UpsertSQLByTenant(ctx context.Context, db *sql.DB, driver, tablePrefix, tenant string, metas []FieldMeta) (inserted, updated int, err error) {
	if len(metas) == 0 {
		return 0, 0, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin tx: %w", err)
	}
	tbl := TableName(tablePrefix, "custom_fields")
	var stmt *sql.Stmt
	switch driver {
	case "postgres":
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf(`INSERT INTO %s (db_id, tenant_id, table_name, column_name, data_type, label_key, widget, widget_config, placeholder_key, nullable, "unique", has_default, default_value, validator, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14, NOW(), NOW()) ON CONFLICT (db_id, tenant_id, table_name, column_name) DO UPDATE SET data_type=EXCLUDED.data_type, label_key=EXCLUDED.label_key, widget=EXCLUDED.widget, widget_config=EXCLUDED.widget_config, placeholder_key=EXCLUDED.placeholder_key, nullable=EXCLUDED.nullable, "unique"=EXCLUDED."unique", has_default=EXCLUDED.has_default, default_value=EXCLUDED.default_value, validator=EXCLUDED.validator, updated_at=NOW() RETURNING xmax = 0`, tbl))
	case "mysql":
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf("INSERT INTO %s (db_id, tenant_id, table_name, column_name, data_type, label_key, widget, widget_config, placeholder_key, nullable, `unique`, has_default, default_value, validator, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW()) ON DUPLICATE KEY UPDATE data_type=VALUES(data_type), label_key=VALUES(label_key), widget=VALUES(widget), widget_config=VALUES(widget_config), placeholder_key=VALUES(placeholder_key), nullable=VALUES(nullable), `unique`=VALUES(`unique`), has_default=VALUES(has_default), default_value=VALUES(default_value), validator=VALUES(validator), updated_at=NOW()", tbl))
	default:
		if rbErr := tx.Rollback(); rbErr != nil {
			return 0, 0, fmt.Errorf("rollback: %v: unsupported driver: %s", rbErr, driver)
		}
		return 0, 0, fmt.Errorf("unsupported driver: %s", driver)
	}
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return 0, 0, fmt.Errorf("rollback: %v: prepare: %w", rbErr, err)
		}
		return 0, 0, fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, m := range metas {
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
		var def string
		if m.Default != nil {
			def = *m.Default
		}
		dbid := monitordb.NormalizeDBID(m.DBID)
		switch driver {
		case "postgres":
			var isInsert bool
			if err := stmt.QueryRowContext(ctx, dbid, tenant, m.TableName, m.ColumnName, m.DataType, labelKey, widget, widgetCfg, placeholderKey, m.Nullable, m.Unique, m.HasDefault, def, m.Validator).Scan(&isInsert); err != nil {
				if rbErr := tx.Rollback(); rbErr != nil {
					return 0, 0, fmt.Errorf("rollback: %v: exec: %w", rbErr, err)
				}
				return 0, 0, fmt.Errorf("exec: %w", err)
			}
			if isInsert {
				inserted++
			} else {
				updated++
			}
		case "mysql":
			res, err := stmt.ExecContext(ctx, dbid, tenant, m.TableName, m.ColumnName, m.DataType, labelKey, widget, widgetCfg, placeholderKey, m.Nullable, m.Unique, m.HasDefault, def, m.Validator)
			if err != nil {
				if rbErr := tx.Rollback(); rbErr != nil {
					return 0, 0, fmt.Errorf("rollback: %v: exec: %w", rbErr, err)
				}
				return 0, 0, fmt.Errorf("exec: %w", err)
			}
			ra, _ := res.RowsAffected()
			if ra == 1 {
				inserted++
			} else {
				updated++
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit: %w", err)
	}
	return inserted, updated, nil
}

// UpsertFieldDefByDB upserts a single field definition for the specified tenant and DB.
func UpsertFieldDefByDB(ctx context.Context, db *sql.DB, driver, tablePrefix, tenant string, meta FieldMeta) error {
	_, _, err := UpsertSQLByTenant(ctx, db, driver, tablePrefix, tenant, []FieldMeta{meta})
	return err
}

// DeleteFieldDefByDB removes a single field definition.
func DeleteFieldDefByDB(ctx context.Context, db *sql.DB, driver, tablePrefix string, meta FieldMeta) error {
	return DeleteSQL(ctx, db, driver, tablePrefix, []FieldMeta{meta})
}

func DeleteSQL(ctx context.Context, db *sql.DB, driver, tablePrefix string, metas []FieldMeta) error {
	if len(metas) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	tbl := TableName(tablePrefix, "custom_fields")
	var stmt *sql.Stmt
	switch driver {
	case "postgres":
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE db_id = $1 AND table_name = $2 AND column_name = $3`, tbl))
	case "mysql":
		stmt, err = tx.PrepareContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE db_id = ? AND table_name = ? AND column_name = ?`, tbl))
	default:
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback: %v: unsupported driver: %s", rbErr, driver)
		}
		return fmt.Errorf("unsupported driver: %s", driver)
	}
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback: %v: prepare: %w", rbErr, err)
		}
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()
	for _, m := range metas {
		dbid := monitordb.NormalizeDBID(m.DBID)
		if _, err := stmt.ExecContext(ctx, dbid, m.TableName, m.ColumnName); err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				return fmt.Errorf("rollback: %v: exec: %w", rbErr, err)
			}
			return fmt.Errorf("exec: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
