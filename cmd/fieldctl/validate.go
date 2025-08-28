package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/internal/server/reserved"
	"github.com/faciam-dev/gcfm/pkg/registry/codec"
)

func newValidateCmd() *cobra.Command {
	var file string
	var checkUI bool
	var checkReserved bool
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate registry YAML",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return errors.New("--file is required")
			}
			clean := filepath.Clean(file)
			data, err := os.ReadFile(clean) // #nosec G304 -- file path cleaned
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
			if checkReserved {
				_, f, _, _ := runtime.Caller(0)
				base := filepath.Join(filepath.Dir(f), "..", "..")
				reserved.Load(filepath.Join(base, "configs", "default.yaml"))
				for _, m := range metas {
					if reserved.Is(m.TableName) {
						return fmt.Errorf("table %s is reserved", m.TableName)
					}
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "registry.yaml", "input file")
	cmd.Flags().BoolVar(&checkUI, "ui", false, "validate display metadata")
	cmd.Flags().BoolVar(&checkReserved, "reserved", false, "validate reserved tables")
	return cmd
}
