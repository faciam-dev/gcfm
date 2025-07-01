package registry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
		return fmt.Errorf("%w", ErrDefaultNotSupported)
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
			uq := fmt.Sprintf("ALTER TABLE `%s` ADD UNIQUE (`%s`)", table, column)
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
		return fmt.Errorf("%w", ErrDefaultNotSupported)
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
			if *unique {
				add := fmt.Sprintf("ALTER TABLE `%s` ADD UNIQUE (`%s`)", table, column)
				if _, err := db.ExecContext(ctx, add); err != nil {
					return fmt.Errorf("add unique: %w", err)
				}
			} else {
				drop := fmt.Sprintf("ALTER TABLE `%s` DROP INDEX `%s`", table, column)
				if _, err := db.ExecContext(ctx, drop); err != nil {
					return fmt.Errorf("drop index: %w", err)
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
