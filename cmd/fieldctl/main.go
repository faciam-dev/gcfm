package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/internal/customfield/registry/codec"
	"github.com/faciam-dev/gcfm/sdk"
)

var rootCmd = &cobra.Command{Use: "fieldctl"}

var (
	dbDSN      string
	schema     string
	dryRun     bool
	driverFlag string
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan schema and register metadata",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		svc := sdk.New(sdk.ServiceConfig{})
		metas, err := svc.Scan(ctx, sdk.DBConfig{Driver: driverFlag, DSN: dbDSN, Schema: schema})
		if err != nil {
			return err
		}
		if dryRun {
			for _, m := range metas {
				fmt.Fprintf(cmd.OutOrStdout(), "%s.%s\t%s\n", m.TableName, m.ColumnName, m.DataType)
			}
			return nil
		}
		data, err := codec.EncodeYAML(metas)
		if err != nil {
			return err
		}
		if _, err := svc.Apply(ctx, sdk.DBConfig{Driver: driverFlag, DSN: dbDSN, Schema: schema}, data, sdk.ApplyOptions{}); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "scanned %d fields\n", len(metas))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(newExportCmd())
	rootCmd.AddCommand(newApplyCmd())
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newMigrateYAMLCmd())
	rootCmd.AddCommand(newPluginsCmd())
	rootCmd.AddCommand(newRegistryCmd())
	rootCmd.AddCommand(newDBCmd())
	rootCmd.AddCommand(newUserCmd())
	rootCmd.AddCommand(newGenerateCmd())
	rootCmd.AddCommand(newGenDocsCmd())
	rootCmd.AddCommand(newSnapshotCmd())
	rootCmd.AddCommand(newNotifierCmd())
	rootCmd.AddCommand(newEventsCmd())
	rootCmd.AddCommand(newDiffCmd())
	scanCmd.Flags().StringVar(&dbDSN, "db", "", "database DSN")
	scanCmd.Flags().StringVar(&schema, "schema", "", "database schema")
	scanCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print fields without upsert")
	scanCmd.Flags().StringVar(&driverFlag, "driver", "", "database driver (mysql|postgres|mongo)")
	scanCmd.MarkFlagRequired("db")
	scanCmd.MarkFlagRequired("schema")
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
		if errors.Is(err, errDiffDetected) {
			os.Exit(2)
		}
		log.Fatal(err)
	}
}
