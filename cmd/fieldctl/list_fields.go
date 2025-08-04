package main

import (
	"database/sql"
	"fmt"

	dbcmd "github.com/faciam-dev/gcfm/cmd/fieldctl/db"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/spf13/cobra"
)

func newListFieldsCmd() *cobra.Command {
	var f dbcmd.DBFlags
	var tenant string
	var dbID int64
	var table string
	cmd := &cobra.Command{Use: "list-fields", Short: "List custom fields", RunE: func(cmd *cobra.Command, args []string) error {
		db, err := sql.Open(f.Driver, f.DSN)
		if err != nil {
			return err
		}
		defer db.Close()
		metas, err := registry.LoadSQLByDB(cmd.Context(), db, registry.DBConfig{Driver: f.Driver, Schema: f.Schema}, tenant, dbID)
		if err != nil {
			return err
		}
		for _, m := range metas {
			if table != "" && m.TableName != table {
				continue
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s.%s\t%s\n", m.TableName, m.ColumnName, m.DataType)
		}
		return nil
	}}
	f.AddFlags(cmd)
	cmd.Flags().Int64Var(&dbID, "db-id", 1, "database id")
	cmd.Flags().StringVar(&tenant, "tenant", "default", "tenant id")
	cmd.Flags().StringVar(&table, "table", "", "filter table")
	return cmd
}
