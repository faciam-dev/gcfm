package gen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/internal/generator"
)

func NewRegistryCmd() *cobra.Command {
	var srcs []string
	var out string
	var merge bool
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Generate registry YAML from Go structs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(srcs) == 0 {
				return errors.New("--src is required")
			}
			cleanOut := filepath.Clean(out)
			b, err := generator.GenerateYAMLFromGo(generator.YAMLFromGoOptions{Srcs: srcs, Merge: merge})
			if err != nil {
				return err
			}
			if cleanOut == "" {
				_, err := cmd.OutOrStdout().Write(b)
				return err
			}
			if merge && cleanOut != "" {
				if existing, err := os.ReadFile(cleanOut); err == nil { // #nosec G304 -- path cleaned
					b, err = generator.GenerateYAMLFromGo(generator.YAMLFromGoOptions{Srcs: srcs, Merge: true, Existing: existing})
					if err != nil {
						return err
					}
				}
			}
			if cleanOut != "" {
				if err := os.WriteFile(cleanOut, b, 0o600); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", cleanOut)
				return nil
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&srcs, "src", nil, "source Go files (glob)")
	cmd.Flags().StringVar(&out, "out", "", "output YAML file")
	cmd.Flags().BoolVar(&merge, "merge", false, "merge with existing file")
	return cmd
}
