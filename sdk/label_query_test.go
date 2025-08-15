package sdk

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sort"
	"testing"
)

type nopDriver struct{}

func (nopDriver) Open(string) (driver.Conn, error) { return nopConn{}, nil }

type nopConn struct{}

func (nopConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (nopConn) Close() error                        { return nil }
func (nopConn) Begin() (driver.Tx, error)           { return nil, errors.New("not implemented") }

func init() { sql.Register("nop", nopDriver{}) }

func TestLabelQueries(t *testing.T) {
	ctx := context.Background()
	reg := NewHotReloadRegistry(nil)

	must := func(err error) {
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	db, _ := sql.Open("nop", "")
	must(reg.Register(ctx, "a", TargetConfig{DB: db, Driver: "nop", Labels: []string{"Region=Tokyo", "Env=Prod", "GPU"}}, nil))
	must(reg.Register(ctx, "b", TargetConfig{DB: db, Driver: "nop", Labels: []string{"region=Tokyo", "ENV=Stg"}}, nil))
	must(reg.Register(ctx, "c", TargetConfig{DB: db, Driver: "nop", Labels: []string{"Region=Osaka", "Deprecated"}}, nil))

	// FindByLabel / existence
	if res := reg.FindByLabel("GPU"); !equalSlices(res, []string{"a"}) {
		t.Fatalf("FindByLabel gpu = %v", res)
	}
	if res := reg.FindByLabel("Region"); !equalSlices(res, []string{"a", "b", "c"}) {
		t.Fatalf("FindByLabel region = %v", res)
	}

	// FindAllByLabels AND
	if res := reg.FindAllByLabels("GPU", "REGION=Tokyo"); !equalSlices(res, []string{"a"}) {
		t.Fatalf("FindAllByLabels = %v", res)
	}

	// FindAnyByLabels OR
	if res := reg.FindAnyByLabels("REGION=Tokyo", "REGION=Osaka"); !equalSlices(res, []string{"a", "b", "c"}) {
		t.Fatalf("FindAnyByLabels = %v", res)
	}

	// Query with AND + IN + NOT
	q, err := ParseQuery("REGION=Tokyo,ENV in (Prod,Stg),!Deprecated")
	must(err)
	hits := reg.FindByQuery(q)
	if want := []string{"a", "b"}; !equalSlices(hits, want) {
		t.Fatalf("FindByQuery hits=%v want=%v", hits, want)
	}
	calls := make([]string, 0)
	err = reg.ForEachByQuery(q, func(k string, _ TargetConn) error {
		calls = append(calls, k)
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachByQuery err=%v", err)
	}
	sort.Strings(calls)
	if want := []string{"a", "b"}; !equalSlices(calls, want) {
		t.Fatalf("ForEachByQuery calls=%v want=%v", calls, want)
	}

	// NOT only query
	q, err = ParseQuery("!Deprecated")
	must(err)
	if hits = reg.FindByQuery(q); !equalSlices(hits, []string{"a", "b"}) {
		t.Fatalf("not query hits=%v", hits)
	}

	// empty set
	q, err = ParseQuery("region=osaka,env=prod")
	must(err)
	if hits = reg.FindByQuery(q); len(hits) != 0 {
		t.Fatalf("expected empty, got %v", hits)
	}

	// dedup in OR
	q, err = ParseQuery("GPU|ENV=Prod")
	must(err)
	if hits = reg.FindByQuery(q); !equalSlices(hits, []string{"a"}) {
		t.Fatalf("dedup failed %v", hits)
	}

	// ReplaceAll consistency
	cfgs := map[string]TargetConfig{
		"d": {DB: db, Driver: "nop", Labels: []string{"region=tokyo"}},
	}
	must(reg.ReplaceAll(ctx, cfgs, nil, "d"))
	if res := reg.FindByLabel("region=tokyo"); !equalSlices(res, []string{"d"}) {
		t.Fatalf("replace all failed: %v", res)
	}
	if res := reg.FindByLabel("gpu"); len(res) != 0 {
		t.Fatalf("old labels still indexed: %v", res)
	}
}

func TestLabelQueriesRace(t *testing.T) {
	ctx := context.Background()
	reg := NewHotReloadRegistry(nil)

	db, _ := sql.Open("nop", "")
	cfg := TargetConfig{DB: db, Driver: "nop", Labels: []string{"region=tokyo"}}
	if err := reg.Register(ctx, "a", cfg, nil); err != nil {
		t.Fatalf("register: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			reg.Get("a")
			reg.FindByLabel("region=tokyo")
			reg.FindAllByLabels("region=tokyo")
			reg.FindAnyByLabels("region=tokyo")
		}
	}()

	for i := 0; i < 100; i++ {
		if err := reg.Update(ctx, "a", cfg, nil); err != nil {
			t.Fatalf("update: %v", err)
		}
		if err := reg.ReplaceAll(ctx, map[string]TargetConfig{"a": cfg}, nil, ""); err != nil {
			t.Fatalf("replace: %v", err)
		}
	}

	<-done
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
