package main

import "github.com/spf13/cobra"

func newPluginsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "Manage validator plugins",
	}
	cmd.AddCommand(newPluginsInstallCmd())
	cmd.AddCommand(newPluginsListCmd())
	cmd.AddCommand(newPluginsRemoveCmd())
	return cmd
}
