package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/golang-jwt/jwt/v5"
)

// ctxKey is used for storing values in request context.
type ctxKey string

const userKey ctxKey = "user"

// UserKey returns the context key used to store the user subject.
func UserKey() any { return userKey }

// JWT returns middleware that validates a bearer token signed with the given secret.
func JWT(api huma.API, secret string) func(huma.Context, func(huma.Context)) {
	key := []byte(secret)
	return func(ctx huma.Context, next func(huma.Context)) {
		r, w := humachi.Unwrap(ctx)
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "unauthorized")
			return
		}
		tokenString := strings.TrimPrefix(auth, "Bearer ")
		t, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return key, nil
		})
		if err != nil || !t.Valid {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "unauthorized")
			return
		}
		claims, ok := t.Claims.(jwt.MapClaims)
		if !ok {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "unauthorized")
			return
		}
		sub, _ := claims["sub"].(string)
		r = r.WithContext(context.WithValue(r.Context(), userKey, sub))
		next(humachi.NewContext(ctx.Operation(), r, w))
	}
}

// UserFromContext returns the user subject stored in the context.
func UserFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(userKey).(string); ok {
		return v
	}
	return ""
}

// RBAC returns middleware that enforces access using the provided enforcer.
func RBAC(e *casbin.Enforcer) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, w := humachi.Unwrap(ctx)
		sub := UserFromContext(r.Context())
		obj := r.URL.Path
		act := r.Method
		ok, err := e.Enforce(sub, obj, act)
		if err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(ctx)
	}
}
