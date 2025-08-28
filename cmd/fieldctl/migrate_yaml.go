package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/pkg/registry/codec"
)

func newMigrateYAMLCmd() *cobra.Command {
	var inFile string
	var outFile string
	cmd := &cobra.Command{
		Use:   "migrate-yaml",
		Short: "Migrate registry YAML to v0.2",
		RunE: func(cmd *cobra.Command, args []string) error {
			if inFile == "" || outFile == "" {
				return errors.New("--in and --out are required")
			}
			inPath := filepath.Clean(inFile)
			outPath := filepath.Clean(outFile)
			data, err := os.ReadFile(inPath) // #nosec G304 -- file paths cleaned
			if err != nil {
				return err
			}
			metas, err := codec.DecodeYAML(data)
			if err != nil {
				return err
			}
			outData, err := codec.EncodeYAML(metas)
			if err != nil {
				return err
			}
			return os.WriteFile(outPath, outData, 0o600)
		},
	}
	cmd.Flags().StringVar(&inFile, "in", "", "input YAML file")
	cmd.Flags().StringVar(&outFile, "out", "", "output YAML file")
	mustFlag(cmd, "in")
	mustFlag(cmd, "out")
	return cmd
}
