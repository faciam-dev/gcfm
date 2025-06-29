package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	pscanner "github.com/faciam-dev/gcfm/internal/driver/postgres"
)

func TestPostgresScan(t *testing.T) {
	ctx := context.Background()
	t.Skip("integration test requires Docker")
	var err error
	container, err := func() (*postgres.PostgresContainer, error) {
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

	if _, err := db.ExecContext(ctx, `CREATE TABLE users (id SERIAL PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}

	sc := pscanner.NewScanner(db)
	metas, err := sc.Scan(ctx, registry.DBConfig{Schema: "public"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(metas) == 0 {
		t.Fatalf("no metas")
	}
}
