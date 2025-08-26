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

	mysqlscanner "github.com/faciam-dev/gcfm/pkg/driver/mysql"
	"github.com/faciam-dev/gcfm/pkg/registry"
)

func TestMySQLScanMetadata(t *testing.T) {
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
	t.Cleanup(func() {
		container.Terminate(ctx)
	})

	dsn, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	_, err = db.ExecContext(ctx, `CREATE TABLE users (
        id INT PRIMARY KEY AUTO_INCREMENT,
        email VARCHAR(64) NOT NULL UNIQUE,
        age INT NULL,
        nickname VARCHAR(64) DEFAULT 'guest'
    )`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	sc := mysqlscanner.NewScanner(db)
	metas, err := sc.Scan(ctx, registry.DBConfig{Schema: "testdb"})
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
	if !nick.Nullable || nick.Unique || !nick.HasDefault || nick.Default == nil || *nick.Default != "guest" {
		t.Fatalf("nickname meta incorrect: %+v", nick)
	}
}
