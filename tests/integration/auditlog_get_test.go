//go:build integration
// +build integration

package integration_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/faciam-dev/gcfm/internal/server"
	sdk "github.com/faciam-dev/gcfm/sdk"
)

func TestAuditLog_Get(t *testing.T) {
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

	disable := false
	svc := sdk.New(sdk.ServiceConfig{PluginEnabled: &disable})
	if err := svc.MigrateRegistry(ctx, sdk.DBConfig{Driver: "postgres", DSN: dsn, Schema: "public"}, 0); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var id int
	err = db.QueryRowContext(ctx, `INSERT INTO gcfm_audit_logs(actor, action, table_name, column_name, before_json, after_json) VALUES ('alice','update','posts','title','{"old":1}','{"new":2}') RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	t.Setenv("JWT_SECRET", "testsecret")
	api := server.New(db, server.DBConfig{Driver: "postgres", DSN: dsn, TablePrefix: "gcfm_"})
	srv := httptest.NewServer(api.Adapter())
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/v1/audit-logs/%d", srv.URL, id))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	defer resp.Body.Close()
	var log map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&log); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if log["id"] == nil || log["actor"] == nil || log["action"] == nil || log["tableName"] == nil {
		t.Fatalf("missing fields in response: %v", log)
	}
	if log["beforeJson"] == nil || log["afterJson"] == nil {
		t.Fatalf("expected beforeJson and afterJson, got %v", log)
	}
}
