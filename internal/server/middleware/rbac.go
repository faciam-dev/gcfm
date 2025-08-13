package middleware

import (
	"context"
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
)

type RoleResolver func(ctx context.Context, user string) ([]string, error)

// RBAC enforces access where either the user or any of their roles is allowed.
func RBAC(enf *casbin.Enforcer, resolve RoleResolver) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, w := humachi.Unwrap(ctx)
		sub := UserFromContext(r.Context())
		obj := r.URL.Path
		act := r.Method

		subjects := []string{sub}
		if resolve != nil {
			if roles, err := resolve(r.Context(), sub); err == nil && len(roles) > 0 {
				subjects = append(subjects, roles...)
			}
		}

		for _, s := range subjects {
			if ok, _ := enf.Enforce(s, obj, act); ok {
				next(ctx)
				return
			}
		}
		http.Error(w, "forbidden", http.StatusForbidden)
	}
}
