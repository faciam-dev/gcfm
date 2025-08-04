package registry

import (
	"context"
	"database/sql"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DBConfig struct {
	DSN         string
	Schema      string
	Driver      string
	TablePrefix string
}

type FieldMeta struct {
	TableName   string       `yaml:"table"`
	ColumnName  string       `yaml:"column"`
	DataType    string       `yaml:"type"`
	Placeholder string       `yaml:"placeholder,omitempty"` // v0.1 compatibility
	Display     *DisplayMeta `yaml:"display,omitempty"`
	Validator   string       `yaml:"validator,omitempty"`
	Nullable    bool         `yaml:"nullable,omitempty"`
	Unique      bool         `yaml:"unique,omitempty"`
	HasDefault  bool         `yaml:"hasDefault,omitempty" json:"hasDefault"`
	Default     *string      `yaml:"defaultValue,omitempty" json:"defaultValue,omitempty"`
}

type Scanner interface {
	Scan(ctx context.Context, conf DBConfig) ([]FieldMeta, error)
}

func LoadSQL(ctx context.Context, db *sql.DB, conf DBConfig) ([]FieldMeta, error) {
	var query string
	switch conf.Driver {
	case "postgres":
		query = `SELECT table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, "unique", has_default, default_value, validator FROM gcfm_custom_fields ORDER BY table_name, column_name`
	default:
		query = "SELECT table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, `unique`, has_default, default_value, validator FROM gcfm_custom_fields ORDER BY table_name, column_name"
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var metas []FieldMeta
	for rows.Next() {
		var m FieldMeta
		var labelKey, widget, placeholderKey sql.NullString
		var def, validator sql.NullString
		var hasDefault bool
		if err := rows.Scan(&m.TableName, &m.ColumnName, &m.DataType, &labelKey, &widget, &placeholderKey, &m.Nullable, &m.Unique, &hasDefault, &def, &validator); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if labelKey.Valid || widget.Valid || placeholderKey.Valid {
			m.Display = &DisplayMeta{LabelKey: labelKey.String, Widget: widget.String, PlaceholderKey: placeholderKey.String}
		}
		m.HasDefault = hasDefault
		if def.Valid {
			val := def.String
			m.Default = &val
		} else {
			m.Default = nil
		}
		if validator.Valid {
			m.Validator = validator.String
		}
		metas = append(metas, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return metas, nil
}

// LoadSQLByTenant is like LoadSQL but filters by tenant ID.
func LoadSQLByTenant(ctx context.Context, db *sql.DB, conf DBConfig, tenant string) ([]FieldMeta, error) {
	var query string
	switch conf.Driver {
	case "postgres":
		query = `SELECT table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, "unique", has_default, default_value, validator FROM gcfm_custom_fields WHERE tenant_id=$1 ORDER BY table_name, column_name`
	default:
		query = "SELECT table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, `unique`, has_default, default_value, validator FROM gcfm_custom_fields WHERE tenant_id=? ORDER BY table_name, column_name"
	}
	rows, err := db.QueryContext(ctx, query, tenant)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var metas []FieldMeta
	for rows.Next() {
		var m FieldMeta
		var labelKey, widget, placeholderKey sql.NullString
		var def, validator sql.NullString
		var hasDefault bool
		if err := rows.Scan(&m.TableName, &m.ColumnName, &m.DataType, &labelKey, &widget, &placeholderKey, &m.Nullable, &m.Unique, &hasDefault, &def, &validator); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if labelKey.Valid || widget.Valid || placeholderKey.Valid {
			m.Display = &DisplayMeta{LabelKey: labelKey.String, Widget: widget.String, PlaceholderKey: placeholderKey.String}
		}
		m.HasDefault = hasDefault
		if def.Valid {
			val := def.String
			m.Default = &val
		} else {
			m.Default = nil
		}
		if validator.Valid {
			m.Validator = validator.String
		}
		metas = append(metas, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return metas, nil
}

func UpsertSQL(ctx context.Context, db *sql.DB, driver string, metas []FieldMeta) error {
	if len(metas) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	var stmt *sql.Stmt
	switch driver {
	case "postgres":
		stmt, err = tx.PrepareContext(ctx, `INSERT INTO gcfm_custom_fields (table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, "unique", has_default, default_value, validator, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11, NOW(), NOW()) ON CONFLICT (table_name, column_name) DO UPDATE SET data_type=EXCLUDED.data_type, label_key=EXCLUDED.label_key, widget=EXCLUDED.widget, placeholder_key=EXCLUDED.placeholder_key, nullable=EXCLUDED.nullable, "unique"=EXCLUDED."unique", has_default=EXCLUDED.has_default, default_value=EXCLUDED.default_value, validator=EXCLUDED.validator, updated_at=NOW()`)
	case "mysql":
		stmt, err = tx.PrepareContext(ctx, "INSERT INTO gcfm_custom_fields (table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, `unique`, has_default, default_value, validator, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW()) ON DUPLICATE KEY UPDATE data_type=VALUES(data_type), label_key=VALUES(label_key), widget=VALUES(widget), placeholder_key=VALUES(placeholder_key), nullable=VALUES(nullable), `unique`=VALUES(`unique`), has_default=VALUES(has_default), default_value=VALUES(default_value), validator=VALUES(validator), updated_at=NOW()")
	default:
		tx.Rollback()
		return fmt.Errorf("unsupported driver: %s", driver)
	}
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, m := range metas {
		var labelKey, widget, placeholderKey string
		if m.Display != nil {
			labelKey = m.Display.LabelKey
			widget = m.Display.Widget
			placeholderKey = m.Display.PlaceholderKey
		}
		var def string
		if m.Default != nil {
			def = *m.Default
		}
		if _, err := stmt.ExecContext(ctx, m.TableName, m.ColumnName, m.DataType, labelKey, widget, placeholderKey, m.Nullable, m.Unique, m.HasDefault, def, m.Validator); err != nil {
			tx.Rollback()
			return fmt.Errorf("exec: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// UpsertSQLByTenant inserts or updates fields for a specific tenant.
func UpsertSQLByTenant(ctx context.Context, db *sql.DB, driver, tenant string, metas []FieldMeta) error {
	if len(metas) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	var stmt *sql.Stmt
	switch driver {
	case "postgres":
		stmt, err = tx.PrepareContext(ctx, `INSERT INTO gcfm_custom_fields (tenant_id, table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, "unique", has_default, default_value, validator, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12, NOW(), NOW()) ON CONFLICT (tenant_id, table_name, column_name) DO UPDATE SET data_type=EXCLUDED.data_type, label_key=EXCLUDED.label_key, widget=EXCLUDED.widget, placeholder_key=EXCLUDED.placeholder_key, nullable=EXCLUDED.nullable, "unique"=EXCLUDED."unique", has_default=EXCLUDED.has_default, default_value=EXCLUDED.default_value, validator=EXCLUDED.validator, updated_at=NOW()`)
	case "mysql":
		stmt, err = tx.PrepareContext(ctx, "INSERT INTO gcfm_custom_fields (tenant_id, table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, `unique`, has_default, default_value, validator, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW()) ON DUPLICATE KEY UPDATE data_type=VALUES(data_type), label_key=VALUES(label_key), widget=VALUES(widget), placeholder_key=VALUES(placeholder_key), nullable=VALUES(nullable), `unique`=VALUES(`unique`), has_default=VALUES(has_default), default_value=VALUES(default_value), validator=VALUES(validator), updated_at=NOW()")
	default:
		tx.Rollback()
		return fmt.Errorf("unsupported driver: %s", driver)
	}
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, m := range metas {
		var labelKey, widget, placeholderKey string
		if m.Display != nil {
			labelKey = m.Display.LabelKey
			widget = m.Display.Widget
			placeholderKey = m.Display.PlaceholderKey
		}
		var def string
		if m.Default != nil {
			def = *m.Default
		}
		if _, err := stmt.ExecContext(ctx, tenant, m.TableName, m.ColumnName, m.DataType, labelKey, widget, placeholderKey, m.Nullable, m.Unique, m.HasDefault, def, m.Validator); err != nil {
			tx.Rollback()
			return fmt.Errorf("exec: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func DeleteSQL(ctx context.Context, db *sql.DB, driver string, metas []FieldMeta) error {
	if len(metas) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	var stmt *sql.Stmt
	switch driver {
	case "postgres":
		stmt, err = tx.PrepareContext(ctx, `DELETE FROM gcfm_custom_fields WHERE table_name = $1 AND column_name = $2`)
	case "mysql":
		stmt, err = tx.PrepareContext(ctx, `DELETE FROM gcfm_custom_fields WHERE table_name = ? AND column_name = ?`)
	default:
		tx.Rollback()
		return fmt.Errorf("unsupported driver: %s", driver)
	}
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()
	for _, m := range metas {
		if _, err := stmt.ExecContext(ctx, m.TableName, m.ColumnName); err != nil {
			tx.Rollback()
			return fmt.Errorf("exec: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func LoadMongo(ctx context.Context, cli *mongo.Client, conf DBConfig) ([]FieldMeta, error) {
	coll := cli.Database(conf.Schema).Collection("custom_fields")
	cur, err := coll.Find(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var metas []FieldMeta
	for cur.Next(ctx) {
		var m FieldMeta
		if err := cur.Decode(&m); err != nil {
			return nil, err
		}
		metas = append(metas, m)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return metas, nil
}

func UpsertMongo(ctx context.Context, cli *mongo.Client, conf DBConfig, metas []FieldMeta) error {
	if len(metas) == 0 {
		return nil
	}
	coll := cli.Database(conf.Schema).Collection("custom_fields")
	for _, m := range metas {
		filter := bson.M{"table_name": m.TableName, "column_name": m.ColumnName}
		update := bson.M{"$set": bson.M{"data_type": m.DataType, "table_name": m.TableName, "column_name": m.ColumnName}}
		if _, err := coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true)); err != nil {
			return err
		}
	}
	return nil
}

func DeleteMongo(ctx context.Context, cli *mongo.Client, conf DBConfig, metas []FieldMeta) error {
	if len(metas) == 0 {
		return nil
	}
	coll := cli.Database(conf.Schema).Collection("custom_fields")
	for _, m := range metas {
		if _, err := coll.DeleteOne(ctx, bson.M{"table_name": m.TableName, "column_name": m.ColumnName}); err != nil {
			return err
		}
	}
	return nil
}
