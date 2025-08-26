package dbcmd

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/pkg/util"
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
	cmd.Flags().StringVar(&f.TablePrefix, "table-prefix", util.GetEnv("CF_TABLE_PREFIX", "gcfm_"), "table name prefix")
}

// DetectDriver returns the driver based on DSN scheme.
func DetectDriver(dsn string) (string, error) {
	// Retained for backward compatibility.
	if _, err := url.Parse(dsn); err != nil {
		return "", fmt.Errorf("failed to parse DSN: %w", err)
	}
	return util.DetectDriver(dsn)
}
