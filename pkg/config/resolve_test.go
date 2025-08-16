package config

import (
	"testing"

	"github.com/spf13/cobra"
)

func newRoot() *cobra.Command {
	cmd := &cobra.Command{Use: "root"}
	cmd.PersistentFlags().String("api-url", "", "")
	cmd.PersistentFlags().String("token", "", "")
	cmd.PersistentFlags().String("profile", "", "")
	return cmd
}

func TestResolvePrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &File{Active: "default", Profiles: map[string]Profile{"default": {Name: "default", APIURL: "cfg", Token: "cfgtok"}}, Version: 1}
	if err := Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	t.Run("config", func(t *testing.T) {
		root := newRoot()
		r, err := Resolve(root)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if r.APIURL != "cfg" || r.Token != "cfgtok" {
			t.Fatalf("unexpected %+v", r)
		}
	})

	t.Run("env", func(t *testing.T) {
		t.Setenv("FIELDTOOL_API_URL", "env")
		t.Setenv("FIELDTOOL_TOKEN", "envtok")
		defer t.Setenv("FIELDTOOL_API_URL", "")
		defer t.Setenv("FIELDTOOL_TOKEN", "")
		root := newRoot()
		r, err := Resolve(root)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if r.APIURL != "env" || r.Token != "envtok" {
			t.Fatalf("unexpected %+v", r)
		}
	})

	t.Run("flag", func(t *testing.T) {
		root := newRoot()
		if err := root.PersistentFlags().Set("api-url", "flag"); err != nil {
			t.Fatalf("set api-url: %v", err)
		}
		if err := root.PersistentFlags().Set("token", "flagtok"); err != nil {
			t.Fatalf("set token: %v", err)
		}
		r, err := Resolve(root)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if r.APIURL != "flag" || r.Token != "flagtok" {
			t.Fatalf("unexpected %+v", r)
		}
	})

	t.Run("profile flag", func(t *testing.T) {
		cfg.Profiles["p2"] = Profile{Name: "p2", APIURL: "p2", Token: "p2tok"}
		if err := Save(cfg); err != nil {
			t.Fatalf("save: %v", err)
		}
		root := newRoot()
		if err := root.PersistentFlags().Set("profile", "p2"); err != nil {
			t.Fatalf("set profile: %v", err)
		}
		r, err := Resolve(root)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if r.APIURL != "p2" || r.Token != "p2tok" || r.Profile != "p2" {
			t.Fatalf("unexpected %+v", r)
		}
	})
}
