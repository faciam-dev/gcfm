package widgets

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Widget struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	Type         string         `json:"type"`
	Scopes       []string       `json:"scopes"`
	Enabled      bool           `json:"enabled"`
	Description  string         `json:"description,omitempty"`
	Capabilities []string       `json:"capabilities,omitempty"`
	Homepage     string         `json:"homepage,omitempty"`
	UpdatedAt    time.Time      `json:"updated_at"`
	Meta         map[string]any `json:"meta,omitempty"`
	Tenants      []string       `json:"-"`
}

type Event struct {
	Type string
	Item *Widget
	ID   string
}

type Options struct {
	Scope  []string
	Tenant string
	Q      string
	Limit  int
	Offset int
}

type Registry interface {
	List(ctx context.Context, opt Options) ([]Widget, int, string, time.Time, error)
	Upsert(ctx context.Context, w Widget) error
	Remove(ctx context.Context, id string) error
	Subscribe() (<-chan Event, func())
}

type inMemory struct {
	mu      sync.RWMutex
	items   map[string]Widget
	subs    map[chan Event]struct{}
	lastMod time.Time
	version uint64
}

func NewInMemory() Registry {
	return &inMemory{
		items: make(map[string]Widget),
		subs:  make(map[chan Event]struct{}),
	}
}

func (r *inMemory) List(ctx context.Context, opt Options) ([]Widget, int, string, time.Time, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []Widget
	for _, w := range r.items {
		if len(opt.Scope) > 0 {
			matched := false
			for _, s := range opt.Scope {
				if contains(w.Scopes, s) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if opt.Tenant != "" && contains(w.Scopes, "tenant") {
			if len(w.Tenants) > 0 && !contains(w.Tenants, opt.Tenant) {
				continue
			}
		}
		if opt.Q != "" {
			q := strings.ToLower(opt.Q)
			if !strings.Contains(strings.ToLower(w.ID), q) &&
				!strings.Contains(strings.ToLower(w.Name), q) &&
				!strings.Contains(strings.ToLower(w.Description), q) {
				continue
			}
		}
		filtered = append(filtered, w)
	}

	sort.Slice(filtered, func(i, j int) bool { return filtered[i].ID < filtered[j].ID })
	total := len(filtered)

	if opt.Offset < 0 {
		opt.Offset = 0
	}
	if opt.Limit <= 0 {
		opt.Limit = 50
	}
	if opt.Limit > 200 {
		opt.Limit = 200
	}
	start := opt.Offset
	if start > total {
		start = total
	}
	end := start + opt.Limit
	if end > total {
		end = total
	}
	items := append([]Widget{}, filtered[start:end]...)

	etag := fmt.Sprintf("\"%d\"", r.version)
	return items, total, etag, r.lastMod, nil
}

func (r *inMemory) Upsert(ctx context.Context, w Widget) error {
	if w.UpdatedAt.IsZero() {
		w.UpdatedAt = time.Now().UTC()
	}
	r.mu.Lock()
	r.items[w.ID] = w
	r.version++
	r.lastMod = time.Now().UTC()
	subs := cloneSubs(r.subs)
	r.mu.Unlock()
	broadcast(subs, Event{Type: "upsert", Item: &w})
	return nil
}

func (r *inMemory) Remove(ctx context.Context, id string) error {
	r.mu.Lock()
	delete(r.items, id)
	r.version++
	r.lastMod = time.Now().UTC()
	subs := cloneSubs(r.subs)
	r.mu.Unlock()
	broadcast(subs, Event{Type: "remove", ID: id})
	return nil
}

func (r *inMemory) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 16) // allow brief slowdowns without dropping events
	r.mu.Lock()
	r.subs[ch] = struct{}{}
	r.mu.Unlock()
	return ch, func() {
		r.mu.Lock()
		delete(r.subs, ch)
		r.mu.Unlock()
	}
}

func broadcast(subs map[chan Event]struct{}, ev Event) {
	for ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func cloneSubs(m map[chan Event]struct{}) map[chan Event]struct{} {
	out := make(map[chan Event]struct{}, len(m))
	for k := range m {
		out[k] = struct{}{}
	}
	return out
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
