package sdk

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/faciam-dev/gcfm/internal/metrics"
)

// TargetRegistry manages monitored database connections.
type TargetRegistry interface {
	Register(ctx context.Context, key string, cfg TargetConfig, mk Connector) error
	Unregister(key string) error
	Update(ctx context.Context, key string, cfg TargetConfig, mk Connector) error
	ReplaceAll(ctx context.Context, cfgs map[string]TargetConfig, mk Connector, defaultKey string) error
	Snapshot() map[string]TargetConn
	Get(key string) (TargetConn, bool)
	Default() (TargetConn, bool)
	SetDefault(key string)
	Keys() []string
}

// TargetConn represents a monitored database connection.
type TargetConn struct {
	DB     *sql.DB
	Driver string
	Schema string
	Labels map[string]struct{}
}

type snapshot struct {
	byKey      map[string]TargetConn
	defaultKey string
	keys       []string
}

// HotReloadRegistry is an RCU-style implementation of TargetRegistry.
type HotReloadRegistry struct {
	mu     sync.RWMutex
	snap   atomic.Value            // *snapshot
	closer map[string]func() error // key -> close func
}

// NewHotReloadRegistry creates a registry initialized with the default connection.
func NewHotReloadRegistry(defaultConn *TargetConn) *HotReloadRegistry {
	r := &HotReloadRegistry{closer: make(map[string]func() error)}
	s := &snapshot{byKey: make(map[string]TargetConn)}
	if defaultConn != nil {
		s.byKey["__default__"] = *defaultConn
		s.defaultKey = "__default__"
		s.keys = []string{"__default__"}
	}
	r.snap.Store(s)
	r.updateMetrics(s)
	return r
}

// buildConn constructs a TargetConn from TargetConfig using the provided connector.
func (r *HotReloadRegistry) buildConn(ctx context.Context, cfg TargetConfig, mk Connector) (TargetConn, func() error, error) {
	var db *sql.DB
	var err error
	if cfg.DB != nil {
		db = cfg.DB
	} else {
		if mk == nil {
			mk = defaultConnector
		}
		db, err = mk(ctx, cfg.Driver, cfg.DSN)
		if err != nil {
			return TargetConn{}, nil, err
		}
		tune(db, cfg)
		if err := db.PingContext(ctx); err != nil {
			_ = db.Close()
			return TargetConn{}, nil, err
		}
	}
	c := TargetConn{DB: db, Driver: cfg.Driver, Schema: cfg.Schema, Labels: toSet(cfg.Labels)}
	closer := func() error {
		if cfg.DB != nil {
			return nil
		}
		return db.Close()
	}
	return c, closer, nil
}

// Register adds a new target.
func (r *HotReloadRegistry) Register(ctx context.Context, key string, cfg TargetConfig, mk Connector) (err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		metrics.TargetOpLatency.WithLabelValues("register", status).Observe(time.Since(start).Seconds())
	}()

	var conn TargetConn
	var closer func() error
	conn, closer, err = r.buildConn(ctx, cfg, mk)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.snap.Load().(*snapshot)
	if _, exists := old.byKey[key]; exists {
		return errors.New("duplicate key")
	}
	ns := cloneSnap(old)
	ns.byKey[key] = conn
	ns.keys = upsertKey(ns.keys, key)
	r.closer[key] = closer
	r.snap.Store(ns)
	r.updateMetrics(ns)
	return nil
}

// Unregister removes a target. The connection is closed after publishing the snapshot.
func (r *HotReloadRegistry) Unregister(key string) (err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		metrics.TargetOpLatency.WithLabelValues("unregister", status).Observe(time.Since(start).Seconds())
	}()

	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.snap.Load().(*snapshot)
	if _, ok := old.byKey[key]; !ok {
		return nil
	}
	ns := cloneSnap(old)
	delete(ns.byKey, key)
	ns.keys = removeKey(ns.keys, key)
	if ns.defaultKey == key {
		ns.defaultKey = ""
	}
	r.snap.Store(ns)
	r.updateMetrics(ns)
	if cl, ok := r.closer[key]; ok {
		_ = cl()
		delete(r.closer, key)
	}
	return nil
}

// Update replaces an existing target's connection.
func (r *HotReloadRegistry) Update(ctx context.Context, key string, cfg TargetConfig, mk Connector) (err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		metrics.TargetOpLatency.WithLabelValues("update", status).Observe(time.Since(start).Seconds())
	}()

	var conn TargetConn
	var closer func() error
	conn, closer, err = r.buildConn(ctx, cfg, mk)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.snap.Load().(*snapshot)
	if _, ok := old.byKey[key]; !ok {
		return errors.New("unknown key")
	}
	ns := cloneSnap(old)
	ns.byKey[key] = conn
	ns.keys = upsertKey(ns.keys, key)
	oldCloser := r.closer[key]
	r.closer[key] = closer
	r.snap.Store(ns)
	r.updateMetrics(ns)
	if oldCloser != nil {
		_ = oldCloser()
	}
	return nil
}

