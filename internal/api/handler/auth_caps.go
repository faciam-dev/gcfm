package handler

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/casbin/casbin/v2"
	huma "github.com/faciam-dev/gcfm/internal/huma"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/server/roles"
	"github.com/faciam-dev/gcfm/pkg/tenant"
	pkgutil "github.com/faciam-dev/gcfm/pkg/util"
)

type AuthHandler struct {
	Enf         *casbin.Enforcer
	DB          *sql.DB
	Driver      string
	TablePrefix string
}

type Capabilities map[string]bool

type capsOut struct {
	Body struct {
		Capabilities Capabilities `json:"capabilities"`
	}
}

// Capability defines the path and HTTP method associated with a capability key.
type Capability struct{ Path, Method string }

// CapMatrix maps capability keys to their required path and HTTP method.
var CapMatrix = map[string]Capability{
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

	// Custom Fields
	"custom_fields:list":   {"/v1/custom-fields", "GET"},
	"custom_fields:create": {"/v1/custom-fields", "POST"},
	"custom_fields:update": {"/v1/custom-fields", "PUT"},
	"custom_fields:delete": {"/v1/custom-fields", "DELETE"},

	// Plugins & Widgets
	"plugins:list":  {"/v1/plugins", "GET"},
	"plugins:write": {"/v1/plugins", "POST"},
	"widgets:list":  {"/v1/metadata/widgets", "GET"},
	"widgets:write": {"/v1/metadata/widgets/*", "PATCH"},

	// Snapshots
	"snapshots:list":   {"/v1/snapshots", "GET"},
	"snapshots:create": {"/v1/snapshots", "POST"},
	"snapshots:apply":  {"/v1/snapshots/{ver}/apply", "POST"},

	// Databases
	"databases:list":         {"/v1/databases", "GET"},
	"databases:create":       {"/v1/databases", "POST"},
	"databases:update":       {"/v1/databases/{id}", "PUT"},
	"databases:delete":       {"/v1/databases/{id}", "DELETE"},
	"databases:scan":         {"/v1/databases/{id}/scan", "POST"},
	"databases:capabilities": {"/v1/databases/{id}/capabilities", "GET"},

	// Targets
	"targets:list":         {"/admin/targets", "GET"},
	"targets:create":       {"/admin/targets", "POST"},
	"targets:update":       {"/admin/targets/{key}", "PUT"},
	"targets:patch":        {"/admin/targets/{key}", "PATCH"},
	"targets:delete":       {"/admin/targets/{key}", "DELETE"},
	"targets:set-default":  {"/admin/targets/{key}/default", "POST"},
	"targets:get-version":  {"/admin/targets/version", "GET"},
	"targets:bump-version": {"/admin/targets/version/bump", "POST"},
}

// CapabilityByKey looks up the capability definition by key.
func CapabilityByKey(key string) (Capability, bool) {
	v, ok := CapMatrix[key]
	return v, ok
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
	user := middleware.UserFromContext(ctx)
	tid := tenant.FromContext(ctx)

	subjects := []string{user}
	if h.DB != nil {
		dialect := pkgutil.DialectFromDriver(h.Driver)
		if _, ok := dialect.(pkgutil.UnsupportedDialect); ok {
			return nil, huma.NewError(http.StatusInternalServerError, "unsupported driver", nil)
		}
		rs, err := roles.OfUser(ctx, h.DB, dialect, h.TablePrefix, user, tid)
		if err != nil {
			return nil, err
		}
		subjects = append(subjects, rs...)
	}

	caps := Capabilities{}
	for key, v := range CapMatrix {
		allow := false
		for _, s := range subjects {
			if ok, _ := h.Enf.Enforce(s, v.Path, v.Method); ok {
				allow = true
				break
			}
		}
		caps[key] = allow
	}
	out := &capsOut{}
	out.Body.Capabilities = caps
	return out, nil
}
