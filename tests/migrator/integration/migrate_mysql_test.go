//go:build integration
// +build integration

package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go/modules/mysql"

	"github.com/faciam-dev/gcfm/pkg/migrator"
)

func TestMySQLMigrationIdempotent(t *testing.T) {
	ctx := context.Background()
	container, err := func() (c *mysql.MySQLContainer, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("%v", r)
			}
		}()
		return mysql.Run(ctx, "mysql:8.4",
			mysql.WithDatabase("testdb"),
			mysql.WithUsername("user"),
			mysql.WithPassword("pass"),
		)
	}()
	if err != nil {
		t.Skipf("container start: %v", err)
	}
	if container == nil {
		t.Fatalf("container is nil")
	}
	t.Cleanup(func() { container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	mig := migrator.NewWithDriver("mysql")
	if err := mig.Up(ctx, db, 1); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM gcfm_registry_schema_version WHERE version=1`); err != nil {
		t.Fatalf("delete version: %v", err)
	}
	if err := mig.Up(ctx, db, 1); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}
