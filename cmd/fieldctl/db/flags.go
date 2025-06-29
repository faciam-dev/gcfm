package dbcmd

import (
	"log"
	"net/url"

	"github.com/spf13/cobra"
)

// DBFlags defines common database flags.
type DBFlags struct {
	Driver string
	DSN    string
	Schema string
}

// AddFlags attaches the DB flags to the command.
func (f *DBFlags) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.DSN, "db", "", "database DSN")
	cmd.Flags().StringVar(&f.Schema, "schema", "", "database schema")
	cmd.Flags().StringVar(&f.Driver, "driver", "", "database driver")
}

// DetectDriver returns the driver based on DSN scheme.
func DetectDriver(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		log.Printf("parse DSN: %v", err)
		return "unknown"
	}
	switch u.Scheme {
	case "postgres", "postgresql":
		return "postgres"
	case "mysql":
		return "mysql"
	default:
		return "unknown"
	}
}
