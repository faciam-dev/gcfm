package tenant

import "context"

// ctxKey is an unexported type for context keys defined in this package.
type ctxKey struct{}

// WithTenant stores the given tenant ID in the context.
func WithTenant(ctx context.Context, tid string) context.Context {
	return context.WithValue(ctx, ctxKey{}, tid)
}

// FromContext retrieves the tenant ID stored in the context. Empty string if missing.
func FromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}
