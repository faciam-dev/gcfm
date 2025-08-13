package handler

import (
	"context"
	"database/sql"

	"github.com/casbin/casbin/v2"
	huma "github.com/faciam-dev/gcfm/internal/huma"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/tenant"
)

type AuthHandler struct {
	Enf    *casbin.Enforcer
	DB     *sql.DB
	Driver string
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
	user := middleware.UserFromContext(ctx)
	tid := tenant.FromContext(ctx)

	subjects := []string{user}
	if h.DB != nil {
		roles, err := h.rolesOfUser(ctx, user, tid)
		if err != nil {
			return nil, err
		}
		subjects = append(subjects, roles...)
	}

	caps := Capabilities{}
	for key, v := range capMatrix {
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

func (h *AuthHandler) rolesOfUser(ctx context.Context, user, tenantID string) ([]string, error) {
	if h.DB == nil {
		return nil, nil
	}
	isID := true
	for _, c := range user {
		if c < '0' || c > '9' {
			isID = false
			break
		}
	}
	var q string
	var args []any
	if h.Driver == "mysql" {
		if isID {
			q = `SELECT r.name FROM gcfm_user_roles ur JOIN gcfm_users u ON u.id=ur.user_id JOIN gcfm_roles r ON r.id=ur.role_id WHERE ur.user_id=? AND u.tenant_id=? ORDER BY r.name`
			args = []any{user, tenantID}
		} else {
			q = `SELECT r.name FROM gcfm_user_roles ur JOIN gcfm_users u ON u.id=ur.user_id JOIN gcfm_roles r ON r.id=ur.role_id WHERE u.username=? AND u.tenant_id=? ORDER BY r.name`
			args = []any{user, tenantID}
		}
	} else {
		if isID {
			q = `SELECT r.name FROM gcfm_user_roles ur JOIN gcfm_users u ON u.id=ur.user_id JOIN gcfm_roles r ON r.id=ur.role_id WHERE ur.user_id=$1 AND u.tenant_id=$2 ORDER BY r.name`
			args = []any{user, tenantID}
		} else {
			q = `SELECT r.name FROM gcfm_user_roles ur JOIN gcfm_users u ON u.id=ur.user_id JOIN gcfm_roles r ON r.id=ur.role_id WHERE u.username=$1 AND u.tenant_id=$2 ORDER BY r.name`
			args = []any{user, tenantID}
		}
	}
	rows, err := h.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		roles = append(roles, name)
	}
	return roles, rows.Err()
}
