package pluginloader

import (
	"errors"
	"os"
	"path/filepath"
	"plugin"
	"runtime"

	"go.uber.org/zap"

	"github.com/faciam-dev/gcfm/internal/customfield"
)

// DefaultDir returns the path where validator plugins are stored for the
// current OS.
func DefaultDir() string {
	if runtime.GOOS == "windows" {
		dir := os.Getenv("APPDATA")
		if dir == "" {
			if h, err := os.UserHomeDir(); err == nil {
				dir = filepath.Join(h, "AppData", "Roaming")
			}
		}
		return filepath.Join(dir, "customfield", "plugins")
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".customfield", "plugins")
	}
	return "./plugins"
}

// LoadAll loads all plugins from dir.
// If dir is empty, DefaultDir() is used.
// It returns an error if loading any plugin fails.
func LoadAll(dir string, logger *zap.SugaredLogger) error {
	if dir == "" {
		dir = DefaultDir()
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.so"))
	if err != nil {
		logger.Warnw("failed to read plugin directory", "dir", dir, "err", err)
	}
	for _, f := range files {
		p, err := plugin.Open(f)
		if err != nil {
			logger.Warnw("plugin open failed", "file", f, "err", err)
			continue
		}
		sym, err := p.Lookup("New")
		if err != nil {
			logger.Warnw("symbol missing", "file", f, "err", err)
			continue
		}
		ctor, ok := sym.(func() customfield.ValidatorPlugin)
		if !ok {
			logger.Warnw("invalid type", "file", f)
			continue
		}
		inst := ctor()
		if err := customfield.RegisterValidator(inst.Name(), inst.Validate); err != nil {
			if errors.Is(err, customfield.ErrValidatorExists) {
				logger.Warnw("validator already registered", "name", inst.Name(), "file", f)
				continue
			}
			return err
		}
		logger.Infow("validator plugin loaded", "name", inst.Name(), "file", f)
	}
	return nil
}
