package main

import (
	dbcmd "github.com/faciam-dev/gcfm/cmd/fieldctl/db"
	"github.com/spf13/cobra"
)

func newDBCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "db", Short: "Database operations"}
	cmd.AddCommand(dbcmd.NewMigrateCmd())
	cmd.AddCommand(dbcmd.NewAddMonCmd())
	cmd.AddCommand(dbcmd.NewListMonCmd())
	cmd.AddCommand(dbcmd.NewRemoveMonCmd())
	return cmd
}
