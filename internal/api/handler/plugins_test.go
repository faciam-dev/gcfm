package handler

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/faciam-dev/gcfm/internal/plugin"
	"github.com/faciam-dev/gcfm/internal/plugin/fsrepo"
)

func TestPluginList(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "example_widget.so")
	if err := os.WriteFile(file, []byte(""), 0o644); err != nil {
		t.Fatalf("write plugin: %v", err)
	}
	h := &PluginHandler{UC: plugin.Usecase{Repo: &fsrepo.Repository{Dir: dir}}}
	out, err := h.list(context.Background(), nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out.Body) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(out.Body))
	}
	if out.Body[0].Type != "widget" {
		t.Fatalf("expected type widget, got %s", out.Body[0].Type)
	}
}
