package sdk

import (
	"context"
	"database/sql"
	"net/http"
	"reflect"
	"sort"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestLabelsFromHTTP(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("X-Tenant-ID", "Acme")
	r.Header.Set("X-Region", "Tokyo")
	rules := HTTPLabelRules{
		HeaderMap: map[string]string{
			"X-Tenant-ID": "tenant",
			"X-Region":    "region",
		},
		Fixed: map[string]string{"env": "Prod"},
	}
	labels := LabelsFromHTTP(r, rules)
	sort.Strings(labels)
	expect := []string{"env", "env=prod", "region", "region=tokyo", "tenant", "tenant=acme"}
	if !reflect.DeepEqual(labels, expect) {
		t.Fatalf("http labels = %v", labels)
	}
}

func TestLabelsFromGRPC(t *testing.T) {
	md := metadata.New(map[string]string{
		"tenant":   "Acme",
		"x-region": "Tokyo",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	rules := GRPCLabelRules{
		MetaMap: map[string]string{
			"tenant":   "tenant",
			"x-region": "region",
		},
		Fixed: map[string]string{"env": "Prod"},
	}
	labels := LabelsFromGRPC(ctx, rules)
	sort.Strings(labels)
	expect := []string{"env", "env=prod", "region", "region=tokyo", "tenant", "tenant=acme"}
	if !reflect.DeepEqual(labels, expect) {
		t.Fatalf("grpc labels = %v", labels)
	}
}

func TestLabelsFromJWT(t *testing.T) {
	claims := map[string]any{
		"tenant": "Acme",
		"region": "Tokyo",
	}
	rules := JWTLabelRules{
		ClaimMap: map[string]string{
			"tenant": "tenant",
			"region": "region",
		},
		Fixed: map[string]string{"env": "Prod"},
	}
	labels := LabelsFromJWT(claims, rules)
	sort.Strings(labels)
	expect := []string{"env", "env=prod", "region", "region=tokyo", "tenant", "tenant=acme"}
	if !reflect.DeepEqual(labels, expect) {
		t.Fatalf("jwt labels = %v", labels)
	}
}

func TestLabelsFromContext(t *testing.T) {
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("tenant"), "Acme")
	ctx = context.WithValue(ctx, ctxKey("region"), "Tokyo")
	rules := CtxValueRules{
		KeyMap: map[any]string{
			ctxKey("tenant"): "tenant",
			ctxKey("region"): "region",
		},
		Fixed: map[string]string{"env": "Prod"},
	}
	labels := LabelsFromContext(ctx, rules)
	sort.Strings(labels)
	expect := []string{"env", "env=prod", "region", "region=tokyo", "tenant", "tenant=acme"}
	if !reflect.DeepEqual(labels, expect) {
		t.Fatalf("context labels = %v", labels)
	}
}

func TestQueryFromLabels(t *testing.T) {
	labels := []string{"region=tokyo", "tenant=acme", "env=prod", "gpu"}
	q := QueryFromLabels(labels)
	if len(q.AND) != 4 {
		t.Fatalf("AND len=%d", len(q.AND))
	}
	want := map[string]bool{
		"region=tokyo": true,
		"tenant=acme":  true,
		"env=prod":     true,
		"gpu":          true,
	}
	for _, e := range q.AND {
		switch v := e.(type) {
		case EqExpr:
			if !want[v.Label+"="+v.Value] {
				t.Fatalf("unexpected eq %s=%s", v.Label, v.Value)
			}
			delete(want, v.Label+"="+v.Value)
		case HasExpr:
			if !want[v.Label] {
				t.Fatalf("unexpected has %s", v.Label)
			}
			delete(want, v.Label)
		default:
			t.Fatalf("unexpected expr %T", v)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing exprs: %v", want)
	}
}

func TestAutoLabelResolver(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("X-Tenant-ID", "acme")
	req.Header.Set("X-Region", "tokyo")
	ctx := WithHTTPRequest(context.Background(), req)
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-env", "prod"))
	ctx = WithJWTClaims(ctx, map[string]any{"gpu": "t4"})
	type ctxKey string
	ctx = context.WithValue(ctx, ctxKey("tier"), "gold")

	opts := AutoLabelResolverOptions{
		HTTP: &HTTPLabelRules{HeaderMap: map[string]string{"X-Tenant-ID": "tenant", "X-Region": "region"}},
		GRPC: &GRPCLabelRules{MetaMap: map[string]string{"x-env": "env"}},
		JWT:  &JWTLabelRules{ClaimMap: map[string]string{"gpu": "gpu"}},
		Ctx:  &CtxValueRules{KeyMap: map[any]string{ctxKey("tier"): "tier"}},
	}
	dec, ok := AutoLabelResolver(opts)(ctx)
	if !ok || dec.Query == nil {
		t.Fatalf("resolver failed: ok=%v, query=%v", ok, dec.Query)
	}
	got := map[string]struct{}{}
	for _, e := range dec.Query.AND {
		switch v := e.(type) {
		case EqExpr:
			got[v.Label+"="+v.Value] = struct{}{}
		case HasExpr:
			got[v.Label] = struct{}{}
		}
	}
	expect := []string{
		"tenant=acme", "tenant",
		"region=tokyo", "region",
		"env=prod", "env",
		"gpu=t4", "gpu",
		"tier=gold", "tier",
	}
	for _, s := range expect {
		if _, ok := got[s]; !ok {
			t.Fatalf("missing %s", s)
		}
	}

	// empty context -> no decision
	if _, ok := AutoLabelResolver(opts)(context.Background()); ok {
		t.Fatalf("expected no decision on empty context")
	}
}

func TestHTTPToQueryAndSelectPrefer(t *testing.T) {
	ctx := context.Background()
	reg := NewHotReloadRegistry(nil)
	must := func(err error) {
		if err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	must(reg.Register(ctx, "tenant:A", TargetConfig{DB: new(sql.DB), Driver: "sqlite3", Schema: "A", Labels: []string{"region=tokyo", "env=prod", "primary=true"}}, nil))
	must(reg.Register(ctx, "tenant:B", TargetConfig{DB: new(sql.DB), Driver: "sqlite3", Schema: "B", Labels: []string{"region=tokyo", "env=prod"}}, nil))

	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("X-Region", "tokyo")
	req.Header.Set("X-Env", "prod")
	ctx = WithHTTPRequest(context.Background(), req)

	opts := AutoLabelResolverOptions{
		HTTP: &HTTPLabelRules{HeaderMap: map[string]string{"X-Region": "region", "X-Env": "env"}},
	}
	dec, ok := AutoLabelResolver(opts)(ctx)
	if !ok || dec.Query == nil {
		t.Fatalf("resolver failed: %v %v", ok, dec.Query)
	}

	got := map[string]bool{}
	for _, e := range dec.Query.AND {
		switch v := e.(type) {
		case EqExpr:
			got[v.Label+"="+v.Value] = true
		case HasExpr:
			got[v.Label] = true
		}
	}
	expect := []string{"region=tokyo", "region", "env=prod", "env"}
	for _, s := range expect {
		if !got[s] {
			t.Fatalf("missing %s in query", s)
		}
	}

	keys := reg.FindByQuery(*dec.Query)
	sort.Strings(keys)
	want := []string{"tenant:A", "tenant:B"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("FindByQuery = %v", keys)
	}

	svc := &service{targets: reg, stratDefault: SelectFirst}
	k, ok := svc.chooseOne(keys, &SelectionHint{Strategy: SelectPreferLabel, PreferLabel: "primary=true"})
	if !ok || k != "tenant:A" {
		t.Fatalf("chooseOne got %s", k)
	}
}

func TestGRPCJWTCombination(t *testing.T) {
	ctx := context.Background()
	reg := NewHotReloadRegistry(nil)
	must := func(err error) {
		if err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	must(reg.Register(ctx, "tenant:acme", TargetConfig{DB: new(sql.DB), Driver: "sqlite3", Labels: []string{"tenant=acme", "region=tokyo"}}, nil))
	must(reg.Register(ctx, "tenant:beta", TargetConfig{DB: new(sql.DB), Driver: "sqlite3", Labels: []string{"tenant=beta", "region=tokyo"}}, nil))

	md := metadata.Pairs("x-region", "tokyo")
	ctx = metadata.NewIncomingContext(context.Background(), md)
	ctx = WithJWTClaims(ctx, map[string]any{"tid": "acme"})

	opts := AutoLabelResolverOptions{
		GRPC: &GRPCLabelRules{MetaMap: map[string]string{"x-region": "region"}},
		JWT:  &JWTLabelRules{ClaimMap: map[string]string{"tid": "tenant"}},
	}
	dec, ok := AutoLabelResolver(opts)(ctx)
	if !ok || dec.Query == nil {
		t.Fatalf("resolver failed")
	}

	keys := reg.FindByQuery(*dec.Query)
	if len(keys) != 1 || keys[0] != "tenant:acme" {
		t.Fatalf("FindByQuery = %v", keys)
	}
}
