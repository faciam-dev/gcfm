package main

import "github.com/spf13/cobra"

// mustFlag marks a flag as required and panics on error.
func mustFlag(cmd *cobra.Command, name string) {
	cobra.CheckErr(cmd.MarkFlagRequired(name))
}
