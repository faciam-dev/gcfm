package main

import (
	"github.com/spf13/cobra"

	gencmd "github.com/faciam-dev/gcfm/cmd/fieldctl/gen"
)

func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generate",
		Short:   "Generate helper files",
		Aliases: []string{"gen"},
	}
	cmd.AddCommand(newPluginImportCmd())
	cmd.AddCommand(gencmd.NewGoCmd())
	cmd.AddCommand(gencmd.NewRegistryCmd())
	return cmd
}
