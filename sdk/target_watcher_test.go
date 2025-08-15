package sdk

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	metapkg "github.com/faciam-dev/gcfm/meta"
	_ "github.com/mattn/go-sqlite3"
)

// countingRegistry wraps HotReloadRegistry to count ReplaceAll calls.
type countingRegistry struct {
	*HotReloadRegistry
	mu    sync.Mutex
	count int
}

func newCountingRegistry() *countingRegistry {
	return &countingRegistry{HotReloadRegistry: NewHotReloadRegistry(nil)}
}

func (r *countingRegistry) ReplaceAll(ctx context.Context, cfgs map[string]TargetConfig, mk Connector, def string) error {
	r.mu.Lock()
	r.count++
	r.mu.Unlock()
	return r.HotReloadRegistry.ReplaceAll(ctx, cfgs, mk, def)
}

func (r *countingRegistry) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

func waitReplace(r *countingRegistry, want int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r.calls() >= want {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func TestMetaDBProviderWatcher(t *testing.T) {
	ctx := context.Background()
	store := newTargetStore(t)

	if err := store.UpsertTarget(ctx, nil, metapkg.TargetRow{Key: "tenant:A", Driver: "sqlite3", DSN: "dsnA"}, []string{"region=tokyo"}); err != nil {
		t.Fatalf("upsert A: %v", err)
	}
	if err := store.SetDefaultTarget(ctx, nil, "tenant:A"); err != nil {
		t.Fatalf("set default A: %v", err)
	}
	if _, err := store.BumpTargetsVersion(ctx, nil); err != nil {
		t.Fatalf("bump1: %v", err)
	}

	var (
		mu   sync.Mutex
		dsns []string
	)
	connector := func(ctx context.Context, driver, dsn string) (*sql.DB, error) {
		mu.Lock()
		dsns = append(dsns, dsn)
		mu.Unlock()
		return sql.Open("sqlite3", ":memory:")
	}

	reg := newCountingRegistry()
	svc := &service{targets: reg, logger: zap.NewNop().Sugar(), cn: connector}
	stop := svc.StartTargetWatcher(ctx, NewMetaDBProvider(store), 10*time.Millisecond)
	defer stop()

	if !waitReplace(reg, 1, time.Second) {
		t.Fatalf("initial replace not observed")
	}

	rowB := metapkg.TargetRow{
		Key:          "tenant:B",
		Driver:       "sqlite3",
		DSN:          "dsnB",
		MaxOpenConns: 5,
		MaxIdleConns: 3,
		ConnMaxIdle:  100 * time.Millisecond,
		ConnMaxLife:  200 * time.Millisecond,
	}
	if err := store.UpsertTarget(ctx, nil, rowB, []string{"gpu"}); err != nil {
		t.Fatalf("upsert B: %v", err)
	}
	if err := store.SetDefaultTarget(ctx, nil, "tenant:B"); err != nil {
		t.Fatalf("set default B: %v", err)
	}
	if _, err := store.BumpTargetsVersion(ctx, nil); err != nil {
		t.Fatalf("bump2: %v", err)
	}

	if !waitReplace(reg, 2, time.Second) {
		t.Fatalf("second replace not observed")
	}

	time.Sleep(50 * time.Millisecond)
	if c := reg.calls(); c != 2 {
		t.Fatalf("unexpected replace calls: %d", c)
	}

	tconn, ok := reg.Get("tenant:B")
	if !ok {
		t.Fatalf("target B missing")
	}
	if _, ok := tconn.Labels["gpu"]; !ok {
		t.Fatalf("label not applied: %#v", tconn.Labels)
	}
	if tconn.DB.Stats().MaxOpenConnections != 5 {
		t.Fatalf("MaxOpenConns not applied: %d", tconn.DB.Stats().MaxOpenConnections)
	}

	mu.Lock()
	got := append([]string(nil), dsns...)
	mu.Unlock()
	seenA, seenB := false, false
	for _, d := range got {
		if d == "dsnA" {
			seenA = true
		}
		if d == "dsnB" {
			seenB = true
		}
	}
	if !seenA || !seenB {
		t.Fatalf("dsns not recorded: %v", got)
	}
}
