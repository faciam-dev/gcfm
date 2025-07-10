package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/internal/customfield/snapshot"
	"github.com/faciam-dev/gcfm/sdk"
)

func newDiffSnapCmd() *cobra.Command {
	var (
		dbDSN      string
		schema     string
		driverFlag string
		tenant     string
		fromVer    string
		toVer      string
	)
	cmd := &cobra.Command{
		Use:   "diff-snap",
		Short: "Diff two snapshots",
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromVer == "" || toVer == "" {
				return errors.New("--from and --to are required")
			}
			if dbDSN == "" {
				return errors.New("--db is required")
			}
			if schema == "" {
				return errors.New("--schema is required")
			}
			if driverFlag == "" {
				driverFlag = detectDriver(dbDSN)
			}
			db, err := sql.Open(driverFlag, dbDSN)
			if err != nil {
				return err
			}
			defer db.Close()
			ctx := context.Background()
			a, err := snapshot.Get(ctx, db, driverFlag, tenant, fromVer)
			if err != nil {
				return err
			}
			b, err := snapshot.Get(ctx, db, driverFlag, tenant, toVer)
			if err != nil {
				return err
			}
			ya, _ := snapshot.Decode(a.YAML)
			yb, _ := snapshot.Decode(b.YAML)
			diff := sdk.UnifiedDiff(string(ya), string(yb))
			fmt.Fprint(cmd.OutOrStdout(), diff)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbDSN, "db", "", "database DSN")
	cmd.Flags().StringVar(&schema, "schema", "", "database schema")
	cmd.Flags().StringVar(&driverFlag, "driver", "", "database driver (mysql|postgres|mongo)")
	cmd.Flags().StringVar(&tenant, "tenant", getenv("CF_TENANT", "default"), "tenant id")
	cmd.Flags().StringVar(&fromVer, "from", "", "from version")
	cmd.Flags().StringVar(&toVer, "to", "", "to version")
	cmd.MarkFlagRequired("db")
	cmd.MarkFlagRequired("schema")
	cmd.MarkFlagRequired("from")
	cmd.MarkFlagRequired("to")
	return cmd
}
