package config

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Config holds global configuration values.
type Config struct {
	TablePrefix string `env:"TABLE_PREFIX,default=gcfm_"`
}

// T prefixes the given table name with the configured prefix.
func (c *Config) T(name string) string {
	return c.TablePrefix + name
}

// CheckPrefix verifies that tables with the configured prefix exist in the
// connected database. It returns an error if none are found.
func CheckPrefix(ctx context.Context, db *sql.DB, driver, prefix string) error {
	var q string
	switch strings.ToLower(driver) {
	case "mysql":
		q = "SELECT COUNT(*) FROM information_schema.tables WHERE table_name LIKE ?"
	default: // postgres and others
		q = "SELECT COUNT(*) FROM information_schema.tables WHERE table_name LIKE $1"
	}
	var n int
	if err := db.QueryRowContext(ctx, q, prefix+"%").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no tables with prefix %q found; run migrations or set TABLE_PREFIX correctly", prefix)
	}
	return nil
}
