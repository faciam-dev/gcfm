package main

import (
	"context"
	"database/sql"
	"errors"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/internal/customfield/snapshot"
	"github.com/faciam-dev/gcfm/sdk"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
)

func newRevertCmd() *cobra.Command {
	var (
		dbDSN       string
		schema      string
		driverFlag  string
		tenant      string
		toVer       string
		tablePrefix string
	)
	cmd := &cobra.Command{
		Use:   "revert",
		Short: "Rollback to a snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			if toVer == "" {
				return errors.New("--to is required")
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
			var dialect ormdriver.Dialect
			if driverFlag == "postgres" {
				dialect = ormdriver.PostgresDialect{}
			} else {
				dialect = ormdriver.MySQLDialect{}
			}
			rec, err := snapshot.Get(ctx, db, dialect, tablePrefix, tenant, toVer)
			if err != nil {
				return err
			}
			data, err := snapshot.Decode(rec.YAML)
			if err != nil {
				return err
			}
			svc := sdk.New(sdk.ServiceConfig{})
			_, err = svc.Apply(ctx, sdk.DBConfig{Driver: driverFlag, DSN: dbDSN, Schema: schema, TablePrefix: tablePrefix}, data, sdk.ApplyOptions{})
			return err
		},
	}
	cmd.Flags().StringVar(&dbDSN, "db", "", "database DSN")
	cmd.Flags().StringVar(&schema, "schema", "", "database schema")
	cmd.Flags().StringVar(&driverFlag, "driver", "", "database driver (mysql|postgres|mongo)")
	cmd.Flags().StringVar(&tenant, "tenant", getenv("CF_TENANT", "default"), "tenant id")
	cmd.Flags().StringVar(&toVer, "to", "", "target snapshot version")
	cmd.Flags().StringVar(&tablePrefix, "table-prefix", getenv("CF_TABLE_PREFIX", "gcfm_"), "table name prefix")
	cmd.MarkFlagRequired("db")
	cmd.MarkFlagRequired("schema")
	cmd.MarkFlagRequired("to")
	return cmd
}
