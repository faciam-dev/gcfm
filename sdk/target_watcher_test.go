package sdk

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// waitDefault waits until registry default key matches expected.
func waitDefault(reg *HotReloadRegistry, expected string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if reg.snap.Load().(*snapshot).defaultKey == expected {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func TestFileProviderWatcher(t *testing.T) {
	ctx := context.Background()
	tmpdir := t.TempDir()
	path := tmpdir + "/targets.json"

	os.Setenv("TENANT_A_DSN", "dsnA")
	os.Setenv("TENANT_B_DSN", "dsnB")

	initial := `{"version":"v1","default":"tenant:A","targets":[{"key":"tenant:A","driver":"sqlite3","dsn":"${TENANT_A_DSN}"},{"key":"tenant:B","driver":"sqlite3","dsn":"${TENANT_B_DSN}"}]}`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	var mu sync.Mutex
	var dsns []string
	connector := func(ctx context.Context, driver, dsn string) (*sql.DB, error) {
		mu.Lock()
		dsns = append(dsns, dsn)
		mu.Unlock()
		return sql.Open("sqlite3", ":memory:")
	}

	svc := New(ServiceConfig{Connector: connector}).(*service)
	stop := svc.StartTargetWatcher(ctx, NewFileProvider(path), 10*time.Millisecond)
	defer stop()

	reg := svc.targets.(*HotReloadRegistry)
	if !waitDefault(reg, "tenant:A", time.Second) {
		t.Fatalf("default not updated to tenant:A")
	}

	updated := `{"version":"v2","default":"tenant:B","targets":[{"key":"tenant:A","driver":"sqlite3","dsn":"${TENANT_A_DSN}"},{"key":"tenant:B","driver":"sqlite3","dsn":"${TENANT_B_DSN}"}]}`
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatalf("write updated: %v", err)
	}

	if !waitDefault(reg, "tenant:B", time.Second) {
		t.Fatalf("default not updated to tenant:B")
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
		t.Fatalf("dsns not expanded: %v", got)
	}
}
