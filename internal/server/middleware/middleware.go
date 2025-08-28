package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/golang-jwt/jwt/v5"
)

// ctxKey is used for storing values in request context.
type ctxKey string

const (
	userKey   ctxKey = "user"
	claimsKey ctxKey = "claims"
)

// UserKey returns the context key used to store the user subject.
func UserKey() any { return userKey }

// ClaimsKey returns the context key used to store JWT claims.
func ClaimsKey() any { return claimsKey }

// JWT returns middleware that validates a bearer token signed with the given secret.
func JWT(api huma.API, secret string) func(huma.Context, func(huma.Context)) {
	key := []byte(secret)
	return func(ctx huma.Context, next func(huma.Context)) {
		r, w := humachi.Unwrap(ctx)
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			if err := huma.WriteErr(api, ctx, http.StatusUnauthorized, "unauthorized"); err != nil {
				// best effort; no logging available
			}
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
			if err := huma.WriteErr(api, ctx, http.StatusUnauthorized, "unauthorized"); err != nil {
				// best effort
			}
			return
		}
		claims, ok := t.Claims.(jwt.MapClaims)
		if !ok {
			if err := huma.WriteErr(api, ctx, http.StatusUnauthorized, "unauthorized"); err != nil {
				// best effort
			}
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
