//go:build integration
// +build integration

package integration_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/faciam-dev/gcfm/internal/server"
)

func TestWidgetPolicySuggest(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:16", postgres.WithDatabase("testdb"), postgres.WithUsername("user"), postgres.WithPassword("pass"))
	if err != nil {
		t.Skipf("container: %v", err)
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

	t.Setenv("JWT_SECRET", "testsecret")
	api := server.New(db, server.DBConfig{Driver: "postgres", DSN: dsn, TablePrefix: "gcfm_"})
	srv := httptest.NewServer(api.Adapter())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/widget-policies/suggest?driver=postgres&type=date&name=created_at")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var out struct {
		Resolved struct {
			ID string `json:"id"`
		} `json:"resolved"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Resolved.ID != "plugin://date-input" {
		t.Fatalf("resolved %s", out.Resolved.ID)
	}
}

func TestWidgetPolicySuggestEmail(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:16", postgres.WithDatabase("testdb"), postgres.WithUsername("user"), postgres.WithPassword("pass"))
	if err != nil {
		t.Skipf("container: %v", err)
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

	t.Setenv("JWT_SECRET", "testsecret")
	api := server.New(db, server.DBConfig{Driver: "postgres", DSN: dsn, TablePrefix: "gcfm_"})
	srv := httptest.NewServer(api.Adapter())
	defer srv.Close()

	url := srv.URL + "/v1/widget-policies/suggest?driver=mysql&type=varchar&validator=email&length=50&name=email"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var out struct {
		Resolved struct {
			ID     string            `json:"id"`
			Config map[string]string `json:"config"`
		} `json:"resolved"`
		Suggested []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"suggested"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Resolved.ID != "plugin://email-input" {
		t.Fatalf("resolved %s", out.Resolved.ID)
	}
	if out.Resolved.Config["placeholder"] != "name@example.com" {
		t.Fatalf("placeholder %v", out.Resolved.Config)
	}
	exp := []struct{ ID, Label string }{
		{"core://auto", "System default"},
		{"plugin://email-input", "Email Input"},
		{"plugin://text-input", "text-input"},
	}
	if len(out.Suggested) < len(exp) {
		t.Fatalf("suggested len=%d", len(out.Suggested))
	}
	for i, e := range exp {
		if out.Suggested[i].ID != e.ID || out.Suggested[i].Label != e.Label {
			t.Fatalf("suggested[%d]=%v", i, out.Suggested[i])
		}
	}
}
