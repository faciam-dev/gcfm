package main

import (
	registrycmd "github.com/faciam-dev/gcfm/cmd/fieldctl/registry"
	"github.com/spf13/cobra"
)

func newRegistryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Registry schema operations",
	}
	cmd.AddCommand(registrycmd.NewMigrateCmd())
	cmd.AddCommand(registrycmd.NewVersionCmd())
	return cmd
}
