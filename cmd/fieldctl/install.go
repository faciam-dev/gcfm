package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newPluginsInstallCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "install [module@version]",
		Short: "Install validator plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mod := args[0]
			dir = resolvePluginDir(dir)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
			name := filepath.Base(strings.Split(mod, "@")[0]) + ".so"
			dst := filepath.Join(dir, name)
			tmp := dst + ".tmp"
			buildCmd := exec.Command("go", "build", "-buildmode=plugin", "-o", tmp, mod)
			buildCmd.Env = os.Environ()
			if out, err := buildCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("go build: %v\n%s", err, out)
			}
			return os.Rename(tmp, dst)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "plugin directory")
	return cmd
}
