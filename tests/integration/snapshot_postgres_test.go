//go:build integration
// +build integration

package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	sdk "github.com/faciam-dev/gcfm/sdk"
	client "github.com/faciam-dev/gcfm/sdk/client"
)

func TestSnapshot_Postgres(t *testing.T) {
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
	if err := svc.MigrateRegistry(ctx, sdk.DBConfig{Driver: "postgres", DSN: dsn, Schema: "public"}, 0); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// initial field
	if err := svc.CreateCustomField(ctx, sdk.FieldMeta{TableName: "posts", ColumnName: "title", DataType: "text"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	snap := client.NewLocalSnapshot(dsn, "postgres", "public", "gcfm_")
	s1, err := snap.Create(ctx, "t1", "patch", "")
	if err != nil {
		t.Fatalf("create snap1: %v", err)
	}
	list, err := snap.List(ctx, "t1")
	if err != nil || len(list) != 1 {
		t.Fatalf("list1: %v len=%d", err, len(list))
	}

	// modify registry
	if err := svc.CreateCustomField(ctx, sdk.FieldMeta{TableName: "posts", ColumnName: "age", DataType: "int"}); err != nil {
		t.Fatalf("create age: %v", err)
	}
	s2, err := snap.Create(ctx, "t1", "patch", "")
	if err != nil {
		t.Fatalf("create snap2: %v", err)
	}
	list, err = snap.List(ctx, "t1")
	if err != nil || len(list) != 2 {
		t.Fatalf("list2: %v len=%d", err, len(list))
	}

	diffStr, err := snap.Diff(ctx, "t1", s1.Semver, s2.Semver)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if !strings.Contains(diffStr, "column: age") {
		t.Fatalf("diff missing age: %s", diffStr)
	}

	var cnt int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM gcfm_custom_fields`).Scan(&cnt); err != nil {
		t.Fatalf("count before: %v", err)
	}
	if cnt != 2 {
		t.Fatalf("before rollback want 2 got %d", cnt)
	}
	if err := snap.Apply(ctx, "t1", s1.Semver); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM gcfm_custom_fields`).Scan(&cnt); err != nil {
		t.Fatalf("count after: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("after rollback want 1 got %d", cnt)
	}

	if _, err := snap.Create(ctx, "t2", "patch", ""); err != nil {
		t.Fatalf("create t2: %v", err)
	}
	list1, err := snap.List(ctx, "t1")
	if err != nil {
		t.Fatalf("list t1: %v", err)
	}
	list2, err := snap.List(ctx, "t2")
	if err != nil {
		t.Fatalf("list t2: %v", err)
	}
	if len(list1) != 2 || len(list2) != 1 {
		t.Fatalf("tenant lists wrong lengths %d %d", len(list1), len(list2))
	}
}
