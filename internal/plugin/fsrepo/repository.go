package fsrepo

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	pluginloader "github.com/faciam-dev/gcfm/internal/customfield/pluginloader"
	"github.com/faciam-dev/gcfm/internal/plugin"
)

// Repository lists plugins from the filesystem.
type Repository struct {
	Dir string
}

// List returns plugins found in the repository directory.
func (r *Repository) List(ctx context.Context) ([]plugin.Plugin, error) {
	dir := r.Dir
	if dir == "" {
		dir = pluginloader.DefaultDir()
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []plugin.Plugin{}, nil
		}
		return nil, err
	}
	var plugins []plugin.Plugin
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".so" {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".so")
		plugins = append(plugins, plugin.Plugin{Name: name, Type: "widget"})
	}
	return plugins, nil
}
