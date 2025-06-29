package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/sdk"
)

func newApplyCmd() *cobra.Command {
	var (
		file       string
		dryRun     bool
		dbDSN      string
		schema     string
		driverFlag string
	)
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply registry YAML to database",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return errors.New("--file is required")
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			ctx := context.Background()
			svc := sdk.New(sdk.ServiceConfig{})
			rep, err := svc.Apply(ctx, sdk.DBConfig{Driver: driverFlag, DSN: dbDSN, Schema: schema}, data, sdk.ApplyOptions{DryRun: dryRun})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "+%d/-%d/±%d updated\n", rep.Added, rep.Deleted, rep.Updated)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbDSN, "db", "", "database DSN")
	cmd.Flags().StringVar(&schema, "schema", "", "database schema")
	cmd.Flags().StringVar(&file, "file", "registry.yaml", "input file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show diff without applying")
	cmd.Flags().StringVar(&driverFlag, "driver", "", "database driver (mysql|postgres|mongo)")
	cmd.MarkFlagRequired("db")
	cmd.MarkFlagRequired("schema")
	return cmd
}
