package sdk

import "context"

// TenantIDKey is the context key used by WithTenantID.
type TenantIDKey struct{}

var tenantKey TenantIDKey

// WithTenantID attaches a tenant ID to the context.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey, tenantID)
}

// TenantResolverFromPrefix returns a TargetResolver that looks up tenant ID with given prefix.
func TenantResolverFromPrefix(prefix string) TargetResolver {
	return func(ctx context.Context) (string, bool) {
		v := ctx.Value(tenantKey)
		id, _ := v.(string)
		if id == "" {
			return "", false
		}
		return prefix + id, true
	}
}
