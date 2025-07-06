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

func TestMigrateErrorIncludesSQL(t *testing.T) {
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

	// run first migration
	if out, err := exec.Command("go", "run", "./cmd/fieldctl", "db", "migrate", "--db", dsn, "--schema", "public", "--driver", "postgres", "--to", "1").CombinedOutput(); err != nil {
		t.Fatalf("migrate1: %v\n%s", err, out)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `ALTER TABLE gcfm_custom_fields ADD COLUMN label_key VARCHAR(255)`); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/fieldctl", "db", "migrate", "--db", dsn, "--schema", "public", "--driver", "postgres", "--to", "2")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error\n%s", out)
	}
	if !strings.Contains(string(out), "ALTER TABLE gcfm_custom_fields ADD COLUMN label_key") {
		t.Fatalf("error missing SQL:\n%s", out)
	}
}
