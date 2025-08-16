//go:build integration
// +build integration

package integration_test

import (
	"bytes"
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

type tokenResp struct {
	AccessToken string `json:"access_token"`
}

type targetResp struct {
	Key    string   `json:"key"`
	Driver string   `json:"driver"`
	DSN    string   `json:"dsn"`
	Labels []string `json:"labels"`
}

type listResp struct {
	Items      []targetResp `json:"items"`
	NextCursor string       `json:"nextCursor"`
}

type versionResp struct {
	Version    string `json:"version"`
	DefaultKey string `json:"defaultKey"`
}

func do(req *http.Request, token string) (*http.Response, error) {
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return http.DefaultClient.Do(req)
}

func TestAdminTargetsCRUD_Postgres(t *testing.T) {
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

	disable := false
	svc := sdk.New(sdk.ServiceConfig{PluginEnabled: &disable})
	if err := svc.MigrateRegistry(ctx, sdk.DBConfig{Driver: "postgres", DSN: dsn}, 0); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	t.Setenv("JWT_SECRET", "testsecret")
	api := server.New(db, server.DBConfig{Driver: "postgres", DSN: dsn, TablePrefix: "gcfm_"})
	srv := httptest.NewServer(api.Adapter())
	defer srv.Close()

	// login to get token
	loginBody := bytes.NewBufferString(`{"username":"admin","password":"admin123"}`)
	reqLogin, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/auth/login", loginBody)
	reqLogin.Header.Set("Content-Type", "application/json")
	reqLogin.Header.Set("X-Tenant-ID", "default")
	resp, err := http.DefaultClient.Do(reqLogin)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d", resp.StatusCode)
	}
	var tk tokenResp
	if err := json.NewDecoder(resp.Body).Decode(&tk); err != nil {
		t.Fatalf("decode token: %v", err)
	}
	resp.Body.Close()
	if tk.AccessToken == "" {
		t.Fatalf("missing token")
	}

	// 1) GET list -> ETag v0
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/admin/targets", nil)
	resp, err = do(req, tk.AccessToken)
	if err != nil {
		t.Fatalf("get list: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d", resp.StatusCode)
	}
	v0 := resp.Header.Get("ETag")
	if v0 == "" {
		t.Fatalf("etag v0 empty")
	}
	resp.Body.Close()

	// 2) POST create target A
	bodyA := `{"key":"A","driver":"postgres","dsn":"postgres://example/A","labels":["tenant=A","region=tokyo","env=prod","primary=true"]}`
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/admin/targets", bytes.NewBufferString(bodyA))
	req.Header.Set("Content-Type", "application/json")
	resp, err = do(req, tk.AccessToken)
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("create status=%d", resp.StatusCode)
	}
	v1 := resp.Header.Get("ETag")
	if v1 == "" || v1 == v0 {
		t.Fatalf("etag v1 invalid")
	}
	resp.Body.Close()

	// 3) PUT upsert B with If-Match v1
	bodyB := `{"driver":"postgres","dsn":"postgres://example/B","labels":["tenant=B"]}`
	req, _ = http.NewRequest(http.MethodPut, srv.URL+"/admin/targets/B", bytes.NewBufferString(bodyB))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", v1)
	resp, err = do(req, tk.AccessToken)
	if err != nil {
		t.Fatalf("put B: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put status=%d", resp.StatusCode)
	}
	v2 := resp.Header.Get("ETag")
	if v2 == "" || v2 == v1 {
		t.Fatalf("etag v2 invalid")
	}
	resp.Body.Close()

	// 4) POST set B as default with If-Match v2
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/admin/targets/B/default", nil)
	req.Header.Set("If-Match", v2)
	resp, err = do(req, tk.AccessToken)
	if err != nil {
		t.Fatalf("set default: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Fatalf("set default status=%d", resp.StatusCode)
	}
	v3 := resp.Header.Get("ETag")
	if v3 == "" || v3 == v2 {
		t.Fatalf("etag v3 invalid")
	}
	resp.Body.Close()

	// verify version endpoint
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/admin/targets/version", nil)
	resp, err = do(req, tk.AccessToken)
	if err != nil {
		t.Fatalf("get version: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get version status=%d", resp.StatusCode)
	}
	var verOut versionResp
	if err := json.NewDecoder(resp.Body).Decode(&verOut); err != nil {
		t.Fatalf("decode version: %v", err)
	}
	resp.Body.Close()
	if verOut.DefaultKey != "B" {
		t.Fatalf("default key=%s", verOut.DefaultKey)
	}
	if verOut.Version != v3 {
		t.Fatalf("version=%s want %s", verOut.Version, v3)
	}

	// 5) PATCH target A labels with If-Match v3
	patchBody := `{"labels":["tenant=A","region=tokyo"]}`
	req, _ = http.NewRequest(http.MethodPatch, srv.URL+"/admin/targets/A", bytes.NewBufferString(patchBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", v3)
	resp, err = do(req, tk.AccessToken)
	if err != nil {
		t.Fatalf("patch A: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch status=%d", resp.StatusCode)
	}
	v4 := resp.Header.Get("ETag")
	if v4 == "" || v4 == v3 {
		t.Fatalf("etag v4 invalid")
	}
	resp.Body.Close()

	// 6) DELETE target B with If-Match v4
	req, _ = http.NewRequest(http.MethodDelete, srv.URL+"/admin/targets/B", nil)
	req.Header.Set("If-Match", v4)
	resp, err = do(req, tk.AccessToken)
	if err != nil {
		t.Fatalf("delete B: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Fatalf("delete status=%d", resp.StatusCode)
	}
	v5 := resp.Header.Get("ETag")
	if v5 == "" || v5 == v4 {
		t.Fatalf("etag v5 invalid")
	}
	resp.Body.Close()

	// 7) list by label
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/admin/targets?label=tenant=A&label=region=tokyo", nil)
	resp, err = do(req, tk.AccessToken)
	if err != nil {
		t.Fatalf("filter list: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("filter status=%d", resp.StatusCode)
	}
	if et := resp.Header.Get("ETag"); et != v5 {
		t.Fatalf("list etag=%s want %s", et, v5)
	}
	var lr listResp
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	resp.Body.Close()
	if len(lr.Items) != 1 || lr.Items[0].Key != "A" {
		t.Fatalf("items=%v", lr.Items)
	}
}
