package sdk

import (
	"context"
	"database/sql"
	"testing"
)

// helper to create registry with sample targets
func newTestRegistry(t *testing.T) TargetRegistry {
	reg := NewHotReloadRegistry(nil)
	ctx := context.Background()
	must := func(err error) {
		if err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	must(reg.Register(ctx, "a", TargetConfig{DB: new(sql.DB), Driver: "sqlite3", Labels: []string{"group=1"}}, nil))
	must(reg.Register(ctx, "b", TargetConfig{DB: new(sql.DB), Driver: "sqlite3", Labels: []string{"group=1", "primary=true"}}, nil))
	must(reg.Register(ctx, "c", TargetConfig{DB: new(sql.DB), Driver: "sqlite3", Labels: []string{"group=1"}}, nil))
	must(reg.Register(ctx, "d", TargetConfig{DB: new(sql.DB), Driver: "sqlite3", Labels: []string{"group=1"}}, nil))
	must(reg.Register(ctx, "e", TargetConfig{DB: new(sql.DB), Driver: "sqlite3", Labels: []string{"group=1"}}, nil))
	return reg
}

func TestChooseOneStrategies(t *testing.T) {
	reg := newTestRegistry(t)
	svc := &service{targets: reg, stratDefault: SelectFirst}

	if k, ok := svc.chooseOne([]string{"b", "c", "a"}, nil); !ok || k != "a" {
		t.Fatalf("SelectFirst got %s", k)
	}

	hint := &SelectionHint{Strategy: SelectPreferLabel, PreferLabel: "primary=true"}
	if k, ok := svc.chooseOne([]string{"a", "b", "c"}, hint); !ok || k != "b" {
		t.Fatalf("SelectPreferLabel got %s", k)
	}

	hashHint := &SelectionHint{Strategy: SelectConsistentHash, HashSource: "tenant1"}
	k1, _ := svc.chooseOne([]string{"a", "b", "c"}, hashHint)
	k2, _ := svc.chooseOne([]string{"c", "b", "a"}, hashHint)
	if k1 != k2 {
		t.Fatalf("SelectConsistentHash inconsistent: %s vs %s", k1, k2)
	}
}

func TestChooseOnePreferLabelMissing(t *testing.T) {
	reg := newTestRegistry(t)
	svc := &service{targets: reg, stratDefault: SelectFirst}
	hint := &SelectionHint{Strategy: SelectPreferLabel, PreferLabel: "doesnotexist=true"}
	if _, ok := svc.chooseOne([]string{"a", "b", "c"}, hint); ok {
		t.Fatalf("expected no selection when preferred label is missing")
	}
}

func TestChooseOneDefaultPrefer(t *testing.T) {
	reg := newTestRegistry(t)
	svc := &service{targets: reg, stratDefault: SelectPreferLabel, stratPrefer: "primary=true"}
	if k, ok := svc.chooseOne([]string{"a", "b", "c"}, nil); !ok || k != "b" {
		t.Fatalf("default prefer label got %s", k)
	}
}

func TestConsistentHashDifferentSources(t *testing.T) {
	reg := newTestRegistry(t)
	svc := &service{targets: reg, stratDefault: SelectConsistentHash}
	keys := []string{"a", "b", "c", "d", "e"}

	acmeHint := &SelectionHint{Strategy: SelectConsistentHash, HashSource: "tenant:acme"}
	a1, _ := svc.chooseOne(keys, acmeHint)
	a2, _ := svc.chooseOne(keys, acmeHint)
	if a1 != a2 {
		t.Fatalf("acme hash unstable: %s vs %s", a1, a2)
	}

	betaHint := &SelectionHint{Strategy: SelectConsistentHash, HashSource: "tenant:beta"}
	b1, _ := svc.chooseOne(keys, betaHint)
	b2, _ := svc.chooseOne(keys, betaHint)
	if b1 != b2 {
		t.Fatalf("beta hash unstable: %s vs %s", b1, b2)
	}
	if a1 == b1 {
		t.Fatalf("expected different keys for different hash sources: %s", a1)
	}
}
