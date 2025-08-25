package registry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

func normalizeType(driver, typ string) string {
	switch driver {
	case "mysql":
		lower := strings.ToLower(strings.TrimSpace(typ))
		if lower == "varchar" {
			return "VARCHAR(255)"
		}
	}
	return typ
}

func escapeLiteral(v string) string {
	return strings.ReplaceAll(v, "'", "''")
}

func quoteIdentifier(driver, ident string) string {
	switch driver {
	case "postgres":
		return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
	case "mysql":
		return "`" + strings.ReplaceAll(ident, "`", "``") + "`"
	default:
		return ident
	}
}

var ErrDefaultNotSupported = errors.New("default not supported for column type")

type UnifiedDefault struct {
	Mode     string
	Raw      string
	OnUpdate bool
}

func isDateTimeLike(typ string) bool {
	t := strings.ToLower(strings.TrimSpace(typ))
	return strings.Contains(t, "timestamp") || strings.Contains(t, "datetime") || strings.Contains(t, "date") || strings.Contains(t, "time")
}

func isMySQLDefaultForbidden(typ string) bool {
	t := strings.ToLower(strings.TrimSpace(typ))
	return strings.Contains(t, "text") || strings.Contains(t, "blob") || strings.Contains(t, "geometry") || strings.Contains(t, "json")
}

func normalizeMySQLLiteral(typ, raw string) (string, error) {
	s := strings.TrimSpace(raw)
	return fmt.Sprintf("'%s'", escapeLiteral(s)), nil
}

func normalizePGLiteral(typ, raw string) (string, error) {
	s := strings.TrimSpace(raw)
	lit := fmt.Sprintf("'%s'", escapeLiteral(s))
	if isDateTimeLike(typ) {
		lit += "::timestamp"
	}
	return lit, nil
}

func isAllowedMySQLExpr(expr, colType string) bool {
	if !isDateTimeLike(colType) && expr != "CURRENT_DATE" && expr != "CURRENT_TIME" {
		return false
	}
	if expr == "CURRENT_TIMESTAMP" || strings.HasPrefix(expr, "CURRENT_TIMESTAMP(") || expr == "NOW()" || expr == "UTC_TIMESTAMP()" || expr == "CURRENT_DATE" || expr == "CURRENT_TIME" {
		return true
	}
	return false
}

func isAllowedPGExpr(expr, colType string) bool {
	up := strings.ToUpper(strings.TrimSpace(expr))
	switch up {
	case "CURRENT_TIMESTAMP", "NOW()", "CURRENT_DATE", "CURRENT_TIME":
		return true
	}
	return false
}

func BuildDefaultClauses(driver, colType string, d UnifiedDefault) (string, string, *string, bool, error) {
	if d.Mode == "none" || (strings.TrimSpace(d.Raw) == "" && d.Mode != "none") {
		return "", "", nil, false, nil
	}
	switch driver {
	case "mysql":
		if isMySQLDefaultForbidden(colType) {
			return "", "", nil, false, ErrDefaultNotSupported
		}
		if d.Mode == "expression" {
			up := strings.ToUpper(strings.TrimSpace(d.Raw))
			if !isAllowedMySQLExpr(up, colType) {
				return "", "", nil, false, fmt.Errorf("unsupported expression for %s: %s", colType, up)
			}
			onup := ""
			if d.OnUpdate && isDateTimeLike(colType) {
				onup = " ON UPDATE CURRENT_TIMESTAMP"
			}
			norm := up
			return " DEFAULT " + up, onup, &norm, true, nil
		}
		lit, err := normalizeMySQLLiteral(colType, d.Raw)
		if err != nil {
			return "", "", nil, false, err
		}
		norm := lit
		return " DEFAULT " + lit, "", &norm, true, nil
	case "postgres":
		if d.Mode == "expression" {
			up := strings.TrimSpace(d.Raw)
			if !isAllowedPGExpr(up, colType) {
				return "", "", nil, false, fmt.Errorf("unsupported expression for %s: %s", colType, up)
			}
			norm := up
			return " DEFAULT " + up, "", &norm, true, nil
		}
		lit, err := normalizePGLiteral(colType, d.Raw)
		if err != nil {
			return "", "", nil, false, err
		}
		norm := lit
		return " DEFAULT " + lit, "", &norm, true, nil
	case "mongo", "mongodb":
		if d.Mode == "expression" {
			return "", "", nil, false, fmt.Errorf("expression default is not supported for MongoDB")
		}
		norm := strings.TrimSpace(d.Raw)
		return "", "", &norm, true, nil
	default:
		return "", "", nil, false, fmt.Errorf("unknown driver")
	}
}

