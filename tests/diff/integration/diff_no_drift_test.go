//go:build integration
// +build integration

package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"strings"
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

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `CREATE TABLE posts(id SERIAL PRIMARY KEY, title TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	if out, err := exec.Command("go", "run", "./cmd/fieldctl", "registry", "migrate", "--db", dsn, "--driver", "postgres").CombinedOutput(); err != nil {
		t.Fatalf("migrate: %v\n%s", err, out)
	}

	yamlPath := t.TempDir() + "/reg.yaml"
	if out, err := exec.Command("go", "run", "./cmd/fieldctl", "export", "--db", dsn, "--schema", "public", "--driver", "postgres", "--out", yamlPath).CombinedOutput(); err != nil {
		t.Fatalf("export: %v\n%s", err, out)
	}

	out, err := exec.Command("go", "run", "./cmd/fieldctl", "diff", "--db", dsn, "--schema", "public", "--driver", "postgres", "--file", yamlPath).CombinedOutput()
	if err != nil {
		t.Fatalf("diff: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "✅ No schema drift detected.") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}
