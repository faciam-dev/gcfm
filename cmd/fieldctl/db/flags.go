package dbcmd

import (
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

// DBFlags defines common database flags.
type DBFlags struct {
	Driver      string
	DSN         string
	Schema      string
	TablePrefix string
}

// AddFlags attaches the DB flags to the command.
func (f *DBFlags) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.DSN, "db", "", "database DSN")
	cmd.Flags().StringVar(&f.Schema, "schema", "", "database schema")
	cmd.Flags().StringVar(&f.Driver, "driver", "", "database driver")
	cmd.Flags().StringVar(&f.TablePrefix, "table-prefix", getenv("CF_TABLE_PREFIX", "gcfm_"), "table name prefix")
}

// DetectDriver returns the driver based on DSN scheme.
func DetectDriver(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("failed to parse DSN: %w", err)
	}
	switch u.Scheme {
	case "postgres", "postgresql":
		return "postgres", nil
	case "mysql":
		return "mysql", nil
	default:
		return "", fmt.Errorf("unsupported DSN scheme: %s", u.Scheme)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
