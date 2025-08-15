package sdk

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc/metadata"
)

// norm normalizes string by trimming spaces, lowercasing, and allowing only safe characters.
func norm(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	// keep only alphanumeric and -_./
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '/' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func labelKV(k, v string) string {
	k, v = norm(k), norm(v)
	if k == "" || v == "" {
		return ""
	}
	return k + "=" + v
}

// HTTPLabelRules configures label extraction from HTTP headers.
type HTTPLabelRules struct {
	HeaderMap map[string]string // header name -> label name
	Fixed     map[string]string
}

// LabelsFromHTTP extracts labels from an *http.Request based on rules.
func LabelsFromHTTP(r *http.Request, rules HTTPLabelRules) []string {
	var out []string
	for h, lab := range rules.HeaderMap {
		hv := r.Header.Get(h)
		if hv == "" {
			continue
		}
		if lv := labelKV(lab, hv); lv != "" {
			out = append(out, lv, lab)
		}
	}
	for k, v := range rules.Fixed {
		if lv := labelKV(k, v); lv != "" {
			out = append(out, lv, k)
		}
	}
	return uniq(out)
}

// GRPCLabelRules configures label extraction from gRPC metadata.
type GRPCLabelRules struct {
	MetaMap map[string]string // metadata key -> label name
	Fixed   map[string]string
}

// LabelsFromGRPC extracts labels from gRPC metadata within a context.
func LabelsFromGRPC(ctx context.Context, rules GRPCLabelRules) []string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.MD{}
	}
	var out []string
	for m, lab := range rules.MetaMap {
		vals := md.Get(m)
		if len(vals) == 0 {
			continue
		}
		v := vals[0]
		if lv := labelKV(lab, v); lv != "" {
			out = append(out, lv, lab)
		}
	}
	for k, v := range rules.Fixed {
		if lv := labelKV(k, v); lv != "" {
			out = append(out, lv, k)
		}
	}
	return uniq(out)
}

// JWTLabelRules configures label extraction from JWT claims.
type JWTLabelRules struct {
	ClaimMap map[string]string // claim name -> label name
	Fixed    map[string]string
}

// LabelsFromJWT extracts labels from a claims map based on rules.
func LabelsFromJWT(claims map[string]any, rules JWTLabelRules) []string {
	var out []string
	for c, lab := range rules.ClaimMap {
		if v, ok := claims[c]; ok {
			if s, ok := v.(string); ok {
				if lv := labelKV(lab, s); lv != "" {
					out = append(out, lv, lab)
				}
			}
		}
	}
	for k, v := range rules.Fixed {
		if lv := labelKV(k, v); lv != "" {
			out = append(out, lv, k)
		}
	}
	return uniq(out)
}

// CtxValueRules configures label extraction from context values.
type CtxValueRules struct {
	KeyMap map[any]string // context key -> label name
	Fixed  map[string]string
}

// LabelsFromContext extracts labels from context values according to rules.
func LabelsFromContext(ctx context.Context, rules CtxValueRules) []string {
	var out []string
	for key, lab := range rules.KeyMap {
		if v := ctx.Value(key); v != nil {
			if s, ok := v.(string); ok {
				if lv := labelKV(lab, s); lv != "" {
					out = append(out, lv, lab)
				}
			}
		}
	}
	for k, v := range rules.Fixed {
		if lv := labelKV(k, v); lv != "" {
			out = append(out, lv, k)
		}
	}
	return uniq(out)
}

func uniq(xs []string) []string {
	m := make(map[string]struct{}, len(xs))
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if _, ok := m[x]; ok {
			continue
		}
		m[x] = struct{}{}
		out = append(out, x)
	}
	return out
}

// AutoLabelResolverOptions configures AutoLabelResolver sources.
type AutoLabelResolverOptions struct {
	HTTP *HTTPLabelRules
	GRPC *GRPCLabelRules
	JWT  *JWTLabelRules
	Ctx  *CtxValueRules
	Hint *SelectionHint
}

// AutoLabelResolver builds a TargetResolverV2 that aggregates labels from
// various sources and turns them into a Query.
func AutoLabelResolver(opts AutoLabelResolverOptions) TargetResolverV2 {
	return func(ctx context.Context) (TargetDecision, bool) {
		var labels []string
		if opts.HTTP != nil {
			if r, ok := ctx.Value(httpRequestKey{}).(*http.Request); ok && r != nil {
				labels = append(labels, LabelsFromHTTP(r, *opts.HTTP)...)
			}
		}
		if opts.GRPC != nil {
			labels = append(labels, LabelsFromGRPC(ctx, *opts.GRPC)...)
		}
		if opts.JWT != nil {
			if cl, ok := ctx.Value(jwtClaimsKey{}).(map[string]any); ok && cl != nil {
				labels = append(labels, LabelsFromJWT(cl, *opts.JWT)...)
			}
		}
		if opts.Ctx != nil {
			labels = append(labels, LabelsFromContext(ctx, *opts.Ctx)...)
		}
		labels = uniq(labels)
		if len(labels) == 0 {
			return TargetDecision{}, false
		}
		q := QueryFromLabels(labels)
		return TargetDecision{Query: &q, Hint: opts.Hint}, true
	}
}

type httpRequestKey struct{}

// WithHTTPRequest stores *http.Request in context for AutoLabelResolver.
func WithHTTPRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, httpRequestKey{}, r)
}

type jwtClaimsKey struct{}

// WithJWTClaims stores JWT claims in context for AutoLabelResolver.
func WithJWTClaims(ctx context.Context, claims map[string]any) context.Context {
	return context.WithValue(ctx, jwtClaimsKey{}, claims)
}
