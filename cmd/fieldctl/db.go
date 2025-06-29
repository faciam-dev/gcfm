package main

import (
	dbcmd "github.com/faciam-dev/gcfm/cmd/fieldctl/db"
	"github.com/spf13/cobra"
)

func newDBCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "db", Short: "Database operations"}
	cmd.AddCommand(dbcmd.NewMigrateCmd())
	return cmd
}
