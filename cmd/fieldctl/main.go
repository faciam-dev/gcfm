package main

import (
	"log"
	"net/url"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{Use: "fieldctl"}

func init() {
	rootCmd.PersistentFlags().String("api-url", getenv("FIELDTOOL_API_URL", ""), "Admin API base URL")
	rootCmd.PersistentFlags().String("token", getenv("FIELDTOOL_TOKEN", ""), "Bearer token for Admin API")
	rootCmd.PersistentFlags().String("output", "table", "Output format (table|json)")

	rootCmd.AddCommand(newScanCmd())
	rootCmd.AddCommand(newExportCmd())
	rootCmd.AddCommand(newApplyCmd())
	rootCmd.AddCommand(newDiffCmd())
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newMigrateYAMLCmd())
	rootCmd.AddCommand(newPluginsCmd())
	rootCmd.AddCommand(newRegistryCmd())
	rootCmd.AddCommand(newDBCmd())
	rootCmd.AddCommand(newUserCmd())
	rootCmd.AddCommand(newGenerateCmd())
	rootCmd.AddCommand(newGenDocsCmd())
	rootCmd.AddCommand(newSnapshotCmd())
	rootCmd.AddCommand(newRevertCmd())
	rootCmd.AddCommand(newDiffSnapCmd())
	rootCmd.AddCommand(newNotifierCmd())
	rootCmd.AddCommand(newEventsCmd())
	rootCmd.AddCommand(newListFieldsCmd())
}

// detectDriver returns the driver name based on the DSN scheme.
// Supported schemes: mysql, postgres/postgresql and mongodb/mongodb+srv.
func detectDriver(dsn string) string {
	parsedURL, err := url.Parse(dsn)
	if err != nil {
		log.Printf("Error parsing DSN: %v", err)
		return "unknown"
	}

	switch parsedURL.Scheme {
	case "postgres", "postgresql":
		return "postgres"
	case "mongodb", "mongodb+srv":
		return "mongo"
	case "mysql":
		return "mysql"
	default:
		return "unknown"
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
