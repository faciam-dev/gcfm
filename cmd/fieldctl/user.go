package main

import (
	usercmd "github.com/faciam-dev/gcfm/cmd/fieldctl/user"
	"github.com/spf13/cobra"
)

func newUserCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "user", Short: "Manage users"}
	cmd.AddCommand(usercmd.NewCreateCmd())
	cmd.AddCommand(usercmd.NewListCmd())
	cmd.AddCommand(usercmd.NewDeleteCmd())
	return cmd
}
