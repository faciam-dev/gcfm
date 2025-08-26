package main

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/internal/monitordb"
	"github.com/faciam-dev/gcfm/internal/server/reserved"
	"github.com/faciam-dev/gcfm/pkg/crypto"
	"github.com/faciam-dev/gcfm/pkg/registry/codec"
	"github.com/faciam-dev/gcfm/pkg/util"
	"github.com/faciam-dev/gcfm/sdk"
)

var (
	dbDSN      string
	schema     string
	dryRun     bool
	driverFlag string
	scanDBID   int64
	verbose    bool
)

func newScanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan schema and register metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if scanDBID != 0 {
				if err := crypto.CheckEnv(); err != nil {
					return err
				}
				_, f, _, _ := runtime.Caller(0)
				base := filepath.Join(filepath.Dir(f), "..", "..")
				reserved.Load(filepath.Join(base, "configs", "default.yaml"))
				db, err := sql.Open(driverFlag, dbDSN)
				if err != nil {
					return err
				}
				defer db.Close()
				dialect := util.DialectFromDriver(driverFlag)
				repo := &monitordb.Repo{DB: db, Driver: driverFlag, Dialect: dialect, TablePrefix: "gcfm_"}
				d, err := repo.Get(ctx, "default", scanDBID)
				if err != nil {
					return err
				}
				var dsn string
				if verbose {
					b, err := crypto.Decrypt(d.DSNEnc)
					if err != nil {
						return err
					}
					dsn = string(b)
					fmt.Fprintf(cmd.ErrOrStderr(), "driver=%s tenant=%s dsn=%s\n", d.Driver, d.TenantID, dsn)
				}
				tables, inserted, updated, skipped, err := monitordb.ScanDatabase(ctx, repo, scanDBID, d.TenantID)
				if err != nil {
					return err
				}
				if verbose {
					fmt.Fprintf(cmd.OutOrStdout(), "tables=%d inserted=%d updated=%d skipped=%d\n", tables, inserted, updated, len(skipped))
					if len(skipped) > 0 {
						tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
						fmt.Fprintln(tw, "TABLE\tCOLUMN\tREASON")
						for _, s := range skipped {
							fmt.Fprintf(tw, "%s\t%s\t%s\n", s.Table, s.Column, s.Reason)
						}
						tw.Flush()
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Scan finished: %d inserted, %d updated, %d skipped\n", inserted, updated, len(skipped))
				return nil
			}
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
	cmd.Flags().StringVar(&dbDSN, "db", "", "database DSN")
	cmd.Flags().StringVar(&schema, "schema", "", "database schema")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print fields without upsert")
	cmd.Flags().StringVar(&driverFlag, "driver", "", "database driver (mysql|postgres|mongo)")
	cmd.Flags().Int64Var(&scanDBID, "db-id", 0, "monitored database id")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	cmd.MarkFlagRequired("db")
	cmd.MarkFlagRequired("schema")
	return cmd
}
