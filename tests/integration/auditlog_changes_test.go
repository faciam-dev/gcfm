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
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/faciam-dev/gcfm/internal/server"
	auditutil "github.com/faciam-dev/gcfm/pkg/audit"
	sdk "github.com/faciam-dev/gcfm/sdk"
)

func TestAuditLog_CountsAndFilters(t *testing.T) {
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

	t.Setenv("JWT_SECRET", "testsecret")
	api := server.New(db, server.DBConfig{Driver: "postgres", DSN: dsn, TablePrefix: "gcfm_"})
	srv := httptest.NewServer(api.Adapter())
	defer srv.Close()

	// empty case
	resp, err := http.Get(srv.URL + "/v1/audit-logs")
	if err != nil {
		t.Fatalf("get empty: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var out struct{ Items []map[string]any }
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode empty: %v", err)
	}
	resp.Body.Close()
	if len(out.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(out.Items))
	}

	// insert logs with various change counts
	_, add1, del1 := auditutil.UnifiedDiff([]byte(`{}`), []byte(`{"a":1,"b":1,"c":1,"d":1,"e":1}`))
	var firstID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO gcfm_audit_logs(actor, action, table_name, column_name, before_json, after_json, added_count, removed_count, change_count) VALUES ('alice','update','posts','t1',$1,$2,$3,$4,$5) RETURNING id`,
		`{}`, `{"a":1,"b":1,"c":1,"d":1,"e":1}`, add1, del1, add1+del1).Scan(&firstID); err != nil {
		t.Fatalf("insert1: %v", err)
	}
	_, add2, del2 := auditutil.UnifiedDiff([]byte(`{"a":1,"b":2}`), []byte(`{"c":3,"d":4,"e":5}`))
	if _, err := db.ExecContext(ctx, `INSERT INTO gcfm_audit_logs(actor, action, table_name, column_name, before_json, after_json, added_count, removed_count, change_count) VALUES ('bob','update','posts','t2',$1,$2,$3,$4,$5)`,
		`{"a":1,"b":2}`, `{"c":3,"d":4,"e":5}`, add2, del2, add2+del2); err != nil {
		t.Fatalf("insert2: %v", err)
	}
	_, add3, del3 := auditutil.UnifiedDiff([]byte(`{"v":1}`), []byte(`{"v":2}`))
	if _, err := db.ExecContext(ctx, `INSERT INTO gcfm_audit_logs(actor, action, table_name, column_name, before_json, after_json, added_count, removed_count, change_count) VALUES ('carol','update','posts','t3',$1,$2,$3,$4,$5)`,
		`{"v":1}`, `{"v":2}`, add3, del3, add3+del3); err != nil {
		t.Fatalf("insert3: %v", err)
	}
	_, add4, del4 := auditutil.UnifiedDiff([]byte(`{}`), []byte(`{"a":1,"b":1,"c":1,"d":1,"e":1,"f":1}`))
	if _, err := db.ExecContext(ctx, `INSERT INTO gcfm_audit_logs(actor, action, table_name, column_name, before_json, after_json, added_count, removed_count, change_count) VALUES ('dave','update','posts','t4',$1,$2,$3,$4,$5)`,
		`{}`, `{"a":1,"b":1,"c":1,"d":1,"e":1,"f":1}`, add4, del4, add4+del4); err != nil {
		t.Fatalf("insert4: %v", err)
	}

	// verify summaries and counts
	resp, err = http.Get(srv.URL + "/v1/audit-logs")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	out = struct{ Items []map[string]any }{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp.Body.Close()
	if len(out.Items) != 4 {
		t.Fatalf("want 4 got %d", len(out.Items))
	}
	found := map[string]bool{"+5 -0": false, "+3 -2": false, "+1 -1": false, "+6 -0": false}
	for _, it := range out.Items {
		summary := it["summary"].(string)
		count := int(it["changeCount"].(float64))
		switch summary {
		case "+5 -0":
			if count != 5 {
				t.Fatalf("summary %s count %d", summary, count)
			}
			found[summary] = true
		case "+3 -2":
			if count != 5 {
				t.Fatalf("summary %s count %d", summary, count)
			}
			found[summary] = true
		case "+1 -1":
			if count != 2 {
				t.Fatalf("summary %s count %d", summary, count)
			}
			found[summary] = true
		case "+6 -0":
			if count != 6 {
				t.Fatalf("summary %s count %d", summary, count)
			}
			found[summary] = true
		}
	}
	for k, v := range found {
		if !v {
			t.Fatalf("missing summary %s", k)
		}
	}

	// verify diff endpoint returns unified text
	resp, err = http.Get(fmt.Sprintf("%s/v1/audit-logs/%d/diff", srv.URL, firstID))
	if err != nil {
		t.Fatalf("get diff: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("diff status=%d", resp.StatusCode)
	}
	var diffOut struct {
		Diff    string `json:"diff"`
		Added   int    `json:"added"`
		Removed int    `json:"removed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&diffOut); err != nil {
		t.Fatalf("diff decode: %v", err)
	}
	resp.Body.Close()
	if diffOut.Added != add1 || diffOut.Removed != del1 {
		t.Fatalf("diff counts %d %d", diffOut.Added, diffOut.Removed)
	}
	if !strings.Contains(diffOut.Diff, "--- before") || !strings.Contains(diffOut.Diff, "+++ after") {
		t.Fatalf("diff text not unified: %s", diffOut.Diff)
	}

	// filter cases
	cases := []struct {
		name  string
		query string
		want  int
	}{
		{"eq-five", "?min_changes=5&max_changes=5", 2},
		{"min-six", "?min_changes=6", 1},
		{"max-two", "?max_changes=2", 1},
		{"range-zero-five", "?min_changes=0&max_changes=5", 3},
		{"camel-range", "?minChanges=0&maxChanges=5", 3},
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
			var out struct{ Items []map[string]any }
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(out.Items) != tc.want {
				t.Fatalf("want %d got %d", tc.want, len(out.Items))
			}
		})
	}
}
