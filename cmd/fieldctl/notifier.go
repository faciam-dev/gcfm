package main

import "github.com/spf13/cobra"

func newNotifierCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "notifier"}
	cmd.AddCommand(NewRunCmd())
	return cmd
}
