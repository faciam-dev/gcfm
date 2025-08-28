package widgets

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LoadAll reads all widget JSON files within dir.
func LoadAll(dir string) ([]Widget, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Widget
	ids := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if shouldIgnore(name) || !strings.HasSuffix(name, ".json") {
			continue
		}
		w, err := LoadOne(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		if prev, ok := ids[w.ID]; ok {
			return nil, fmt.Errorf("duplicate widget id %s in %s and %s", w.ID, prev, name)
		}
		ids[w.ID] = name
		out = append(out, w)
	}
	return out, nil
}

// LoadOne reads a single widget JSON file.
func LoadOne(path string) (Widget, error) {
	p := filepath.Clean(path)
	b, err := os.ReadFile(p) // #nosec G304 -- path derived from directory listing
	if err != nil {
		return Widget{}, err
	}
	var w Widget
	if err := json.Unmarshal(b, &w); err != nil {
		return Widget{}, err
	}
	if w.ID == "" || w.Name == "" || w.Type != "widget" {
		return Widget{}, errors.New("invalid widget: id, name, type required")
	}
	if len(w.Scopes) == 0 {
		w.Scopes = []string{"system"}
	}
	w.UpdatedAt = time.Now().UTC()
	return w, nil
}

func shouldIgnore(name string) bool {
	base := filepath.Base(name)
	if strings.HasPrefix(base, ".") {
		return true
	}
	switch {
	case strings.HasSuffix(base, "~"),
		strings.HasSuffix(base, ".swp"),
		strings.HasSuffix(base, ".swx"),
		strings.HasSuffix(base, ".tmp"),
		strings.HasSuffix(base, ".partial"),
		strings.HasSuffix(base, "4913"),
		strings.HasPrefix(base, "#") && strings.HasSuffix(base, "#"):
		return true
	}
	return false
}
