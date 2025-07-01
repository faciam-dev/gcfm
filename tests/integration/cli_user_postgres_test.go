//go:build integration
// +build integration

package integration_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"testing"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestCLIUserCommands(t *testing.T) {
	ctx := context.Background()
	container, err := func() (c *postgres.PostgresContainer, err error) {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Recovered from panic: %v", r)
				err = fmt.Errorf("%v", r)
			}
		}()
		return postgres.Run(ctx, "postgres:16", postgres.WithDatabase("testdb"), postgres.WithUsername("user"), postgres.WithPassword("pass"))
	}()
	if err != nil {
		t.Skipf("container: %v", err)
	}
	if container == nil {
		t.Fatalf("container nil")
	}
	t.Cleanup(func() { container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}

	// migrate with seed
	cmd := exec.Command("go", "run", "./cmd/fieldctl", "db", "migrate", "--db", dsn, "--schema", "public", "--driver", "postgres", "--seed")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("migrate: %v\n%s", err, out)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	out, err := exec.Command("go", "run", "./cmd/fieldctl", "user", "list", "--db", dsn, "--driver", "postgres").CombinedOutput()
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("admin")) {
		t.Fatalf("admin not listed")
	}

	// create new user
	cmd = exec.Command("go", "run", "./cmd/fieldctl", "user", "create", "--db", dsn, "--username", "bob", "--password", "pw", "--role", "editor", "--driver", "postgres")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create: %v\n%s", err, out)
	}
	var count int
	row := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM gcfm_users WHERE username='bob'`)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("scan bob: %v", err)
	}
	if count != 1 {
		t.Fatalf("bob missing")
	}
}
