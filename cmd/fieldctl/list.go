package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newPluginsListCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir = resolvePluginDir(dir)
			files, _ := filepath.Glob(filepath.Join(dir, "*.so"))
			for _, f := range files {
				name := strings.TrimSuffix(filepath.Base(f), ".so")
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", name, f)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "plugin directory")
	return cmd
}
