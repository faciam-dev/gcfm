//go:build integration
// +build integration

package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go/modules/mysql"

	"github.com/faciam-dev/gcfm/internal/server"
	sdk "github.com/faciam-dev/gcfm/sdk"
)

func TestAPI_Create_CF_InvalidDefault_MySQL(t *testing.T) {
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
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `CREATE TABLE posts(id INT PRIMARY KEY AUTO_INCREMENT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	disable := false
	svc := sdk.New(sdk.ServiceConfig{PluginEnabled: &disable})
	if err := svc.MigrateRegistry(ctx, sdk.DBConfig{Driver: "mysql", DSN: dsn}, 0); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	t.Setenv("JWT_SECRET", "testsecret")
	api := server.New(db, "mysql", dsn)
	srv := httptest.NewServer(api.Adapter())
	defer srv.Close()

	body := `{"table":"posts","column":"body","type":"text","default":"abc"}`
	resp, err := http.Post(srv.URL+"/v1/custom-fields", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}
