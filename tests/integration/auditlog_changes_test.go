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
	auditutil "github.com/faciam-dev/gcfm/pkg/audit"
	sdk "github.com/faciam-dev/gcfm/sdk"
)

func TestAuditLog_MinMaxChanges(t *testing.T) {
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

	_, add, del := auditutil.UnifiedDiff([]byte(`{"v":1}`), []byte(`{"v":2}`))
	if _, err := db.ExecContext(ctx, `INSERT INTO gcfm_audit_logs(actor, action, table_name, column_name, before_json, after_json, added_count, removed_count, change_count) VALUES ('alice','update','posts','title',$1,$2,$3,$4,$5)`,
		`{"v":1}`, `{"v":2}`, add, del, add+del); err != nil {
		t.Fatalf("insert: %v", err)
	}

	t.Setenv("JWT_SECRET", "testsecret")
	api := server.New(db, server.DBConfig{Driver: "postgres", DSN: dsn, TablePrefix: "gcfm_"})
	srv := httptest.NewServer(api.Adapter())
	defer srv.Close()

	cases := []struct {
		name  string
		query string
		want  int
	}{
		{"min-hit", "?min_changes=2", 1},
		{"min-miss", "?min_changes=3", 0},
		{"max-hit", "?max_changes=2", 1},
		{"max-miss", "?max_changes=1", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(srv.URL + "/v1/audit-logs" + tc.query)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status=%d", resp.StatusCode)
			}
			defer resp.Body.Close()
			var out struct {
				Items []map[string]any `json:"items"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(out.Items) != tc.want {
				t.Fatalf("want %d got %d", tc.want, len(out.Items))
			}
		})
	}
}
