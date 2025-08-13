package handler

import (
	"context"

	"github.com/casbin/casbin/v2"
	huma "github.com/faciam-dev/gcfm/internal/huma"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/tenant"
)

type AuthHandler struct {
	Enf *casbin.Enforcer
}

type Capabilities map[string]bool

type capsOut struct {
	Body struct {
		Capabilities Capabilities `json:"capabilities"`
	}
}

var capMatrix = map[string]struct{ Path, Method string }{
	// Users
	"users:list":   {"/v1/rbac/users", "GET"},
	"users:create": {"/v1/rbac/users", "POST"},

	// Roles
	"roles:list":          {"/v1/rbac/roles", "GET"},
	"roles:members:read":  {"/v1/rbac/roles/{id}/members", "GET"},
	"roles:members:write": {"/v1/rbac/roles/{id}/members", "PUT"},

	// Rules (Casbin policy UI)
	"rules:list":  {"/v1/authz/rules", "GET"},
	"rules:write": {"/v1/authz/rules", "POST"},

	// Audit
	"audit:list": {"/v1/audit-logs", "GET"},
	"audit:diff": {"/v1/audit-logs/{id}/diff", "GET"},

	// Metadata
	"metadata:tables": {"/v1/metadata/tables", "GET"},

	// Snapshots
	"snapshots:list":   {"/v1/snapshots", "GET"},
	"snapshots:create": {"/v1/snapshots", "POST"},
	"snapshots:apply":  {"/v1/snapshots/{ver}/apply", "POST"},

	// Databases
	"databases:list":   {"/v1/databases", "GET"},
	"databases:create": {"/v1/databases", "POST"},
	"databases:update": {"/v1/databases/{id}", "PUT"},
	"databases:delete": {"/v1/databases/{id}", "DELETE"},
	"databases:scan":   {"/v1/databases/{id}/scan", "POST"},
}

func RegisterAuthCaps(api huma.API, h *AuthHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "meCapabilities",
		Method:      "GET",
		Path:        "/v1/auth/me/capabilities",
		Summary:     "Get current user's capabilities",
		Tags:        []string{"Auth"},
	}, h.meCaps)
}

func (h *AuthHandler) meCaps(ctx context.Context, _ *struct{}) (*capsOut, error) {
	sub := middleware.UserFromContext(ctx)
	_ = tenant.FromContext(ctx)

	caps := Capabilities{}
	for key, v := range capMatrix {
		ok, _ := h.Enf.Enforce(sub, v.Path, v.Method)
		caps[key] = ok
	}
	out := &capsOut{}
	out.Body.Capabilities = caps
	return out, nil
}
