package sdk

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestTargetRegistry(t *testing.T) {
	defDB, _ := sql.Open("sqlite3", ":memory:")
	reg := NewTargetRegistry(TargetConn{DB: defDB, Driver: "sqlite3"})

	if _, ok := reg.Default(); !ok {
		t.Fatalf("default not set")
	}

	dbA, _ := sql.Open("sqlite3", ":memory:")
	dbB, _ := sql.Open("sqlite3", ":memory:")
	if err := reg.Register(TargetConfig{Key: "a", DB: dbA}); err != nil {
		t.Fatalf("register a: %v", err)
	}
	if err := reg.Register(TargetConfig{Key: "b", DB: dbB}); err != nil {
		t.Fatalf("register b: %v", err)
	}

	if _, ok := reg.Get("a"); !ok {
		t.Fatalf("get a failed")
	}

	var keys []string
	if err := reg.ForEach(func(k string, _ TargetConn) error {
		keys = append(keys, k)
		return nil
	}); err != nil {
		t.Fatalf("foreach: %v", err)
	}
	if len(keys) != 3 { // default + a + b
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
}
