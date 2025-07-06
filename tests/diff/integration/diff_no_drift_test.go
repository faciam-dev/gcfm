//go:build integration
// +build integration

package integration_test

import (
	"context"
	"fmt"
	"os/exec"
	"testing"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestDiffNoDrift(t *testing.T) {
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

	if out, err := exec.Command("go", "run", "./cmd/fieldctl", "diff", "--db", dsn, "--schema", "public", "--driver", "postgres", "--file", "registry.yaml").CombinedOutput(); err != nil {
		t.Fatalf("diff: %v\n%s", err, out)
	}
}
