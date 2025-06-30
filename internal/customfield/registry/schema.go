package registry

import (
	"context"
	"database/sql"
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

func AddColumnSQL(ctx context.Context, db *sql.DB, driver, table, column, typ string) error {
	typ = normalizeType(driver, typ)
	var stmt string
	switch driver {
	case "postgres":
		stmt = fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN "%s" %s`, table, column, typ)
	case "mysql":
		stmt = fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` %s", table, column, typ)
	default:
		return fmt.Errorf("unsupported driver: %s", driver)
	}
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("add column: %w", err)
	}
	return nil
}

func ModifyColumnSQL(ctx context.Context, db *sql.DB, driver, table, column, typ string) error {
	typ = normalizeType(driver, typ)
	var stmt string
	switch driver {
	case "postgres":
		stmt = fmt.Sprintf(`ALTER TABLE "%s" ALTER COLUMN "%s" TYPE %s`, table, column, typ)
	case "mysql":
		stmt = fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `%s` %s", table, column, typ)
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
		stmt = fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN IF EXISTS `%s`", table, column)
	default:
		return fmt.Errorf("unsupported driver: %s", driver)
	}
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("drop column: %w", err)
	}
	return nil
}
