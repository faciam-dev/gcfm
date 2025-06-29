package gen

import (
	"errors"
	"fmt"
	"os"

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
			b, err := generator.GenerateYAMLFromGo(generator.YAMLFromGoOptions{Srcs: srcs, Merge: merge})
			if err != nil {
				return err
			}
			if out == "" {
				_, err := cmd.OutOrStdout().Write(b)
				return err
			}
			if merge && out != "" {
				if existing, err := os.ReadFile(out); err == nil {
					b, err = generator.GenerateYAMLFromGo(generator.YAMLFromGoOptions{Srcs: srcs, Merge: true, Existing: existing})
					if err != nil {
						return err
					}
				}
			}
			if out != "" {
				if err := os.WriteFile(out, b, 0644); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", out)
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
