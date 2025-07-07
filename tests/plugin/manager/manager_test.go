package manager_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/faciam-dev/gcfm/internal/plugin"
)

var repoRoot string

func init() {
	_, file, _, _ := runtime.Caller(0)
	repoRoot = filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
}

func buildPlugin(t *testing.T, dir string) string {
	t.Helper()
	so := filepath.Join(dir, "p.so")
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", so, filepath.Join(repoRoot, "tests", "plugin", "manager", "testplugin"))
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build plugin: %v\n%s", err, out)
	}
	return so
}

func TestManagerLoad(t *testing.T) {
	if testing.CoverMode() != "" {
		t.Skip("plugin build incompatible with coverage")
	}
	dir := t.TempDir()
	so := buildPlugin(t, dir)
	m := plugin.New()
	if err := m.Load(so); err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, ok := m.Validator("email"); !ok {
		t.Fatalf("validator not found")
	}
	if _, ok := m.Widget("dummy"); !ok {
		t.Fatalf("widget not found")
	}
}
