package sdk

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

func createCFTable(t *testing.T, db *sql.DB) {
	t.Helper()
	schema := `CREATE TABLE ` + registry.T("custom_fields") + ` (
       db_id INTEGER,
       tenant_id TEXT,
       table_name TEXT,
       column_name TEXT,
       data_type TEXT,
       label_key TEXT,
       widget TEXT,
       placeholder_key TEXT,
       nullable BOOLEAN NOT NULL DEFAULT 0,
       "unique" BOOLEAN NOT NULL DEFAULT 0,
       has_default BOOLEAN NOT NULL DEFAULT 0,
       default_value TEXT,
       validator TEXT
   );`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create table: %v", err)
	}
}

func TestTenantResolver(t *testing.T) {
	aDB, _ := sql.Open("sqlite3", ":memory:")
	bDB, _ := sql.Open("sqlite3", ":memory:")
	createCFTable(t, aDB)
	createCFTable(t, bDB)
	// insert different rows
	aDB.Exec(`INSERT INTO ` + registry.T("custom_fields") + ` (db_id, tenant_id, table_name, column_name, data_type, nullable, "unique", has_default) VALUES (1, 'default', 'posts', 'a_col', 'text', 1, 0, 0)`)
	bDB.Exec(`INSERT INTO ` + registry.T("custom_fields") + ` (db_id, tenant_id, table_name, column_name, data_type, nullable, "unique", has_default) VALUES (1, 'default', 'posts', 'b_col', 'text', 1, 0, 0)`)

	svc := New(ServiceConfig{
		Targets: []TargetConfig{
			{Key: "tenant:a", DB: aDB, Driver: "sqlite3"},
			{Key: "tenant:b", DB: bDB, Driver: "sqlite3"},
		},
		TargetResolver: TenantResolverFromPrefix("tenant:"),
	}).(*service)

	ctxA := WithTenantID(context.Background(), "a")
	metasA, err := svc.ListCustomFields(ctxA, 1, "")
	if err != nil {
		t.Fatalf("list a: %v", err)
	}
	if len(metasA) != 1 || metasA[0].ColumnName != "a_col" {
		t.Fatalf("unexpected metasA: %+v", metasA)
	}

	ctxB := WithTenantID(context.Background(), "b")
	metasB, err := svc.ListCustomFields(ctxB, 1, "")
	if err != nil {
		t.Fatalf("list b: %v", err)
	}
	if len(metasB) != 1 || metasB[0].ColumnName != "b_col" {
		t.Fatalf("unexpected metasB: %+v", metasB)
	}
}

func TestPickTargetFallback(t *testing.T) {
	db, _ := sql.Open("sqlite3", ":memory:")
	svc := New(ServiceConfig{DB: db, Driver: "sqlite3", TargetResolver: func(ctx context.Context) (string, bool) {
		return "missing", true
	}}).(*service)
	if _, err := svc.pickTarget(context.Background()); err != nil {
		t.Fatalf("fallback to default failed: %v", err)
	}

	svc = New(ServiceConfig{TargetResolver: func(ctx context.Context) (string, bool) {
		return "", false
	}}).(*service)
	if _, err := svc.pickTarget(context.Background()); err == nil {
		t.Fatalf("expected error when no target")
	}
}

func TestPickTargetPriority(t *testing.T) {
	ctx := context.Background()
	reg := NewHotReloadRegistry(nil)
	must := func(err error) {
		if err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	must(reg.Register(ctx, "v1", TargetConfig{DB: new(sql.DB), Driver: "sqlite3", Schema: "v1", Labels: []string{"group=1"}}, nil))
	must(reg.Register(ctx, "v2", TargetConfig{DB: new(sql.DB), Driver: "sqlite3", Schema: "v2", Labels: []string{"group=1", "primary=true"}}, nil))
	must(reg.Register(ctx, "def", TargetConfig{DB: new(sql.DB), Driver: "sqlite3", Schema: "def"}, nil))
	reg.SetDefault("def")

	t.Run("v2 key overrides v1", func(t *testing.T) {
		svc := &service{
			targets:   reg,
			resolveV1: func(context.Context) (string, bool) { return "v1", true },
			resolveV2: func(context.Context) (TargetDecision, bool) { return TargetDecision{Key: "v2"}, true },
		}
		tc, err := svc.pickTarget(context.Background())
		if err != nil || tc.Schema != "v2" {
			t.Fatalf("expected v2, got %s err=%v", tc.Schema, err)
		}
	})

	t.Run("v2 query overrides v1", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("X-Group", "1")
		ctxReq := WithHTTPRequest(context.Background(), req)

		svc := &service{
			targets:   reg,
			resolveV1: func(context.Context) (string, bool) { return "v1", true },
			resolveV2: AutoLabelResolver(AutoLabelResolverOptions{
				HTTP: &HTTPLabelRules{HeaderMap: map[string]string{"X-Group": "group"}},
				Hint: &SelectionHint{Strategy: SelectPreferLabel, PreferLabel: "primary=true"},
			}),
			stratDefault: SelectFirst,
		}
		tc, err := svc.pickTarget(ctxReq)
		if err != nil || tc.Schema != "v2" {
			t.Fatalf("expected v2 via query, got %s err=%v", tc.Schema, err)
		}
	})

	t.Run("fallback order", func(t *testing.T) {
		svc := &service{
			targets:      reg,
			resolveV1:    func(context.Context) (string, bool) { return "v1", true },
			resolveV2:    AutoLabelResolver(AutoLabelResolverOptions{HTTP: &HTTPLabelRules{HeaderMap: map[string]string{"X-Region": "region"}}}),
			stratDefault: SelectFirst,
		}
		tc, err := svc.pickTarget(context.Background())
		if err != nil || tc.Schema != "v1" {
			t.Fatalf("expected v1 fallback, got %s err=%v", tc.Schema, err)
		}

		svc.resolveV1 = func(context.Context) (string, bool) { return "", false }
		tc, err = svc.pickTarget(context.Background())
		if err != nil || tc.Schema != "def" {
			t.Fatalf("expected default, got %s err=%v", tc.Schema, err)
		}
	})
}
