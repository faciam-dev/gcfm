package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	sm "github.com/faciam-dev/gcfm/internal/server/middleware"
)

// reuse the context key defined in server middleware
var (
	userKey   = sm.UserKey()
	claimsKey = sm.ClaimsKey()
)

// Middleware validates JWT tokens and stores the subject in context.
func Middleware(api huma.API, j *JWT) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, w := humachi.Unwrap(ctx)
		authHdr := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHdr, "Bearer ") {
			if err := huma.WriteErr(api, ctx, http.StatusUnauthorized, "unauthorized"); err != nil {
				// best effort
			}
			return
		}
		token := strings.TrimPrefix(authHdr, "Bearer ")
		claims, err := j.Validate(token)
		if err != nil {
			if err := huma.WriteErr(api, ctx, http.StatusUnauthorized, "unauthorized"); err != nil {
				// best effort
			}
			return
		}
		ctxWithUser := context.WithValue(r.Context(), userKey, claims.Subject)
		ctxWithClaims := context.WithValue(ctxWithUser, claimsKey, claims)
		r = r.WithContext(ctxWithClaims)
		next(humachi.NewContext(ctx.Operation(), r, w))
	}
}

// UserFromContext returns the user subject stored in the context.
func UserFromContext(ctx context.Context) string { return sm.UserFromContext(ctx) }

// ClaimsFromContext returns the JWT claims stored in context, if any.
func ClaimsFromContext(ctx context.Context) *Claims {
	if c, ok := ctx.Value(claimsKey).(*Claims); ok {
		return c
	}
	return nil
}
