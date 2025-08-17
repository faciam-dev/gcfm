package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/internal/customfield/snapshot"
	"github.com/faciam-dev/gcfm/sdk"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
)

func newSnapshotCmd() *cobra.Command {
	var (
		dbDSN       string
		schema      string
		tenant      string
		bump        string
		driverFlag  string
		message     string
		tablePrefix string
	)
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Create registry snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			svc := sdk.New(sdk.ServiceConfig{})
			data, err := svc.Export(ctx, sdk.DBConfig{Driver: driverFlag, DSN: dbDSN, Schema: schema, TablePrefix: tablePrefix})
			if err != nil {
				return err
			}
			comp, err := snapshot.Encode(data)
			if err != nil {
				return err
			}
			var dialect ormdriver.Dialect
			if driverFlag == "postgres" {
				dialect = ormdriver.PostgresDialect{}
			} else {
				dialect = ormdriver.MySQLDialect{}
			}
			last, err := snapshot.LatestSemver(ctx, db, dialect, tablePrefix, tenant)
			if err != nil {
				return err
			}
			if bump == "" {
				bump = "patch"
			}
			ver := snapshot.NextSemver(last, bump)
			rec, err := snapshot.Insert(ctx, db, dialect, tablePrefix, tenant, ver, message, comp)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), rec.Semver)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbDSN, "db", "", "database DSN")
	cmd.Flags().StringVar(&schema, "schema", "", "database schema")
	cmd.Flags().StringVar(&driverFlag, "driver", "", "database driver (mysql|postgres|mongo)")
	cmd.Flags().StringVar(&tenant, "tenant", getenv("CF_TENANT", "default"), "tenant id")
	cmd.Flags().StringVar(&bump, "bump", "patch", "semver bump type")
	cmd.Flags().StringVar(&message, "message", "", "snapshot message")
	cmd.Flags().StringVar(&tablePrefix, "table-prefix", getenv("CF_TABLE_PREFIX", "gcfm_"), "table name prefix")
	cmd.MarkFlagRequired("db")
	cmd.MarkFlagRequired("schema")
	return cmd
}
