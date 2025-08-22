package widgets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOneInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.json")
	os.WriteFile(p, []byte("{"), 0o644)
	if _, err := LoadOne(p); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadOneDefaultScope(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "b.json")
	os.WriteFile(p, []byte(`{"id":"w1","name":"W","type":"widget"}`), 0o644)
	w, err := LoadOne(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.Scopes) != 1 || w.Scopes[0] != "system" {
		t.Fatalf("scope not defaulted: %+v", w.Scopes)
	}
}

func TestLoadAllIgnores(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "c.json"), []byte(`{"id":"w1","name":"W","type":"widget"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "d.json~"), []byte(""), 0o644)
	ws, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(ws) != 1 {
		t.Fatalf("expected 1 widget, got %d", len(ws))
	}
}
