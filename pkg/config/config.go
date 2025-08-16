package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Profile struct {
	Name     string `json:"name"`
	APIURL   string `json:"apiUrl"`
	Token    string `json:"token"`
	Insecure bool   `json:"insecure"`
}

type File struct {
	Active   string             `json:"active"`
	Profiles map[string]Profile `json:"profiles"`
	Version  int                `json:"version"`
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".fieldctl")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (*File, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &File{Active: "default", Profiles: map[string]Profile{}, Version: 1}, nil
		}
		return nil, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	if f.Profiles == nil {
		f.Profiles = map[string]Profile{}
	}
	if f.Active == "" {
		f.Active = "default"
	}
	if f.Version == 0 {
		f.Version = 1
	}
	return &f, nil
}

func Save(f *File) error {
	p, err := Path()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
