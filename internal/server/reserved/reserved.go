package reserved

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

var tables map[string]struct{}

// Load reads reserved tables from the given YAML file. If the environment
// variable CF_RESERVED_TABLES is set, it overrides the file contents.
func Load(path string) {
	tables = map[string]struct{}{}
	data, err := os.ReadFile(path)
	if err == nil {
		var cfg struct {
			Reserved []string `yaml:"reservedTables"`
		}
		if yamlErr := yaml.Unmarshal(data, &cfg); yamlErr == nil {
			for _, t := range cfg.Reserved {
				tables[t] = struct{}{}
			}
		}
	}
	if env := os.Getenv("CF_RESERVED_TABLES"); env != "" {
		tables = map[string]struct{}{}
		for _, t := range strings.Split(env, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tables[t] = struct{}{}
			}
		}
	}
}

// Is returns true if the table is reserved.
func Is(table string) bool {
	_, ok := tables[table]
	return ok
}
