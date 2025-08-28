package gen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/internal/generator"
)

func NewGoCmd() *cobra.Command {
	var pkg string
	var out string
	var table string
	var file string
	cmd := &cobra.Command{
		Use:   "go",
		Short: "Generate Go struct from registry YAML",
		RunE: func(cmd *cobra.Command, args []string) error {
			if pkg == "" || out == "" || table == "" {
				return errors.New("--pkg, --out and --table are required")
			}
			if file == "" {
				file = "registry.yaml"
			}
			inPath := filepath.Clean(file)
			outPath := filepath.Clean(out)
			data, err := os.ReadFile(inPath) // #nosec G304 -- path cleaned
			if err != nil {
				return err
			}
			b, err := generator.GenerateGoFromYAML(data, generator.GoFromYAMLOptions{Package: pkg, Table: table})
			if err != nil {
				return err
			}
			if err := os.WriteFile(outPath, b, 0o600); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "generated %s\n", outPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&pkg, "pkg", "", "package name")
	cmd.Flags().StringVar(&out, "out", "", "output file")
	cmd.Flags().StringVar(&table, "table", "", "table name")
	cmd.Flags().StringVar(&file, "file", "registry.yaml", "input registry YAML")
	return cmd
}
