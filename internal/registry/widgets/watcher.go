package widgets

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches a directory of widget JSON files and applies changes to the registry.
type Watcher struct {
	dir      string
	reg      Registry
	debounce time.Duration
	logger   *slog.Logger

	stopOnce sync.Once
	stopFn   context.CancelFunc
	known    map[string]string // path -> id
}

func NewWatcher(dir string, reg Registry, debounce time.Duration, logger *slog.Logger) *Watcher {
	return &Watcher{dir: dir, reg: reg, debounce: debounce, logger: logger, known: map[string]string{}}
}

// Start begins watching. Returns stop function.
func (w *Watcher) Start(ctx context.Context) (func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	w.stopFn = cancel

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fw.Add(w.dir); err != nil {
		return nil, err
	}

	changes := make(chan string, 1024)
	go func() {
		defer fw.Close()
		for {
			select {
			case ev := <-fw.Events:
				// shouldIgnore is defined in loader.go
				if shouldIgnore(ev.Name) || !strings.HasSuffix(ev.Name, ".json") {
					continue
				}
				changes <- ev.Name
			case err := <-fw.Errors:
				if err != nil {
					w.logger.Warn("fsnotify error", "err", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(w.debounce)
		defer ticker.Stop()
		pending := map[string]struct{}{}
		for {
			select {
			case p := <-changes:
				pending[p] = struct{}{}
			case <-ticker.C:
				if len(pending) == 0 {
					continue
				}
				paths := make([]string, 0, len(pending))
				for p := range pending {
					paths = append(paths, p)
				}
				pending = map[string]struct{}{}
				w.applyPaths(ctx, paths)
			case <-ctx.Done():
				return
			}
		}
	}()

	return func() { w.stopOnce.Do(cancel) }, nil
}

func (w *Watcher) applyPaths(ctx context.Context, paths []string) {
	var upserts []Widget
	var removes []string
	for _, p := range paths {
		wi, err := LoadOne(p)
		if errors.Is(err, os.ErrNotExist) {
			if id, ok := w.known[p]; ok {
				removes = append(removes, id)
				delete(w.known, p)
			}
			continue
		}
		if err != nil {
			w.logger.Warn("skip invalid widget json", "path", p, "err", err)
			continue
		}
		upserts = append(upserts, wi)
		w.known[p] = wi.ID
	}
	if len(upserts) == 0 && len(removes) == 0 {
		return
	}
	if _, _, err := w.reg.ApplyDiff(ctx, upserts, removes); err != nil {
		w.logger.Error("apply diff failed", "err", err)
	}
}