func ColumnExists(ctx context.Context, db *sql.DB, dialect ormdriver.Dialect, schema, table, column string) (bool, error) {
	q := query.New(db, "information_schema.columns", dialect).
		Where("table_name", table).
		Where("column_name", column)

	if schema != "" {
		q = q.Where("table_schema", schema)
	} else {
		switch dialect.(type) {
		case ormdriver.MySQLDialect, *ormdriver.MySQLDialect:
			q = q.WhereRaw("table_schema = DATABASE()", nil)
		case ormdriver.PostgresDialect, *ormdriver.PostgresDialect:
			q = q.Where("table_schema", "public")
		default:
			// For other dialects, omit the schema filter
		}
	}

	cnt, err := q.WithContext(ctx).Count("*")
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func AddColumnSQL(ctx context.Context, db *sql.DB, driver, table, column, typ string, nullable, unique *bool, d UnifiedDefault) error {
	typ = normalizeType(driver, typ)
	defClause, onUpdateClause, _, _, err := BuildDefaultClauses(driver, typ, d)
	if err != nil {
		return err
	}
	opts := []string{typ}
	if nullable != nil && !*nullable {
		opts = append(opts, "NOT NULL")
	}
	if defClause != "" {
		opts = append(opts, defClause)
	}
	if onUpdateClause != "" {
		opts = append(opts, onUpdateClause)
	}
	var stmt string
	switch driver {
	case "postgres":
		if unique != nil && *unique {
			opts = append(opts, fmt.Sprintf(`CONSTRAINT "%s_%s_key" UNIQUE`, table, column))
		}
		stmt = fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN "%s" %s`, table, column, strings.Join(opts, " "))
	case "mysql":
		stmt = fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` %s", table, column, strings.Join(opts, " "))
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("add column: %w", err)
		}
		if unique != nil && *unique {
			name := fmt.Sprintf("%s_%s_key", table, column)
			uq := fmt.Sprintf("ALTER TABLE `%s` ADD CONSTRAINT `%s` UNIQUE (`%s`)", table, name, column)
			if _, err := db.ExecContext(ctx, uq); err != nil {
				return fmt.Errorf("add unique: %w", err)
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported driver: %s", driver)
	}
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("add column: %w", err)
	}
	return nil
}

func ModifyColumnSQL(ctx context.Context, db *sql.DB, driver, table, column, typ string, nullable, unique *bool, d UnifiedDefault) error {
	typ = normalizeType(driver, typ)
	defClause, onUpdateClause, _, _, err := BuildDefaultClauses(driver, typ, d)
	if err != nil {
		return err
	}
	var stmt string
	switch driver {
	case "postgres":
		clauses := []string{fmt.Sprintf(`ALTER COLUMN "%s" TYPE %s`, column, typ)}
		if nullable != nil {
			if *nullable {
				clauses = append(clauses, fmt.Sprintf(`ALTER COLUMN "%s" DROP NOT NULL`, column))
			} else {
				clauses = append(clauses, fmt.Sprintf(`ALTER COLUMN "%s" SET NOT NULL`, column))
			}
		}
		if defClause != "" {
			expr := strings.TrimPrefix(defClause, " DEFAULT ")
			clauses = append(clauses, fmt.Sprintf(`ALTER COLUMN "%s" SET DEFAULT %s`, column, expr))
		}
		if unique != nil {
			if *unique {
				clauses = append(clauses, fmt.Sprintf(`ADD UNIQUE ("%s")`, column))
			} else {
				uqName := fmt.Sprintf("%s_%s_key", table, column)
				clauses = append(clauses, fmt.Sprintf(`DROP CONSTRAINT IF EXISTS "%s"`, uqName))
			}
		}
		stmt = fmt.Sprintf(`ALTER TABLE "%s" %s`, table, strings.Join(clauses, ", "))
	case "mysql":
		opts := []string{typ}
		if nullable != nil && !*nullable {
			opts = append(opts, "NOT NULL")
		}
		if defClause != "" {
			opts = append(opts, defClause)
		}
		if onUpdateClause != "" {
			opts = append(opts, onUpdateClause)
		}
		stmt = fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `%s` %s", table, column, strings.Join(opts, " "))
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("modify column: %w", err)
		}
		if unique != nil {
			name := fmt.Sprintf("%s_%s_key", table, column)
			if *unique {
				add := fmt.Sprintf("ALTER TABLE `%s` ADD CONSTRAINT `%s` UNIQUE (`%s`)", table, name, column)
				if _, err := db.ExecContext(ctx, add); err != nil {
					return fmt.Errorf("add unique: %w", err)
				}
			} else {
				var cnt int
				err := db.QueryRowContext(ctx,
					`SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?`,
					table, name).Scan(&cnt)
				if err != nil {
					return fmt.Errorf("failed to check index existence: %w", err)
				}
				if cnt > 0 {
					drop := fmt.Sprintf("ALTER TABLE `%s` DROP INDEX `%s`", table, name)
					if _, err := db.ExecContext(ctx, drop); err != nil {
						return fmt.Errorf("drop index: %w", err)
					}
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported driver: %s", driver)
	}
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("modify column: %w", err)
	}
	return nil
}

func DropColumnSQL(ctx context.Context, db *sql.DB, driver, table, column string) error {
	var stmt string
	switch driver {
	case "postgres":
		stmt = fmt.Sprintf(`ALTER TABLE "%s" DROP COLUMN IF EXISTS "%s"`, table, column)
	case "mysql":
		// MySQL < 8.0 does not support IF EXISTS for DROP COLUMN.
		// Check whether the column exists before attempting to drop it.
		var columnCount int
		err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`,
			table, column).Scan(&columnCount)
		if err != nil {
			return fmt.Errorf("failed to check column existence: %w", err)
		}
		if columnCount == 0 {
			return nil
		}
		stmt = fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", quoteIdentifier(driver, table), quoteIdentifier(driver, column))
	default:
		return fmt.Errorf("unsupported driver: %s", driver)
	}
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("drop column: %w", err)
	}
	return nil
}
