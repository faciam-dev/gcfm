package events

import (
	"os"

	"gopkg.in/yaml.v3"
)

// LoadConfig reads YAML from file path. If path is empty, returns zero value.
func LoadConfig(path string) (Config, error) {
	var c Config
	if path == "" {
		return c, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	err = yaml.Unmarshal(data, &c)
	return c, err
}
