package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/faciam-dev/gcfm/internal/auth"
	sm "github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/pkg/tenant"
)

type tenantResp struct {
	Body struct {
		Tenant string `json:"tenant"`
	}
}

func newAPI(secret string) huma.API {
	r := chi.NewRouter()
	api := humachi.New(r, huma.DefaultConfig("test", "1.0"))
	if secret != "" {
		jwtH := auth.NewJWT(secret, time.Minute)
		api.UseMiddleware(auth.Middleware(api, jwtH))
	}
	api.UseMiddleware(sm.ExtractTenant(api))
	type in struct{}
	huma.Register(api, huma.Operation{
		OperationID:   "getTenant",
		Method:        http.MethodGet,
		Path:          "/tenant",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, _ *in) (*tenantResp, error) {
		var r tenantResp
		r.Body.Tenant = tenant.FromContext(ctx)
		return &r, nil
	})
	return api
}

func TestExtractTenant_Header(t *testing.T) {
	api := newAPI("")
	req := httptest.NewRequest(http.MethodGet, "/tenant", nil)
	req.Header.Set("X-Tenant-ID", "t1")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	api.Adapter().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	t.Logf("status %d body %s", w.Code, w.Body.String())
	var resp struct {
		Tenant string `json:"tenant"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	if resp.Tenant != "t1" {
		t.Fatalf("tenant %s", resp.Tenant)
	}
}

func TestExtractTenant_JWT(t *testing.T) {
	secret := "secret"
	api := newAPI(secret)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "u1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
		TenantID: "t2",
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/tenant", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	api.Adapter().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	t.Logf("status %d body %s", w.Code, w.Body.String())
	var resp struct {
		Tenant string `json:"tenant"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	if resp.Tenant != "t2" {
		t.Fatalf("tenant %s", resp.Tenant)
	}
}

func TestExtractTenant_Missing(t *testing.T) {
	api := newAPI("")
	req := httptest.NewRequest(http.MethodGet, "/tenant", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	api.Adapter().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
}
