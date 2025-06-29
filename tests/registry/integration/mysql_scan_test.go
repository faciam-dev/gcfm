package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go/modules/mysql"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	mysqlscanner "github.com/faciam-dev/gcfm/internal/driver/mysql"
)

func TestScanAndUpsert(t *testing.T) {
	ctx := context.Background()
	t.Skip("integration test requires Docker")
	var err error
	container, err := func() (*mysql.MySQLContainer, error) {
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

	_, err = db.ExecContext(ctx, `CREATE TABLE users (id INT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(16))`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	_, err = db.ExecContext(ctx, `CREATE TABLE custom_fields (
        id BIGINT PRIMARY KEY AUTO_INCREMENT,
        table_name VARCHAR(255) NOT NULL,
        column_name VARCHAR(255) NOT NULL,
        data_type VARCHAR(255) NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        UNIQUE KEY unique_table_column (table_name, column_name)
    )`)
	if err != nil {
		t.Fatalf("create meta: %v", err)
	}

	sc := mysqlscanner.NewScanner(db)
	metas, err := sc.Scan(ctx, registry.DBConfig{Schema: "testdb"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if err := registry.UpsertSQL(ctx, db, "mysql", metas); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var count int
	row := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM custom_fields`)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if want := 2; count != want {
		t.Fatalf("got %d, want %d", count, want)
	}
}
