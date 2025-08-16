package sdk

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	metapkg "github.com/faciam-dev/gcfm/meta"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

func TestOrderCandidates(t *testing.T) {
	ctx := context.Background()
	reg := NewHotReloadRegistry(nil)
	_ = reg.Register(ctx, "p", TargetConfig{DB: &sql.DB{}}, nil)
	_ = reg.Register(ctx, "s1", TargetConfig{DB: &sql.DB{}, Labels: []string{"secondary=true"}}, nil)
	_ = reg.Register(ctx, "s2", TargetConfig{DB: &sql.DB{}, Labels: []string{"secondary=true"}}, nil)
	svc := &service{targets: reg}
	keys := []string{"s1", "s2"}
	hint := &SelectionHint{Strategy: SelectPreferLabel, PreferLabel: "secondary=true"}
	got := svc.orderCandidates(keys, "p", hint)
	want := []string{"p", "s1", "s2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("orderCandidates = %v, want %v", got, want)
	}
}

func TestRunWithTargetFailover(t *testing.T) {
	ctx := context.Background()
	reg := NewHotReloadRegistry(nil)
	_ = reg.Register(ctx, "p", TargetConfig{DB: &sql.DB{}, Schema: "p"}, nil)
	_ = reg.Register(ctx, "s", TargetConfig{DB: &sql.DB{}, Schema: "s", Labels: []string{"secondary=true"}}, nil)
	svc := &service{
		targets:  reg,
		failover: FailoverPolicy{Enabled: true, MaxAttempts: 2},
		classify: func(error) (bool, bool) { return true, true },
		health:   newHealthRegistry(FailoverPolicy{OpenAfterFailures: 1, OpenDuration: 0}),
	}
	q, _ := ParseQuery("secondary=true")
	dec := TargetDecision{Key: "p", Query: &q}
	var tries []string
	err := svc.RunWithTarget(ctx, dec, false, func(t TargetConn) error {
		tries = append(tries, t.Schema)
		if t.Schema == "p" {
			return errors.New("fail")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunWithTarget error: %v", err)
	}
	if !reflect.DeepEqual(tries, []string{"p", "s"}) {
		t.Fatalf("execution order = %v", tries)
	}
}

func TestCircuitBreaker(t *testing.T) {
	ctx := context.Background()
	reg := NewHotReloadRegistry(nil)
	_ = reg.Register(ctx, "p", TargetConfig{DB: &sql.DB{}, Schema: "p"}, nil)
	_ = reg.Register(ctx, "s", TargetConfig{DB: &sql.DB{}, Schema: "s", Labels: []string{"secondary=true"}}, nil)
	pol := FailoverPolicy{Enabled: true, MaxAttempts: 2, OpenAfterFailures: 1, OpenDuration: 20 * time.Millisecond}
	svc := &service{
		targets:  reg,
		failover: pol,
		classify: func(error) (bool, bool) { return true, true },
		health:   newHealthRegistry(pol),
	}
	q, _ := ParseQuery("secondary=true")
	dec := TargetDecision{Key: "p", Query: &q}

	// first call fails on primary -> circuit opens
	_ = svc.RunWithTarget(ctx, dec, false, func(t TargetConn) error {
		if t.Schema == "p" {
			return errors.New("fail")
		}
		return nil
	})

	// second call before expiry should skip primary
	var tries []string
	if err := svc.RunWithTarget(ctx, dec, false, func(t TargetConn) error {
		tries = append(tries, t.Schema)
		return nil
	}); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if !reflect.DeepEqual(tries, []string{"s"}) {
		t.Fatalf("expected secondary only, got %v", tries)
	}

	// after open duration, primary is probed again
	time.Sleep(25 * time.Millisecond)
	tries = tries[:0]
	if err := svc.RunWithTarget(ctx, dec, false, func(t TargetConn) error {
		tries = append(tries, t.Schema)
		return nil
	}); err != nil {
		t.Fatalf("third run: %v", err)
	}
	if !reflect.DeepEqual(tries, []string{"p"}) {
		t.Fatalf("expected probe on primary, got %v", tries)
	}
}

func TestRunWithTargetWriteNoRetry(t *testing.T) {
	ctx := context.Background()
	reg := NewHotReloadRegistry(nil)
	_ = reg.Register(ctx, "p", TargetConfig{DB: &sql.DB{}, Schema: "p"}, nil)
	_ = reg.Register(ctx, "s", TargetConfig{DB: &sql.DB{}, Schema: "s", Labels: []string{"secondary=true"}}, nil)
	svc := &service{
		targets:  reg,
		failover: FailoverPolicy{Enabled: true, MaxAttempts: 2},
		classify: func(error) (bool, bool) { return true, true },
		health:   newHealthRegistry(FailoverPolicy{OpenAfterFailures: 1, OpenDuration: time.Minute}),
	}
	q, _ := ParseQuery("secondary=true")
	dec := TargetDecision{Key: "p", Query: &q}
	var tries []string
	err := svc.RunWithTarget(ctx, dec, true, func(t TargetConn) error {
		tries = append(tries, t.Schema)
		return errors.New("fail")
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !reflect.DeepEqual(tries, []string{"p"}) {
		t.Fatalf("expected only primary attempt, got %v", tries)
	}
}

func TestWatcherFailoverSwitch(t *testing.T) {
	ctx := context.Background()
	store := newTargetStore(t)
	if err := store.UpsertTarget(ctx, nil, metapkg.TargetRow{Key: "p", Driver: "sqlite3", DSN: ":memory:", Schema: "p", IsDefault: true}, []string{"tenant=A"}); err != nil {
		t.Fatalf("upsert p: %v", err)
	}
	if _, err := store.BumpTargetsVersion(ctx, nil); err != nil {
		t.Fatalf("bump1: %v", err)
	}

	reg := NewHotReloadRegistry(nil)
	svc := &service{
		targets:  reg,
		cn:       func(ctx context.Context, driver, dsn string) (*sql.DB, error) { return sql.Open("sqlite3", dsn) },
		logger:   zap.NewNop().Sugar(),
		failover: FailoverPolicy{Enabled: true, MaxAttempts: 2, PreferOnFail: &SelectionHint{Strategy: SelectPreferLabel, PreferLabel: "secondary=true"}, OpenAfterFailures: 1, OpenDuration: time.Minute},
		classify: func(error) (bool, bool) { return true, true },
	}
	svc.health = newHealthRegistry(svc.failover)
	stop := svc.StartTargetWatcher(ctx, NewMetaDBProvider(store), 10*time.Millisecond)
	defer stop()

	// wait for primary to load
	for i := 0; i < 100; i++ {
		if _, ok := reg.Get("p"); ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	q, _ := ParseQuery("tenant=A")
	dec := TargetDecision{Key: "p", Query: &q}

	// first attempt fails on primary and opens circuit
	_ = svc.RunWithTarget(ctx, dec, false, func(t TargetConn) error {
		return errors.New("down")
	})

	// add secondary target and bump version
	if err := store.UpsertTarget(ctx, nil, metapkg.TargetRow{Key: "s", Driver: "sqlite3", DSN: ":memory:", Schema: "s"}, []string{"tenant=A", "secondary=true"}); err != nil {
		t.Fatalf("upsert s: %v", err)
	}
	if _, err := store.BumpTargetsVersion(ctx, nil); err != nil {
		t.Fatalf("bump2: %v", err)
	}

	// wait for watcher to load secondary
	for i := 0; i < 100; i++ {
		if _, ok := reg.Get("s"); ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	var tries []string
	if err := svc.RunWithTarget(ctx, dec, false, func(t TargetConn) error {
		tries = append(tries, t.Schema)
		if t.Schema == "p" {
			return errors.New("down")
		}
		return nil
	}); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if !reflect.DeepEqual(tries, []string{"s"}) {
		t.Fatalf("expected secondary after watcher, got %v", tries)
	}
}
