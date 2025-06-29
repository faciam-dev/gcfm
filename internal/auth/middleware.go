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
var userKey = sm.UserKey()

// Middleware validates JWT tokens and stores the subject in context.
func Middleware(api huma.API, j *JWT) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, w := humachi.Unwrap(ctx)
		authHdr := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHdr, "Bearer ") {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "unauthorized")
			return
		}
		token := strings.TrimPrefix(authHdr, "Bearer ")
		claims, err := j.Validate(token)
		if err != nil {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "unauthorized")
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), userKey, claims.Subject))
		next(humachi.NewContext(ctx.Operation(), r, w))
	}
}

// UserFromContext returns the user subject stored in the context.
func UserFromContext(ctx context.Context) string { return sm.UserFromContext(ctx) }
