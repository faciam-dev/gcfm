//go:build integration
// +build integration

package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"testing"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestDiffDrift(t *testing.T) {
	ctx := context.Background()
	container, err := func() (c *postgres.PostgresContainer, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("%v", r)
			}
		}()
		return postgres.Run(ctx, "postgres:16", postgres.WithDatabase("app"), postgres.WithUsername("postgres"), postgres.WithPassword("pass"))
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

	if out, err := exec.Command("go", "run", "./cmd/fieldctl", "db", "migrate", "--db", dsn, "--schema", "public", "--driver", "postgres", "--seed").CombinedOutput(); err != nil {
		t.Fatalf("migrate: %v\n%s", err, out)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `ALTER TABLE gcfm_custom_fields DROP COLUMN validator`); err != nil {
		t.Fatalf("alter: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/fieldctl", "diff", "--db", dsn, "--schema", "public", "--driver", "postgres", "--file", "registry.yaml", "--fail-on-change")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("expected error, got none\n%s", out)
	} else {
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 2 {
			t.Fatalf("expected exit 2 got %v\n%s", err, out)
		}
	}
}
