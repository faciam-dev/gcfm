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

	"github.com/faciam-dev/gcfm/pkg/registry"
	"github.com/faciam-dev/gcfm/internal/server"
	sdk "github.com/faciam-dev/gcfm/sdk"
)

func TestAPI_Create_CF_Integration(t *testing.T) {
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

	disable := false
	svc := sdk.New(sdk.ServiceConfig{PluginEnabled: &disable})
	if err := svc.MigrateRegistry(ctx, sdk.DBConfig{Driver: "postgres", DSN: dsn}, 0); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	t.Setenv("JWT_SECRET", "testsecret")
	api := server.New(db, server.DBConfig{Driver: "postgres", DSN: dsn, TablePrefix: "gcfm_"})
	srv := httptest.NewServer(api.Adapter())
	defer srv.Close()

	body := `{"table":"posts","column":"title","type":"text","display":{"widget":"text"}}`
	resp, err := http.Post(srv.URL+"/v1/custom-fields", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d", resp.StatusCode)
	}

	var count int
	row := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM gcfm_custom_fields WHERE table_name='posts' AND column_name='title'`)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 got %d", count)
	}
}

func TestAPI_Create_CF_FullPayload(t *testing.T) {
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

	disable := false
	svc := sdk.New(sdk.ServiceConfig{PluginEnabled: &disable})
	if err := svc.MigrateRegistry(ctx, sdk.DBConfig{Driver: "postgres", DSN: dsn}, 0); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	t.Setenv("JWT_SECRET", "testsecret")
	api := server.New(db, server.DBConfig{Driver: "postgres", DSN: dsn, TablePrefix: "gcfm_"})
	srv := httptest.NewServer(api.Adapter())
	defer srv.Close()

	body := `{"table":"posts","column":"title","type":"text","display":{"labelKey":"test","placeholderKey":"test","widget":"text"},"nullable":true,"unique":false,"hasDefault":true,"defaultValue":"n/a","validator":"uuid"}`
	resp, err := http.Post(srv.URL+"/v1/custom-fields", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	defer resp.Body.Close()
	var out registry.FieldMeta
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.Nullable || out.Unique || !out.HasDefault || out.Default == nil || *out.Default != "n/a" || out.Validator != "uuid" {
		t.Fatalf("unexpected meta: %+v", out)
	}
	row := db.QueryRowContext(ctx, `SELECT nullable, "unique", has_default, default_value, validator FROM gcfm_custom_fields WHERE table_name='posts' AND column_name='title'`)
	var nullable, unique, hasDef bool
	var def, validator string
	if err := row.Scan(&nullable, &unique, &hasDef, &def, &validator); err != nil {
		t.Fatalf("select: %v", err)
	}
	if !nullable || unique || !hasDef || def != "n/a" || validator != "uuid" {
		t.Fatalf("persisted values mismatch: %v %v %v %s %s", nullable, unique, hasDef, def, validator)
	}
}

func TestAPI_Create_CF_CoreAuto(t *testing.T) {
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

	disable := false
	svc := sdk.New(sdk.ServiceConfig{PluginEnabled: &disable})
	if err := svc.MigrateRegistry(ctx, sdk.DBConfig{Driver: "postgres", DSN: dsn}, 0); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	t.Setenv("JWT_SECRET", "testsecret")
	api := server.New(db, server.DBConfig{Driver: "postgres", DSN: dsn, TablePrefix: "gcfm_"})
	srv := httptest.NewServer(api.Adapter())
	defer srv.Close()

	body := `{"table":"posts","column":"started_at","type":"date","display":{"widget":"core://auto"}}`
	resp, err := http.Post(srv.URL+"/v1/custom-fields", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	defer resp.Body.Close()
	var out registry.FieldMeta
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Display == nil || out.Display.Widget != "core://auto" || out.Display.WidgetResolved != "plugin://date-input" {
		t.Fatalf("unexpected display: %+v", out.Display)
	}
}
