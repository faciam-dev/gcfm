//go:build integration
// +build integration

package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	sdk "github.com/faciam-dev/gcfm/sdk"
	client "github.com/faciam-dev/gcfm/sdk/client"
)

func TestLocalClient_Postgres(t *testing.T) {
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

	if _, err := db.ExecContext(ctx, `CREATE TABLE posts(id SERIAL PRIMARY KEY)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	svc := sdk.New(sdk.ServiceConfig{DB: db, Driver: "postgres", Schema: "public"})
	if err := svc.MigrateRegistry(ctx, sdk.DBConfig{Driver: "postgres", DSN: dsn}, 0); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cli := client.NewLocalService(svc)
	if err := cli.Create(ctx, sdk.FieldMeta{TableName: "posts", ColumnName: "title", DataType: "text"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	metas, err := cli.List(ctx, "posts")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("want 1 got %d", len(metas))
	}
}
