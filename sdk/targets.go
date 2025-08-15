package sdk

import (
	"database/sql"
	"errors"
	"sync"
)

// TargetRegistry manages monitored database connections.
type TargetRegistry interface {
	Register(cfg TargetConfig) error
	Get(key string) (TargetConn, bool)
	Default() (TargetConn, bool)
	Keys() []string
	ForEach(func(key string, t TargetConn) error) error
}

// TargetConn represents a monitored database connection.
type TargetConn struct {
	DB     *sql.DB
	Driver string
	Schema string
	Labels map[string]struct{}
}

// isValidTargetConn returns true if any field in TargetConn is set.
func isValidTargetConn(conn TargetConn) bool {
	return conn.DB != nil || conn.Driver != "" || conn.Schema != "" || len(conn.Labels) > 0
}

// targetRegistry is an in-memory implementation of TargetRegistry.
type targetRegistry struct {
	mu         sync.RWMutex
	targets    map[string]TargetConn
	defaultKey string
}

// NewTargetRegistry creates a registry initialized with the default connection.
func NewTargetRegistry(defaultConn TargetConn) TargetRegistry {
	r := &targetRegistry{targets: make(map[string]TargetConn)}
	if isValidTargetConn(defaultConn) {
		r.targets["default"] = defaultConn
		r.defaultKey = "default"
	}
	return r
}

func (r *targetRegistry) Register(cfg TargetConfig) error {
	if cfg.Key == "" {
		return errors.New("key required")
	}
	if cfg.DB == nil {
		return errors.New("DB required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.targets[cfg.Key]; exists {
		return errors.New("duplicate key")
	}
	labels := make(map[string]struct{}, len(cfg.Labels))
	for _, l := range cfg.Labels {
		labels[l] = struct{}{}
	}
	r.targets[cfg.Key] = TargetConn{DB: cfg.DB, Driver: cfg.Driver, Schema: cfg.Schema, Labels: labels}
	return nil
}

func (r *targetRegistry) Get(key string) (TargetConn, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.targets[key]
	return t, ok
}

func (r *targetRegistry) Default() (TargetConn, bool) {
	if r.defaultKey == "" {
		return TargetConn{}, false
	}
	return r.Get(r.defaultKey)
}

func (r *targetRegistry) Keys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	keys := make([]string, 0, len(r.targets))
	for k := range r.targets {
		keys = append(keys, k)
	}
	return keys
}

func (r *targetRegistry) ForEach(fn func(key string, t TargetConn) error) error {
	r.mu.RLock()
	d := make(map[string]TargetConn, len(r.targets))
	for k, v := range r.targets {
		d[k] = v
	}
	r.mu.RUnlock()
	for k, v := range d {
		if err := fn(k, v); err != nil {
			return err
		}
	}
	return nil
}
