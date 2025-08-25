package widgetpolicy

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestResolve(t *testing.T) {
	p := &WidgetPolicy{
		Rules: []PolicyRule{
			{ID: "email", When: RuleWhen{Validator: []string{"email"}}, Widget: "plugin://email-input", Stop: true},
			{ID: "date", When: RuleWhen{Types: []string{"date"}}, Widget: "plugin://date-input", Stop: true},
			{ID: "fallback", Widget: "plugin://text-input", Stop: true},
		},
	}
	id, _ := p.Resolve(AutoResolveCtx{Type: "date"}, func(string) bool { return true })
	if id != "plugin://date-input" {
		t.Fatalf("expected date-input, got %s", id)
	}
	id, _ = p.Resolve(AutoResolveCtx{Validator: "email"}, func(string) bool { return true })
	if id != "plugin://email-input" {
		t.Fatalf("expected email-input, got %s", id)
	}
	id, _ = p.Resolve(AutoResolveCtx{Type: "unknown"}, func(string) bool { return true })
	if id != "plugin://text-input" {
		t.Fatalf("fallback failed: %s", id)
	}
}

func TestStoreHotReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.yml")
	os.WriteFile(path, []byte("version: 1\nrules:\n- widget: plugin://text-input\n  stop: true\n"), 0644)
	st, err := NewStore(path, testLogger())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := st.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	id, _ := st.Policy().Resolve(AutoResolveCtx{}, func(string) bool { return true })
	if id != "plugin://text-input" {
		t.Fatalf("initial resolve: %s", id)
	}
	os.WriteFile(path, []byte("version: 1\nrules:\n- widget: plugin://textarea\n  stop: true\n"), 0644)
	time.Sleep(300 * time.Millisecond)
	id, _ = st.Policy().Resolve(AutoResolveCtx{}, func(string) bool { return true })
	if id != "plugin://textarea" {
		t.Fatalf("reload failed: %s", id)
	}
}
