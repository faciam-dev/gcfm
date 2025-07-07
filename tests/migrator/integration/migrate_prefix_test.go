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

const prefix = "gcfm_"

func runFieldctl(t *testing.T, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "./cmd/fieldctl"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fieldctl %v: %v\n%s", args, err, out)
	}
	return out
}

func setupPostgres(t *testing.T) (*postgres.PostgresContainer, string) {
	t.Helper()
	ctx := context.Background()
	c, err := func() (c *postgres.PostgresContainer, err error) {
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
	if c == nil {
		t.Fatalf("container nil")
	}
	t.Cleanup(func() { c.Terminate(ctx) })
	dsn, err := c.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	return c, dsn
}

func TestMigrateBootstrap(t *testing.T) {
	_, dsn := setupPostgres(t)
	runFieldctl(t, "db", "migrate", "--db", dsn, "--driver", "postgres", "--schema", "public", "--table-prefix", prefix)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	var v int
	if err := db.QueryRow(`SELECT version FROM ` + prefix + `registry_schema_version`).Scan(&v); err != nil {
		t.Fatalf("select version: %v", err)
	}
	if v != 0 {
		t.Fatalf("expected version 0 got %d", v)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	_, dsn := setupPostgres(t)
	runFieldctl(t, "db", "migrate", "--db", dsn, "--driver", "postgres", "--schema", "public", "--table-prefix", prefix)
	runFieldctl(t, "db", "migrate", "--db", dsn, "--driver", "postgres", "--schema", "public", "--table-prefix", prefix)
}

func TestDiffAfterMigrate(t *testing.T) {
	_, dsn := setupPostgres(t)
	runFieldctl(t, "db", "migrate", "--db", dsn, "--driver", "postgres", "--schema", "public", "--table-prefix", prefix)

	yamlPath := t.TempDir() + "/reg.yaml"
	runFieldctl(t, "export", "--db", dsn, "--schema", "public", "--driver", "postgres", "--out", yamlPath)

	out := runFieldctl(t, "diff", "--db", dsn, "--schema", "public", "--driver", "postgres", "--file", yamlPath, "--table-prefix", prefix)
	if string(out) != "✅ No schema drift detected.\n" {
		t.Fatalf("unexpected diff: %s", out)
	}
}
