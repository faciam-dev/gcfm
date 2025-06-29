package pluginloader_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"go.uber.org/zap/zaptest"

	"github.com/faciam-dev/gcfm/internal/customfield"
	"github.com/faciam-dev/gcfm/internal/customfield/pluginloader"
)

var repoRoot string

func init() {
	_, file, _, _ := runtime.Caller(0)
	repoRoot = filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
}

func buildSample(t *testing.T, src, dir, name string) string {
	t.Helper()
	so := filepath.Join(dir, name)
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", so, filepath.Join(repoRoot, src))
	cmd.Env = os.Environ()
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build plugin: %v\n%s", err, out)
	}
	return so
}

func TestLoadAll(t *testing.T) {
	base := filepath.Join(repoRoot, "tests", "runtime", "pluginloader", t.Name())
	os.RemoveAll(base)
	os.MkdirAll(base, 0755)
	defer os.RemoveAll(base)
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", base)
	defer os.Setenv("HOME", originalHome)
	dir := pluginloader.DefaultDir()
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)
	buildSample(t, "sample/validator_uppercase", dir, "a.so")
	logger := zaptest.NewLogger(t).Sugar()
	if err := pluginloader.LoadAll(logger); err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, ok := customfield.GetValidator("uppercase"); !ok {
		t.Fatalf("validator not registered")
	}
	// calling LoadAll again should not fail even though the validator is already registered
	if err := pluginloader.LoadAll(logger); err != nil {
		t.Fatalf("reload: %v", err)
	}
}
