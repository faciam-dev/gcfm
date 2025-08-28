package widgets

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
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
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = filepath.Clean(dir)
	}
	return &Watcher{dir: absDir, reg: reg, debounce: debounce, logger: logger, known: map[string]string{}}
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
		cleaned := filepath.Clean(p)
		abs := cleaned
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(w.dir, cleaned)
		}
		dir := filepath.Dir(abs)
		resolvedDir, err := filepath.EvalSymlinks(dir)
		if err != nil {
			w.logger.Warn("skip invalid path", "path", p, "err", err)
			continue
		}
		resolved := filepath.Join(resolvedDir, filepath.Base(abs))
		rel, err := filepath.Rel(w.dir, resolved)
		if err != nil || strings.HasPrefix(rel, "..") {
			w.logger.Warn("skip outside widget dir", "path", resolved)
			continue
		}
		if _, err := os.Stat(resolved); errors.Is(err, os.ErrNotExist) {
			if id, ok := w.known[resolved]; ok {
				removes = append(removes, id)
				delete(w.known, resolved)
			}
			continue
		} else if err != nil {
			w.logger.Warn("skip invalid path", "path", resolved, "err", err)
			continue
		}
		wi, err := LoadOne(resolved)
		if err != nil {
			w.logger.Warn("skip invalid widget json", "path", resolved, "err", err)
			continue
		}
		upserts = append(upserts, wi)
		w.known[resolved] = wi.ID
	}
	if len(upserts) == 0 && len(removes) == 0 {
		return
	}
	if _, _, err := w.reg.ApplyDiff(ctx, upserts, removes); err != nil {
		w.logger.Error("apply diff failed", "err", err)
	}
}
