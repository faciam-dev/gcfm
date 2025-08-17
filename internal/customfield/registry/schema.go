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

func ColumnExists(ctx context.Context, db *sql.DB, dialect ormdriver.Dialect, schema, table, column string) (bool, error) {
	q := query.New(db, "information_schema.columns", dialect).
		SelectRaw("COUNT(*) as cnt").
		Where("table_schema", schema).
		Where("table_name", table).
		Where("column_name", column).
		WithContext(ctx)

	var res struct{ Cnt int }
	if err := q.First(&res); err != nil {
		return false, err
	}
	return res.Cnt > 0, nil
}

func supportsDefault(driver, typ string) bool {
	if driver != "mysql" {
		return true
	}
	t := strings.ToLower(strings.TrimSpace(typ))
	if strings.Contains(t, "text") || strings.Contains(t, "blob") || strings.Contains(t, "geometry") || strings.Contains(t, "json") {
		return false
	}
	return true
}

func AddColumnSQL(ctx context.Context, db *sql.DB, driver, table, column, typ string, nullable, unique *bool, def *string) error {
	typ = normalizeType(driver, typ)
	if def != nil && !supportsDefault(driver, typ) {
		def = nil
	}
	opts := []string{typ}
	if nullable != nil && !*nullable {
		opts = append(opts, "NOT NULL")
	}
	if def != nil {
		opts = append(opts, fmt.Sprintf("DEFAULT '%s'", escapeLiteral(*def)))
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

func ModifyColumnSQL(ctx context.Context, db *sql.DB, driver, table, column, typ string, nullable, unique *bool, def *string) error {
	typ = normalizeType(driver, typ)
	if def != nil && !supportsDefault(driver, typ) {
		def = nil
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
		if def != nil {
			clauses = append(clauses, fmt.Sprintf(`ALTER COLUMN "%s" SET DEFAULT '%s'`, column, escapeLiteral(*def)))
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
		if def != nil {
			opts = append(opts, fmt.Sprintf("DEFAULT '%s'", escapeLiteral(*def)))
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
				// MySQL stores the UNIQUE constraint as an index so it must be dropped separately.
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
