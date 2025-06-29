package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/sdk"
)

func newExportCmd() *cobra.Command {
	var (
		out        string
		force      bool
		dbDSN      string
		schema     string
		driverFlag string
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export registry to YAML",
		RunE: func(cmd *cobra.Command, args []string) error {
			if out == "" {
				return errors.New("--out is required")
			}
			if _, err := os.Stat(out); err == nil && !force {
				return fmt.Errorf("%s exists (use --force to overwrite)", out)
			}
			ctx := context.Background()
			svc := sdk.New(sdk.ServiceConfig{})
			data, err := svc.Export(ctx, sdk.DBConfig{Driver: driverFlag, DSN: dbDSN, Schema: schema})
			if err != nil {
				return err
			}
			if err := os.WriteFile(out, data, 0644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "exported registry to %s\n", out)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbDSN, "db", "", "database DSN")
	cmd.Flags().StringVar(&schema, "schema", "", "database schema")
	cmd.Flags().StringVar(&out, "out", "registry.yaml", "output file")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite without confirmation")
	cmd.Flags().StringVar(&driverFlag, "driver", "", "database driver (mysql|postgres|mongo)")
	cmd.MarkFlagRequired("db")
	cmd.MarkFlagRequired("schema")
	return cmd
}
