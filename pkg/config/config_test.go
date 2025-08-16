package config

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLoadSave(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &File{
		Active: "p1",
		Profiles: map[string]Profile{
			"p1": {Name: "p1", APIURL: "http://api", Token: "tok", Insecure: true},
		},
		Version: 1,
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	p, err := Path()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %v", info.Mode().Perm())
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if diff := cmp.Diff(cfg, loaded); diff != "" {
		t.Fatalf("cfg diff (-want +got)\n%s", diff)
	}
}
