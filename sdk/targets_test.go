package sdk

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestTargetRegistry(t *testing.T) {
	defDB, _ := sql.Open("sqlite3", ":memory:")
	reg := NewHotReloadRegistry(&TargetConn{DB: defDB, Driver: "sqlite3"})

	if _, ok := reg.Default(); !ok {
		t.Fatalf("default not set")
	}

	dbA, _ := sql.Open("sqlite3", ":memory:")
	dbB, _ := sql.Open("sqlite3", ":memory:")
	if err := reg.Register(context.Background(), "a", TargetConfig{DB: dbA}, defaultConnector); err != nil {
		t.Fatalf("register a: %v", err)
	}
	if err := reg.Register(context.Background(), "b", TargetConfig{DB: dbB}, defaultConnector); err != nil {
		t.Fatalf("register b: %v", err)
	}

	if _, ok := reg.Get("a"); !ok {
		t.Fatalf("get a failed")
	}

	keys := reg.Keys()
	if len(keys) != 3 { // default + a + b
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
}

func TestRegistryAppliesPoolSettings(t *testing.T) {
	ctx := context.Background()
	reg := NewHotReloadRegistry(nil)

	if err := reg.Register(ctx, "a", TargetConfig{Driver: "sqlite3", DSN: ":memory:", MaxOpenConns: 1}, nil); err != nil {
		t.Fatalf("register: %v", err)
	}
	conn, _ := reg.Get("a")
	if got := conn.DB.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("max open %d", got)
	}

	if err := reg.Update(ctx, "a", TargetConfig{Driver: "sqlite3", DSN: ":memory:", MaxOpenConns: 3}, nil); err != nil {
		t.Fatalf("update: %v", err)
	}
	conn, _ = reg.Get("a")
	if got := conn.DB.Stats().MaxOpenConnections; got != 3 {
		t.Fatalf("max open after update %d", got)
	}

	cfgs := map[string]TargetConfig{"a": {Driver: "sqlite3", DSN: ":memory:", MaxOpenConns: 5}}
	if err := reg.ReplaceAll(ctx, cfgs, nil, "a"); err != nil {
		t.Fatalf("replaceAll: %v", err)
	}
	conn, _ = reg.Get("a")
	if got := conn.DB.Stats().MaxOpenConnections; got != 5 {
		t.Fatalf("max open after replace %d", got)
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	reg := NewHotReloadRegistry(nil)

	var wg sync.WaitGroup
	// writers
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			key := fmt.Sprintf("k%d", i)
			_ = reg.Register(ctx, key, TargetConfig{Driver: "sqlite3", DSN: ":memory:"}, nil)
			_ = reg.Update(ctx, key, TargetConfig{Driver: "sqlite3", DSN: ":memory:"}, nil)
			_ = reg.Unregister(key)
		}
		cfgs := map[string]TargetConfig{"x": {Driver: "sqlite3", DSN: ":memory:"}}
		_ = reg.ReplaceAll(ctx, cfgs, nil, "x")
	}()

	// readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			deadline := time.Now().Add(100 * time.Millisecond)
			for time.Now().Before(deadline) {
				reg.Get("nope")
				reg.Keys()
				reg.Default()
			}
		}()
	}

	wg.Wait()
}

func TestUpdateReplacesDSN(t *testing.T) {
	ctx := context.Background()
	reg := NewHotReloadRegistry(nil)

	if err := reg.Register(ctx, "a", TargetConfig{Driver: "sqlite3", DSN: ":memory:"}, nil); err != nil {
		t.Fatalf("register: %v", err)
	}
	old, _ := reg.Get("a")
	if _, err := old.DB.Exec("CREATE TABLE t (n int)"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	if err := reg.Update(ctx, "a", TargetConfig{Driver: "sqlite3", DSN: ":memory:"}, nil); err != nil {
		t.Fatalf("update: %v", err)
	}
	newConn, _ := reg.Get("a")
	if newConn.DB == old.DB {
		t.Fatalf("expected new DB instance")
	}
	if err := old.DB.Ping(); err == nil {
		t.Fatalf("old DB should be closed")
	}
	if err := newConn.DB.Ping(); err != nil {
		t.Fatalf("new DB ping: %v", err)
	}
	if _, err := newConn.DB.Exec("SELECT * FROM t"); err == nil {
		t.Fatalf("old table should not exist in new DB")
	}
}

func TestReplaceAllKeepsOldConnAlive(t *testing.T) {
	ctx := context.Background()
	reg := NewHotReloadRegistry(nil)
	if err := reg.Register(ctx, "a", TargetConfig{Driver: "sqlite3", DSN: ":memory:"}, nil); err != nil {
		t.Fatalf("register: %v", err)
	}
	old, _ := reg.Get("a")

	block := make(chan struct{})
	connector := func(ctx context.Context, driver, dsn string) (*sql.DB, error) {
		<-block
		return sql.Open("sqlite3", ":memory:")
	}

	done := make(chan error)
	go func() {
		_, err := old.DB.Exec("SELECT 1")
		done <- err
	}()

	go func() {
		cfgs := map[string]TargetConfig{"b": {Driver: "sqlite3", DSN: ":memory:"}}
		_ = reg.ReplaceAll(ctx, cfgs, connector, "b")
	}()

	time.Sleep(10 * time.Millisecond) // allow exec to start
	close(block)

	if err := <-done; err != nil {
		t.Fatalf("exec on old connection failed: %v", err)
	}
}
