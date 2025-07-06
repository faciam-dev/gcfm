package registrycmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/internal/customfield/migrator"
	"github.com/faciam-dev/gcfm/sdk"
)

// NewVersionCmd creates the version subcommand.
func NewVersionCmd() *cobra.Command {
	var (
		dbDSN  string
		driver string
	)
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show registry schema version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbDSN == "" {
				return fmt.Errorf("--db is required")
			}
			svc := sdk.New(sdk.ServiceConfig{})
			ctx := context.Background()
			cur, err := svc.RegistryVersion(ctx, sdk.DBConfig{Driver: driver, DSN: dbDSN})
			if err != nil {
				return err
			}
			m := migrator.New("")
			fmt.Fprintln(cmd.OutOrStdout(), m.SemVer(cur))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbDSN, "db", "", "database DSN")
	cmd.Flags().StringVar(&driver, "driver", "", "database driver")
	cmd.MarkFlagRequired("db")
	return cmd
}
