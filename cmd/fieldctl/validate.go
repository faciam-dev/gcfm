package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/internal/customfield/registry/codec"
)

func newValidateCmd() *cobra.Command {
	var file string
	var checkUI bool
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate registry YAML",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return errors.New("--file is required")
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			metas, err := codec.DecodeYAML(data)
			if err != nil {
				return err
			}
			if checkUI {
				var missing int
				for _, m := range metas {
					if m.Display == nil {
						missing++
					}
				}
				if missing > 0 {
					return fmt.Errorf("%d fields missing display", missing)
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "registry.yaml", "input file")
	cmd.Flags().BoolVar(&checkUI, "ui", false, "validate display metadata")
	return cmd
}
