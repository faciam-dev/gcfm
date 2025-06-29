package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newPluginsRemoveCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "remove [name]",
		Short: "Remove installed plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			dir = resolvePluginDir(dir)
			path := filepath.Join(dir, name+".so")
			return os.Remove(path)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "plugin directory")
	return cmd
}