// ReplaceAll swaps the registry contents atomically.
func (r *HotReloadRegistry) ReplaceAll(ctx context.Context, cfgs map[string]TargetConfig, mk Connector, defaultKey string) (err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		metrics.TargetOpLatency.WithLabelValues("replace_all", status).Observe(time.Since(start).Seconds())
	}()

	nextByKey := make(map[string]TargetConn, len(cfgs))
	nextCloser := make(map[string]func() error, len(cfgs))
	for k, c := range cfgs {
		var conn TargetConn
		var closer func() error
		conn, closer, err = r.buildConn(ctx, c, mk)
		if err != nil {
			for _, cl := range nextCloser {
				_ = cl()
			}
			return err
		}
		nextByKey[k], nextCloser[k] = conn, closer
	}

	r.mu.Lock()
	ns := &snapshot{byKey: nextByKey, defaultKey: defaultKey, keys: keysOf(nextByKey)}
	r.snap.Store(ns)
	r.updateMetrics(ns)
	oldClosers := r.closer
	r.closer = nextCloser
	r.mu.Unlock()

	for k, cl := range oldClosers {
		_ = cl()
		delete(oldClosers, k)
	}
	return nil
}

// SetDefault marks the given key as default if it exists.
func (r *HotReloadRegistry) SetDefault(key string) {
	r.mu.Lock()
	old := r.snap.Load().(*snapshot)
	if key != "" {
		if _, ok := old.byKey[key]; !ok {
			r.mu.Unlock()
			return
		}
	}
	ns := cloneSnap(old)
	ns.defaultKey = key
	r.snap.Store(ns)
	r.mu.Unlock()
}

// Snapshot returns a copy of the current targets.
func (r *HotReloadRegistry) Snapshot() map[string]TargetConn {
	s := r.snap.Load().(*snapshot)
	out := make(map[string]TargetConn, len(s.byKey))
	for k, v := range s.byKey {
		out[k] = v
	}
	return out
}

// Get retrieves a target by key.
func (r *HotReloadRegistry) Get(key string) (TargetConn, bool) {
	s := r.snap.Load().(*snapshot)
	v, ok := s.byKey[key]
	return v, ok
}

// Default returns the default target connection.
func (r *HotReloadRegistry) Default() (TargetConn, bool) {
	s := r.snap.Load().(*snapshot)
	if s.defaultKey == "" {
		return TargetConn{}, false
	}
	v, ok := s.byKey[s.defaultKey]
	return v, ok
}

// Keys returns a copy of available keys.
func (r *HotReloadRegistry) Keys() []string {
	s := r.snap.Load().(*snapshot)
	out := make([]string, len(s.keys))
	copy(out, s.keys)
	return out
}

// Helper functions ---------------------------------------------------------

func tune(db *sql.DB, cfg TargetConfig) {
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxIdle > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdle)
	}
	if cfg.ConnMaxLife > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLife)
	}
}

func toSet(labels []string) map[string]struct{} {
	m := make(map[string]struct{}, len(labels))
	for _, l := range labels {
		m[l] = struct{}{}
	}
	return m
}

func cloneSnap(s *snapshot) *snapshot {
	ns := &snapshot{
		byKey:      make(map[string]TargetConn, len(s.byKey)),
		defaultKey: s.defaultKey,
		keys:       append([]string(nil), s.keys...),
	}
	for k, v := range s.byKey {
		ns.byKey[k] = v
	}
	return ns
}

func upsertKey(keys []string, key string) []string {
	for _, k := range keys {
		if k == key {
			return keys
		}
	}
	return append(keys, key)
}

func removeKey(keys []string, key string) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if k != key {
			out = append(out, k)
		}
	}
	return out
}

func keysOf(m map[string]TargetConn) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// defaultConnector is the built-in connector using database/sql.
func defaultConnector(ctx context.Context, driver, dsnOrURL string) (*sql.DB, error) {
	return sql.Open(driver, dsnOrURL)
}

func (r *HotReloadRegistry) updateMetrics(s *snapshot) {
	metrics.Targets.Set(float64(len(s.keys)))
	metrics.TargetLabels.Reset()
	counts := make(map[string]int)
	for _, c := range s.byKey {
		for l := range c.Labels {
			counts[l]++
		}
	}
	for l, n := range counts {
		metrics.TargetLabels.WithLabelValues(l).Set(float64(n))
	}
}
