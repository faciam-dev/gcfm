package handler

import (
	"context"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"

	"github.com/faciam-dev/gcfm/internal/server/middleware"
)

func TestMeCapabilities(t *testing.T) {
	m := model.NewModel()
	m.AddDef("r", "r", "sub, obj, act")
	m.AddDef("p", "p", "sub, obj, act")
	m.AddDef("e", "e", "some(where (p.eft == allow))")
	m.AddDef("m", "m", "r.sub == p.sub && keyMatch2(r.obj, p.obj) && r.act == p.act")

	e, err := casbin.NewEnforcer(m)
	if err != nil {
		t.Fatalf("enforcer: %v", err)
	}
	if _, err := e.AddPolicy("tester", "/v1/rbac/users", "GET"); err != nil {
		t.Fatalf("policy: %v", err)
	}

	h := &AuthHandler{Enforcer: e}
	ctx := context.WithValue(context.Background(), middleware.UserKey(), "tester")
	out, err := h.meCapabilities(ctx, nil)
	if err != nil {
		t.Fatalf("meCapabilities: %v", err)
	}
	if !out.Body.Capabilities["users:list"] {
		t.Fatalf("expected users:list true")
	}
	if out.Body.Capabilities["roles:list"] {
		t.Fatalf("expected roles:list false")
	}
}
