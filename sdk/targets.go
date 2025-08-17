package sdk

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/faciam-dev/gcfm/internal/metrics"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
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
	FindByLabel(label string) []string
	FindAllByLabels(labels ...string) []string
	FindAnyByLabels(labels ...string) []string
	FindByQuery(q Query) []string
	ForEachByQuery(q Query, fn func(key string, t TargetConn) error) error
}

// TargetConn represents a monitored database connection.
type TargetConn struct {
	DB      *sql.DB
	Driver  string
	Schema  string
	Dialect ormdriver.Dialect
	Labels  map[string]struct{}
}

type snapshot struct {
	byKey      map[string]TargetConn
	defaultKey string
	keys       []string
	labelIndex map[string]map[string]struct{}
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
	}
	keys, idx := keysOf(s.byKey)
	s.keys = keys
	s.labelIndex = idx
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
	}
	c := TargetConn{DB: db, Driver: cfg.Driver, Schema: cfg.Schema, Dialect: driverDialect(cfg.Driver), Labels: toSet(cfg.Labels)}
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
	if cfg.DB == nil {
		if err = conn.DB.PingContext(ctx); err != nil {
			_ = closer()
			return err
		}
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
	addToIndex(ns.labelIndex, key, conn.Labels)
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
	removeFromIndex(ns.labelIndex, key, old.byKey[key].Labels)
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
	if cfg.DB == nil {
		if err = conn.DB.PingContext(ctx); err != nil {
			_ = closer()
			return err
		}
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
	updateIndex(ns.labelIndex, key, old.byKey[key].Labels, conn.Labels)
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

	var wg sync.WaitGroup
	errCh := make(chan error, len(nextByKey))
	for k, c := range nextByKey {
		if cfgs[k].DB != nil {
			continue
		}
		wg.Add(1)
		go func(db *sql.DB) {
			defer wg.Done()
			if err := db.PingContext(ctx); err != nil {
				errCh <- err
			}
		}(c.DB)
	}
	wg.Wait()
	close(errCh)
	var pingErr error
	for e := range errCh {
		pingErr = e
		break
	}
	if pingErr != nil {
		for _, cl := range nextCloser {
			_ = cl()
		}
		return pingErr
	}

	r.mu.Lock()
	keys, idx := keysOf(nextByKey)
	ns := &snapshot{byKey: nextByKey, defaultKey: defaultKey, keys: keys, labelIndex: idx}
	r.snap.Store(ns)
	r.updateMetrics(ns)
	oldClosers := r.closer
	r.closer = nextCloser
	r.mu.Unlock()

	for _, cl := range oldClosers {
		_ = cl()
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

// DefaultKey returns the current default key.
func (r *HotReloadRegistry) DefaultKey() string {
	s := r.snap.Load().(*snapshot)
	return s.defaultKey
}

// Keys returns a copy of available keys.
func (r *HotReloadRegistry) Keys() []string {
	s := r.snap.Load().(*snapshot)
	out := make([]string, len(s.keys))
	copy(out, s.keys)
	return out
}

// FindByLabel returns keys of targets having the given label.
func (r *HotReloadRegistry) FindByLabel(label string) []string {
	label = strings.ToLower(label)
	start := time.Now()
	s := r.snap.Load().(*snapshot)
	ks, ok := s.labelIndex[label]
	var out []string
	if ok {
		out = make([]string, 0, len(ks))
		for k := range ks {
			out = append(out, k)
		}
		sort.Strings(out)
	}
	dur := time.Since(start).Seconds()
	metrics.TargetQueryLatency.WithLabelValues("label").Observe(dur)
	metrics.TargetQueryHits.WithLabelValues("label").Observe(float64(len(out)))
	return out
}

// FindAllByLabels returns keys of targets containing all the specified labels.
func (r *HotReloadRegistry) FindAllByLabels(labels ...string) []string {
	start := time.Now()
	s := r.snap.Load().(*snapshot)
	if len(labels) == 0 {
		metrics.TargetQueryLatency.WithLabelValues("all").Observe(time.Since(start).Seconds())
		metrics.TargetQueryHits.WithLabelValues("all").Observe(0)
		return nil
	}
	sets := make([]map[string]struct{}, 0, len(labels))
	for _, l := range labels {
		l = strings.ToLower(l)
		ks, ok := s.labelIndex[l]
		if !ok {
			metrics.TargetQueryLatency.WithLabelValues("all").Observe(time.Since(start).Seconds())
			metrics.TargetQueryHits.WithLabelValues("all").Observe(0)
			return nil
		}
		sets = append(sets, ks)
	}
	hits := intersectMany(sets...)
	if hits == nil {
		metrics.TargetQueryLatency.WithLabelValues("all").Observe(time.Since(start).Seconds())
		metrics.TargetQueryHits.WithLabelValues("all").Observe(0)
		return nil
	}
	out := make([]string, 0, len(hits))
	for k := range hits {
		out = append(out, k)
	}
	sort.Strings(out)
	dur := time.Since(start).Seconds()
	metrics.TargetQueryLatency.WithLabelValues("all").Observe(dur)
	metrics.TargetQueryHits.WithLabelValues("all").Observe(float64(len(out)))
	return out
}

// FindAnyByLabels returns keys of targets containing any of the specified labels.
func (r *HotReloadRegistry) FindAnyByLabels(labels ...string) []string {
	start := time.Now()
	s := r.snap.Load().(*snapshot)
	sets := make([]map[string]struct{}, 0, len(labels))
	for _, l := range labels {
		l = strings.ToLower(l)
		if ks, ok := s.labelIndex[l]; ok {
			sets = append(sets, ks)
		}
	}
	hits := unionMany(sets...)
	if hits == nil {
		metrics.TargetQueryLatency.WithLabelValues("any").Observe(time.Since(start).Seconds())
		metrics.TargetQueryHits.WithLabelValues("any").Observe(0)
		return nil
	}
	out := make([]string, 0, len(hits))
	for k := range hits {
		out = append(out, k)
	}
	sort.Strings(out)
	dur := time.Since(start).Seconds()
	metrics.TargetQueryLatency.WithLabelValues("any").Observe(dur)
	metrics.TargetQueryHits.WithLabelValues("any").Observe(float64(len(out)))
	return out
}

// FindByQuery returns keys of targets matching q.
func (r *HotReloadRegistry) FindByQuery(q Query) []string {
	start := time.Now()
	s := r.snap.Load().(*snapshot)
	hits := s.filter(q)
	if hits == nil {
		metrics.TargetQueryLatency.WithLabelValues("query").Observe(time.Since(start).Seconds())
		metrics.TargetQueryHits.WithLabelValues("query").Observe(0)
		return nil
	}
	out := make([]string, 0, len(hits))
	for k := range hits {
		out = append(out, k)
	}
	sort.Strings(out)
	dur := time.Since(start).Seconds()
	metrics.TargetQueryLatency.WithLabelValues("query").Observe(dur)
	metrics.TargetQueryHits.WithLabelValues("query").Observe(float64(len(out)))
	return out
}

// ForEachByQuery executes fn for each target matching q.
func (r *HotReloadRegistry) ForEachByQuery(q Query, fn func(key string, t TargetConn) error) error {
	start := time.Now()
	s := r.snap.Load().(*snapshot)
	hits := s.filter(q)
	if hits == nil {
		metrics.TargetQueryLatency.WithLabelValues("foreach").Observe(time.Since(start).Seconds())
		metrics.TargetQueryHits.WithLabelValues("foreach").Observe(0)
		return nil
	}
	keys := make([]string, 0, len(hits))
	for k := range hits {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if c, ok := s.byKey[k]; ok {
			if err := fn(k, c); err != nil {
				return err
			}
		}
	}
	dur := time.Since(start).Seconds()
	metrics.TargetQueryLatency.WithLabelValues("foreach").Observe(dur)
	metrics.TargetQueryHits.WithLabelValues("foreach").Observe(float64(len(keys)))
	return nil
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
	s := make(map[string]struct{}, len(labels))
	for _, l := range labels {
		l = strings.ToLower(strings.TrimSpace(l))
		if l == "" {
			continue
		}
		s[l] = struct{}{}
		if i := strings.IndexByte(l, '='); i >= 0 {
			s[l[:i]] = struct{}{}
		}
	}
	return s
}

func deepCopyLabelIndex(src map[string]map[string]struct{}) map[string]map[string]struct{} {
	dst := make(map[string]map[string]struct{}, len(src))
	for label, keys := range src {
		keysCopy := make(map[string]struct{}, len(keys))
		for k := range keys {
			keysCopy[k] = struct{}{}
		}
		dst[label] = keysCopy
	}
	return dst
}

func cloneSnap(s *snapshot) *snapshot {
	ns := &snapshot{
		byKey:      make(map[string]TargetConn, len(s.byKey)),
		defaultKey: s.defaultKey,
		keys:       append([]string(nil), s.keys...),
		labelIndex: deepCopyLabelIndex(s.labelIndex),
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

func keysOf(m map[string]TargetConn) ([]string, map[string]map[string]struct{}) {
	keys := make([]string, 0, len(m))
	idx := make(map[string]map[string]struct{})
	for k, v := range m {
		keys = append(keys, k)
		addToIndex(idx, k, v.Labels)
	}
	return keys, idx
}

func addToIndex(idx map[string]map[string]struct{}, key string, labels map[string]struct{}) {
	for l := range labels {
		s, ok := idx[l]
		if !ok {
			s = make(map[string]struct{})
			idx[l] = s
		}
		s[key] = struct{}{}
	}
}

func removeFromIndex(idx map[string]map[string]struct{}, key string, labels map[string]struct{}) {
	for l := range labels {
		if s, ok := idx[l]; ok {
			delete(s, key)
			if len(s) == 0 {
				delete(idx, l)
			}
		}
	}
}

func driverDialect(d string) ormdriver.Dialect {
	switch d {
	case "postgres":
		return ormdriver.PostgresDialect{}
	case "mysql":
		return ormdriver.MySQLDialect{}
	default:
		return nil
	}
}

func updateIndex(idx map[string]map[string]struct{}, key string, oldLabels, newLabels map[string]struct{}) {
	for l := range oldLabels {
		if _, ok := newLabels[l]; !ok {
			if s, ok := idx[l]; ok {
				delete(s, key)
				if len(s) == 0 {
					delete(idx, l)
				}
			}
		}
	}
	for l := range newLabels {
		if _, ok := oldLabels[l]; !ok {
			s, ok := idx[l]
			if !ok {
				s = make(map[string]struct{})
				idx[l] = s
			}
			s[key] = struct{}{}
		}
	}
}

func intersectMany(sets ...map[string]struct{}) map[string]struct{} {
	if len(sets) == 0 {
		return nil
	}
	baseIdx := -1
	for i, s := range sets {
		if s == nil {
			return nil
		}
		if baseIdx == -1 || len(s) < len(sets[baseIdx]) {
			baseIdx = i
		}
	}
	res := make(map[string]struct{}, len(sets[baseIdx]))
	for k := range sets[baseIdx] {
		res[k] = struct{}{}
	}
	for i, s := range sets {
		if i == baseIdx {
			continue
		}
		for k := range res {
			if _, ok := s[k]; !ok {
				delete(res, k)
			}
		}
		if len(res) == 0 {
			return nil
		}
	}
	return res
}

func unionMany(sets ...map[string]struct{}) map[string]struct{} {
	res := make(map[string]struct{})
	for _, s := range sets {
		for k := range s {
			res[k] = struct{}{}
		}
	}
	if len(res) == 0 {
		return nil
	}
	return res
}

func diffSet(all, exclude map[string]struct{}) map[string]struct{} {
	if all == nil {
		return nil
	}
	res := make(map[string]struct{}, len(all))
	for k := range all {
		if _, ok := exclude[k]; !ok {
			res[k] = struct{}{}
		}
	}
	if len(res) == 0 {
		return nil
	}
	return res
}

func (s *snapshot) evalAnd(exprs []LabelExpr) map[string]struct{} {
	positives := make([]map[string]struct{}, 0, len(exprs))
	negatives := make([]map[string]struct{}, 0)
	for _, e := range exprs {
		switch ex := e.(type) {
		case EqExpr:
			positives = append(positives, s.labelIndex[ex.Label+"="+ex.Value])
		case HasExpr:
			positives = append(positives, s.labelIndex[ex.Label])
		case InExpr:
			inner := make([]map[string]struct{}, len(ex.Values))
			for i, v := range ex.Values {
				inner[i] = s.labelIndex[ex.Label+"="+v]
			}
			positives = append(positives, unionMany(inner...))
		case NotExpr:
			negatives = append(negatives, s.labelIndex[ex.Label])
		}
	}
	res := intersectMany(positives...)
	if len(positives) > 0 && res == nil {
		return nil
	}
	if len(positives) == 0 {
		res = make(map[string]struct{}, len(s.keys))
		for _, k := range s.keys {
			res[k] = struct{}{}
		}
	}
	if len(negatives) > 0 {
		res = diffSet(res, unionMany(negatives...))
	}
	return res
}

func (s *snapshot) filter(q Query) map[string]struct{} {
	if len(q.OR) > 0 {
		groups := make([]map[string]struct{}, 0, len(q.OR))
		for _, grp := range q.OR {
			groups = append(groups, s.evalAnd(grp))
		}
		return unionMany(groups...)
	}
	return s.evalAnd(q.AND)
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
