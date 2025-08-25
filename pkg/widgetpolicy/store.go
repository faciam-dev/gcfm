package widgetpolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

type Store struct {
	path   string
	cur    atomic.Value // *WidgetPolicy
	logger *slog.Logger
}

func NewStore(path string, logger *slog.Logger) *Store {
	s := &Store{path: path, logger: logger}
	s.cur.Store(&WidgetPolicy{})
	return s
}

func (s *Store) Load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("read policy: %w", err)
	}
	var p WidgetPolicy
	if strings.HasSuffix(strings.ToLower(s.path), ".json") {
		if err := json.Unmarshal(b, &p); err != nil {
			return fmt.Errorf("parse json: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(b, &p); err != nil {
			return fmt.Errorf("parse yaml: %w", err)
		}
	}
	if p.SuggestTop == 0 {
		p.SuggestTop = 6
	}
	p.Normalize()
	s.cur.Store(&p)
	s.logger.Info("widget policy loaded", "path", s.path, "rules", len(p.Rules))
	return nil
}

func (s *Store) Watch(ctx context.Context) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		s.logger.Error("watcher", "err", err)
		return
	}
	defer w.Close()
	_ = w.Add(s.path)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-w.Events:
			if ev.Has(fsnotify.Write) || ev.Has(fsnotify.Create) || ev.Has(fsnotify.Rename) {
				time.Sleep(200 * time.Millisecond)
				if err := s.Load(); err != nil {
					s.logger.Error("reload failed", "err", err)
				}
			}
		case err := <-w.Errors:
			s.logger.Error("watch error", "err", err)
		}
	}
}

func (s *Store) Get() *WidgetPolicy {
	return s.cur.Load().(*WidgetPolicy)
}
