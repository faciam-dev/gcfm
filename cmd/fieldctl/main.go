package main

import (
	"log"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{Use: "fieldctl"}

func init() {
	rootCmd.PersistentFlags().String("api-url", "", "Admin API base URL")
	rootCmd.PersistentFlags().String("token", "", "Bearer token for Admin API")
	rootCmd.PersistentFlags().String("profile", "", "Profile name in config (overrides active)")
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
	rootCmd.AddCommand(newLoginCmd())
	rootCmd.AddCommand(newConfigCmd())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
