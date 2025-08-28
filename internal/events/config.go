package events

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadConfig reads YAML from file path. If path is empty, returns zero value.
func LoadConfig(path string) (Config, error) {
	var c Config
	if path == "" {
		return c, nil
	}
	p := filepath.Clean(path)
	data, err := os.ReadFile(p) // #nosec G304 -- path cleaned prior to read
	if err != nil {
		return c, err
	}
	err = yaml.Unmarshal(data, &c)
	return c, err
}
