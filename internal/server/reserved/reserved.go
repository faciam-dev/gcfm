package reserved

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var patterns []*regexp.Regexp

// Load reads reserved tables from the given YAML file. If the environment
// variable CF_RESERVED_TABLES is set, it overrides the file contents.
func Load(path string) {
	patterns = nil
	p := filepath.Clean(path)
	data, err := os.ReadFile(p) // #nosec G304 -- configuration path provided by operator
	if err == nil {
		var cfg struct {
			Reserved []string `yaml:"reserved_tables"`
			Legacy   []string `yaml:"reservedTables"`
		}
		if yamlErr := yaml.Unmarshal(data, &cfg); yamlErr == nil {
			list := cfg.Reserved
			if len(list) == 0 {
				for _, t := range cfg.Legacy {
					list = append(list, "^"+t+"$")
				}
			}
			for _, p := range list {
				if r, err := regexp.Compile(p); err == nil {
					patterns = append(patterns, r)
				}
			}
		}
	}
	if env := os.Getenv("CF_RESERVED_TABLES"); env != "" {
		patterns = nil
		for _, t := range strings.Split(env, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				if r, err := regexp.Compile(t); err == nil {
					patterns = append(patterns, r)
				}
			}
		}
	}
}

// Is returns true if the table is reserved.
func Is(table string) bool {
	for _, r := range patterns {
		if r.MatchString(table) {
			return true
		}
	}
	return false
}

// Patterns returns the reserved table regex patterns as strings.
func Patterns() []string {
	out := make([]string, len(patterns))
	for i, r := range patterns {
		out[i] = r.String()
	}
	return out
}
