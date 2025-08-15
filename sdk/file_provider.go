package sdk

import (
	"context"
	"encoding/json"
	"os"
)

// FileProvider reads target configurations from a JSON file.
type FileProvider struct{ path string }

// NewFileProvider creates a provider for the given file path.
func NewFileProvider(path string) *FileProvider { return &FileProvider{path: path} }

// Fetch loads target configs from the file, expanding environment variables in DSNs.
func (p *FileProvider) Fetch(ctx context.Context) (map[string]TargetConfig, string, string, error) {
	b, err := os.ReadFile(p.path)
	if err != nil {
		return nil, "", "", err
	}
	var v struct {
		Version string         `json:"version"`
		Default string         `json:"default"`
		Targets []TargetConfig `json:"targets"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, "", "", err
	}
	cfgs := make(map[string]TargetConfig, len(v.Targets))
	for _, t := range v.Targets {
		t.DSN = os.ExpandEnv(t.DSN)
		cfgs[t.Key] = t
	}
	return cfgs, v.Default, v.Version, nil
}
