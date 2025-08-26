package widgets

import (
	"context"
	"testing"
	"time"
)

func TestListFilters(t *testing.T) {
	r := NewInMemory()
	ctx := context.Background()
	r.Upsert(ctx, Widget{ID: "w1", Name: "Widget One", Scopes: []string{"system"}})
	r.Upsert(ctx, Widget{ID: "w2", Name: "Widget Two", Scopes: []string{"tenant"}, Tenants: []string{"t1"}})
	r.Upsert(ctx, Widget{ID: "w3", Name: "Another", Scopes: []string{"tenant"}, Tenants: []string{"t2"}})

	items, total, _, _, _ := r.List(ctx, Options{})
	if total != 3 || len(items) != 3 {
		t.Fatalf("expected 3 items, got %d (%d)", len(items), total)
	}

	items, total, _, _, _ = r.List(ctx, Options{Scope: []string{"system"}})
	if total != 1 || items[0].ID != "w1" {
		t.Fatalf("scope filter failed")
	}

	items, total, _, _, _ = r.List(ctx, Options{Scope: []string{"tenant"}, Tenant: "t1"})
	if total != 1 || items[0].ID != "w2" {
		t.Fatalf("tenant filter failed")
	}

	items, total, _, _, _ = r.List(ctx, Options{Q: "another"})
	if total != 1 || items[0].ID != "w3" {
		t.Fatalf("query filter failed")
	}
}

func TestSubscribe(t *testing.T) {
	r := NewInMemory()
	ch, unsub := r.Subscribe()
	defer unsub()
	ctx := context.Background()
	r.Upsert(ctx, Widget{ID: "w1", Name: "Widget"})
	select {
	case ev := <-ch:
		if ev.Type != "upsert" || ev.Item == nil || ev.Item.ID != "w1" {
			t.Fatalf("unexpected event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatalf("no event received")
	}
}

func TestHasBuiltin(t *testing.T) {
	r := NewInMemory()
	if !r.Has("text-input") {
		t.Fatalf("expected builtin widget to be known")
	}
}
