package registrycmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/pkg/migrator"
	"github.com/faciam-dev/gcfm/sdk"
)

// NewMigrateCmd creates the migrate subcommand.
func NewMigrateCmd() *cobra.Command {
	var (
		to     int
		dryRun bool
		dbDSN  string
		driver string
	)
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate registry schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbDSN == "" {
				return fmt.Errorf("--db is required")
			}
			svc := sdk.New(sdk.ServiceConfig{})
			ctx := context.Background()
			if dryRun {
				cur, err := svc.RegistryVersion(ctx, sdk.DBConfig{Driver: driver, DSN: dbDSN})
				if err != nil {
					return err
				}
				m := migrator.NewWithDriver(driver)
				target := to
				if target == 0 {
					target = len(migrator.DefaultForDriver(driver))
				}
				sqls := m.SQLForRange(cur, target)
				for _, s := range sqls {
					fmt.Fprintln(cmd.OutOrStdout(), s+";")
				}
				return nil
			}
			return svc.MigrateRegistry(ctx, sdk.DBConfig{Driver: driver, DSN: dbDSN}, to)
		},
	}
	cmd.Flags().StringVar(&dbDSN, "db", "", "database DSN")
	cmd.Flags().StringVar(&driver, "driver", "", "database driver")
	cmd.Flags().IntVar(&to, "to", 0, "target version (0=latest)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print SQL without executing")
	cobra.CheckErr(cmd.MarkFlagRequired("db"))
	return cmd
}
