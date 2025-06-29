package main

import "github.com/spf13/cobra"

func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate helper files",
	}
	cmd.AddCommand(newPluginImportCmd())
	return cmd
}
