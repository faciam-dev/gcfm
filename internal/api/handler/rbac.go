package handler

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
)

// RBACHandler provides role and user listing endpoints.
type RBACHandler struct {
	DB     *sql.DB
	Driver string
}

type listRolesOutput struct{ Body []schema.Role }

type listUsersOutput struct{ Body []schema.User }

func RegisterRBAC(api huma.API, h *RBACHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "listRoles",
		Method:      http.MethodGet,
		Path:        "/v1/roles",
		Summary:     "List roles",
		Tags:        []string{"RBAC"},
	}, h.listRoles)

	huma.Register(api, huma.Operation{
		OperationID: "listUsers",
		Method:      http.MethodGet,
		Path:        "/v1/users",
		Summary:     "List users",
		Tags:        []string{"RBAC"},
	}, h.listUsers)
}

func (h *RBACHandler) listRoles(ctx context.Context, _ *struct{}) (*listRolesOutput, error) {
	rows, err := h.DB.QueryContext(ctx, "SELECT id, name, comment FROM gcfm_roles ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := []schema.Role{}
	for rows.Next() {
		var r schema.Role
		if err := rows.Scan(&r.ID, &r.Name, &r.Comment); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range roles {
		var rpRows *sql.Rows
		if h.Driver == "postgres" {
			rpRows, err = h.DB.QueryContext(ctx, "SELECT path, method FROM gcfm_role_policies WHERE role_id=$1", roles[i].ID)
		} else {
			rpRows, err = h.DB.QueryContext(ctx, "SELECT path, method FROM gcfm_role_policies WHERE role_id=?", roles[i].ID)
		}
		if err != nil {
			return nil, err
		}
		for rpRows.Next() {
			var p schema.Policy
			if err := rpRows.Scan(&p.Path, &p.Method); err != nil {
				rpRows.Close()
				return nil, err
			}
			roles[i].Policies = append(roles[i].Policies, p)
		}
		rpRows.Close()
	}
	return &listRolesOutput{Body: roles}, nil
}

func (h *RBACHandler) listUsers(ctx context.Context, _ *struct{}) (*listUsersOutput, error) {
	rows, err := h.DB.QueryContext(ctx, `SELECT u.id, u.username, r.name FROM gcfm_users u LEFT JOIN gcfm_user_roles ur ON u.id=ur.user_id LEFT JOIN gcfm_roles r ON ur.role_id=r.id ORDER BY u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[uint64]*schema.User{}
	for rows.Next() {
		var id uint64
		var username, role sql.NullString
		if err := rows.Scan(&id, &username, &role); err != nil {
			return nil, err
		}
		u, ok := m[id]
		if !ok {
			m[id] = &schema.User{ID: id, Username: username.String}
			u = m[id]
		}
		if role.Valid {
			u.Roles = append(u.Roles, role.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	users := make([]schema.User, 0, len(m))
	for _, u := range m {
		users = append(users, *u)
	}
	return &listUsersOutput{Body: users}, nil
}
