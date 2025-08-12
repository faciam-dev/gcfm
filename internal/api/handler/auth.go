package handler

import (
	"context"
	"net/http"

	"github.com/casbin/casbin/v2"
	huma "github.com/faciam-dev/gcfm/internal/huma"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
)

type AuthHandler struct {
	Enforcer *casbin.Enforcer
}

type Capabilities map[string]bool

type capsOutput struct {
	Body struct {
		Capabilities Capabilities `json:"capabilities"`
	}
}

var capMatrix = map[string]struct{ Path, Method string }{
	"users:list":      {"/v1/rbac/users", http.MethodGet},
	"users:create":    {"/v1/rbac/users", http.MethodPost},
	"roles:list":      {"/v1/rbac/roles", http.MethodGet},
	"roles:members":   {"/v1/rbac/roles/{id}/members", http.MethodGet},
	"rules:list":      {"/v1/authz/rules", http.MethodGet},
	"rules:write":     {"/v1/authz/rules", http.MethodPost},
	"audit:list":      {"/v1/audit-logs", http.MethodGet},
	"audit:diff":      {"/v1/audit-logs/{id}/diff", http.MethodGet},
	"metadata:tables": {"/v1/metadata/tables", http.MethodGet},
}

func RegisterAuth(api huma.API, h *AuthHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "meCapabilities",
		Method:      http.MethodGet,
		Path:        "/v1/auth/me/capabilities",
		Summary:     "Get user capabilities",
		Tags:        []string{"Auth"},
	}, h.meCapabilities)
}

func (h *AuthHandler) meCapabilities(ctx context.Context, _ *struct{}) (*capsOutput, error) {
	sub := middleware.UserFromContext(ctx)

	caps := Capabilities{}
	for k, v := range capMatrix {
		allowed, _ := h.Enforcer.Enforce(sub, v.Path, v.Method)
		caps[k] = allowed
	}
	out := &capsOutput{}
	out.Body.Capabilities = caps
	return out, nil
}
