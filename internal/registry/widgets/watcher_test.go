package widgets

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"log/slog"
)

func TestWatcherUpsertRemove(t *testing.T) {
	dir := t.TempDir()
	reg := NewInMemory()
	w := NewWatcher(dir, reg, 20*time.Millisecond, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop, err := w.Start(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer stop()
	ch, unsub := reg.Subscribe()
	defer unsub()

	// create widget file
	p := filepath.Join(dir, "a.json")
	os.WriteFile(p, []byte(`{"id":"a","name":"A","type":"widget"}`), 0o644)
	select {
	case ev := <-ch:
		if ev.Type != "upsert" || ev.Item.ID != "a" {
			t.Fatalf("unexpected event %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatalf("no upsert event")
	}

	os.Remove(p)
	select {
	case ev := <-ch:
		if ev.Type != "remove" || ev.ID != "a" {
			t.Fatalf("unexpected remove %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatalf("no remove event")
	}
}
