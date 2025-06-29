package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func newGenDocsCmd() *cobra.Command {
	var dir string
	var format string
	cmd := &cobra.Command{
		Use:    "gen-docs",
		Short:  "Generate CLI documentation",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				return fmt.Errorf("--dir is required")
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			switch format {
			case "markdown":
				return doc.GenMarkdownTree(rootCmd, dir)
			default:
				return fmt.Errorf("unsupported format: %s", format)
			}
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "./docs/cli", "output directory")
	cmd.Flags().StringVar(&format, "format", "markdown", "documentation format")
	return cmd
}
