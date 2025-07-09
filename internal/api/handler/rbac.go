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
	defer func() {
		_ = rows.Close()
	}()
	roles := []schema.Role{}
	for rows.Next() {
		var r schema.Role
		var comment sql.NullString
		if err := rows.Scan(&r.ID, &r.Name, &comment); err != nil {
			return nil, err
		}
		if comment.Valid {
			r.Comment = comment.String
		}
		roles = append(roles, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var pRows *sql.Rows
	pRows, err = h.DB.QueryContext(ctx, "SELECT role_id, path, method FROM gcfm_role_policies")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = pRows.Close()
	}()

	byRole := make(map[int64][]schema.Policy)
	for pRows.Next() {
		var id int64
		var p schema.Policy
		if err := pRows.Scan(&id, &p.Path, &p.Method); err != nil {
			return nil, err
		}
		byRole[id] = append(byRole[id], p)
	}
	if err := pRows.Err(); err != nil {
		return nil, err
	}
	for i := range roles {
		roles[i].Policies = byRole[roles[i].ID]
	}
	return &listRolesOutput{Body: roles}, nil
}

func (h *RBACHandler) listUsers(ctx context.Context, _ *struct{}) (*listUsersOutput, error) {
	rows, err := h.DB.QueryContext(ctx, `SELECT u.id, u.username, r.name FROM gcfm_users u LEFT JOIN gcfm_user_roles ur ON u.id=ur.user_id LEFT JOIN gcfm_roles r ON ur.role_id=r.id ORDER BY u.id`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
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
