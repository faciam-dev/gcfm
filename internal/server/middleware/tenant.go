package middleware

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"

	"github.com/faciam-dev/gcfm/internal/tenant"
)

// claimsKey is the context key used by the auth middleware to store JWT claims.
// This mirrors the implementation there so that this middleware can read them.
// ExtractTenant obtains the tenant ID from the X-Tenant-ID header or JWT claim
// "tid". A missing tenant results in 400.
func ExtractTenant(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, w := humachi.Unwrap(ctx)
		tid := r.Header.Get("X-Tenant-ID")
		if tid == "" {
			if claims, ok := r.Context().Value(ClaimsKey()).(interface{ GetTenantID() string }); ok {
				tid = claims.GetTenantID()
			}
		}
		if tid == "" {
			huma.WriteErr(api, ctx, 400, "missing tenant identifier: set X-Tenant-ID header or tid claim")
			return
		}
		r = r.WithContext(tenant.WithTenant(r.Context(), tid))
		next(humachi.NewContext(ctx.Operation(), r, w))
	}
}
