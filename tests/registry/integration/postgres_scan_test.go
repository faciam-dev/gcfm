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

	"github.com/faciam-dev/gcfm/pkg/registry"
	pscanner "github.com/faciam-dev/gcfm/pkg/driver/postgres"
)

func TestPostgresScanMetadata(t *testing.T) {
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
		t.Fatalf("container is nil")
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

	if _, err := db.ExecContext(ctx, `CREATE TABLE users (
        id SERIAL PRIMARY KEY,
        email TEXT UNIQUE NOT NULL,
        age INT,
        nickname TEXT DEFAULT 'guest'
    )`); err != nil {
		t.Fatalf("create: %v", err)
	}

	sc := pscanner.NewScanner(db)
	metas, err := sc.Scan(ctx, registry.DBConfig{Schema: "public"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	find := func(col string) registry.FieldMeta {
		for _, m := range metas {
			if m.TableName == "users" && m.ColumnName == col {
				return m
			}
		}
		t.Fatalf("column %s not found", col)
		return registry.FieldMeta{}
	}
	email := find("email")
	if email.Nullable || !email.Unique || email.HasDefault {
		t.Fatalf("email meta incorrect: %+v", email)
	}
	age := find("age")
	if !age.Nullable || age.Unique || age.HasDefault {
		t.Fatalf("age meta incorrect: %+v", age)
	}
	nick := find("nickname")
	if !nick.Nullable || nick.Unique || !nick.HasDefault || nick.Default == nil || !strings.Contains(*nick.Default, "guest") {
		t.Fatalf("nickname meta incorrect: %+v", nick)
	}
}
