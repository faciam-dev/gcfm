package sdk

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/faciam-dev/gcfm/internal/metrics"
)

// FailoverPolicy controls retry and circuit breaker behavior.
type FailoverPolicy struct {
	Enabled           bool
	MaxAttempts       int
	BaseBackoff       time.Duration
	MaxBackoff        time.Duration
	JitterRatio       float64
	OpenAfterFailures int
	OpenDuration      time.Duration
	HalfOpenProbe     int
	PreferOnFail      *SelectionHint
	AllowWriteRetry   bool
}

// ErrorClassifier determines whether an error is transient and retryable.
type ErrorClassifier func(error) (transient bool, retryable bool)

// DefaultErrorClassifier provides baseline classification for common errors.
func DefaultErrorClassifier(err error) (bool, bool) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true, true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true, true
	}
	if errors.Is(err, driver.ErrBadConn) {
		return true, true
	}
	return false, false
}

// breakerState represents circuit breaker states.
type breakerState int

const (
	stateClosed breakerState = iota
	stateOpen
	stateHalfOpen
)

type targetHealth struct {
	mu        sync.Mutex
	state     breakerState
	failures  int
	openUntil time.Time
}

type healthRegistry struct {
	mu  sync.RWMutex
	m   map[string]*targetHealth
	pol FailoverPolicy
}

func newHealthRegistry(pol FailoverPolicy) *healthRegistry {
	return &healthRegistry{m: make(map[string]*targetHealth), pol: pol}
}

func (r *healthRegistry) get(key string) *targetHealth {
	r.mu.RLock()
	h, ok := r.m[key]
	r.mu.RUnlock()
	if ok {
		return h
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok = r.m[key]; ok {
		return h
	}
	h = &targetHealth{}
	r.m[key] = h
	return h
}

func (r *healthRegistry) onSuccess(key string) {
	h := r.get(key)
	h.mu.Lock()
	h.state = stateClosed
	h.failures = 0
	h.mu.Unlock()
	r.setStateMetric(key, stateClosed)
}
func (r *healthRegistry) onFailure(key string, transient bool) {
	metrics.TargetFailures.WithLabelValues(key, strconv.FormatBool(transient)).Inc()
	h := r.get(key)
	h.mu.Lock()
	defer func() {
		st := h.state
		h.mu.Unlock()
		r.setStateMetric(key, st)
	}()
	if h.state == stateHalfOpen {
		h.state = stateOpen
		h.openUntil = time.Now().Add(r.pol.OpenDuration)
		h.failures = 0
		return
	}
	if transient {
		h.failures++
		if r.pol.OpenAfterFailures > 0 && h.failures >= r.pol.OpenAfterFailures {
			h.state = stateOpen
			h.openUntil = time.Now().Add(r.pol.OpenDuration)
			h.failures = 0
		}
		return
	}
	h.state = stateOpen
	h.openUntil = time.Now().Add(r.pol.OpenDuration)
	h.failures = 0
}

func (r *healthRegistry) isAvailable(key string) bool {
	h := r.get(key)
	h.mu.Lock()
	available := true
	if h.state == stateOpen {
		if time.Now().Before(h.openUntil) {
			available = false
		} else {
			h.state = stateHalfOpen
		}
	} else if h.state == stateHalfOpen {
		available = false
	}
	st := h.state
	h.mu.Unlock()
	r.setStateMetric(key, st)
	return available
}

func (r *healthRegistry) prune(keys []string) {
	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		set[k] = struct{}{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for k := range r.m {
		if _, ok := set[k]; !ok {
			delete(r.m, k)
		}
	}
}

func (r *healthRegistry) setStateMetric(key string, st breakerState) {
	metrics.TargetState.WithLabelValues(key, "closed").Set(boolToFloat(st == stateClosed))
	metrics.TargetState.WithLabelValues(key, "open").Set(boolToFloat(st == stateOpen))
	metrics.TargetState.WithLabelValues(key, "half_open").Set(boolToFloat(st == stateHalfOpen))
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (p FailoverPolicy) backoff(attempt int) time.Duration {
	if attempt <= 0 || p.BaseBackoff <= 0 {
		return 0
	}
	d := p.BaseBackoff
	for i := 1; i < attempt; i++ {
		d *= 2
		if p.MaxBackoff > 0 && d > p.MaxBackoff {
			d = p.MaxBackoff
			break
		}
	}
	if p.JitterRatio > 0 {
		jitter := time.Duration(rand.Float64() * p.JitterRatio * float64(d))
		d += jitter
	}
	return d
}

func (s *service) RunWithTarget(ctx context.Context, dec TargetDecision, isWrite bool, fn func(TargetConn) error) error {
	if !s.failover.Enabled {
		var key string
		if dec.Key != "" {
			key = dec.Key
		} else if dec.Query != nil {
			keys := s.targets.FindByQuery(*dec.Query)
			if k, ok := s.chooseOne(keys, dec.Hint); ok {
				key = k
			}
		} else {
			if t, ok := s.targets.Default(); ok {
				return fn(t)
			}
			return ErrNoTarget
		}
		if key != "" {
			if t, ok := s.targets.Get(key); ok {
				return fn(t)
			}
		}
		return ErrNoTarget
	}

	var keys []string
	if dec.Query != nil {
		keys = s.targets.FindByQuery(*dec.Query)
	}
	ordered := s.orderCandidates(keys, dec.Key, s.failoverPrefer(dec))
	if len(ordered) == 0 && dec.Key != "" {
		ordered = []string{dec.Key}
	}

	attempts := 0
	maxAttempts := s.failover.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	for i := 0; i < len(ordered) && attempts < maxAttempts; i++ {
		key := ordered[i]
		if !s.health.isAvailable(key) {
			continue
		}
		tgt, ok := s.targets.Get(key)
		if !ok {
			continue
		}
		attempts++
		err := fn(tgt)
		if err == nil {
			s.health.onSuccess(key)
			return nil
		}
		transient, retryable := s.classify(err)
		s.health.onFailure(key, transient)
		if !retryable || (isWrite && !s.failover.AllowWriteRetry) {
			return err
		}
		if d := s.failover.backoff(attempts); d > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(d):
			}
		}
	}
	return fmt.Errorf("all candidates failed after %d attempts", attempts)
}

func (s *service) orderCandidates(keys []string, primary string, prefer *SelectionHint) []string {
	set := map[string]struct{}{}
	out := make([]string, 0, len(keys)+1)
	if primary != "" {
		out = append(out, primary)
		set[primary] = struct{}{}
	}
	if prefer != nil && prefer.Strategy == SelectPreferLabel && prefer.PreferLabel != "" {
		pref := s.targets.FindByLabel(prefer.PreferLabel)
		for _, k := range pref {
			if _, ok := set[k]; !ok {
				out = append(out, k)
				set[k] = struct{}{}
			}
		}
	}
	for _, k := range keys {
		if _, ok := set[k]; !ok {
			out = append(out, k)
			set[k] = struct{}{}
		}
	}
	return out
}

func (s *service) failoverPrefer(dec TargetDecision) *SelectionHint {
	if dec.Hint != nil {
		return dec.Hint
	}
	if s.failover.PreferOnFail != nil {
		return s.failover.PreferOnFail
	}
	return nil
}
