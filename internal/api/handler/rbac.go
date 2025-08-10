package handler

import (
	"context"
	"database/sql"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"

	"github.com/faciam-dev/gcfm/internal/api/schema"
	huma "github.com/faciam-dev/gcfm/internal/huma"
	"github.com/faciam-dev/gcfm/internal/rbac"
)

// RBACHandler provides role and user listing endpoints.
type RBACHandler struct {
	DB     *sql.DB
	Driver string
}

type listRolesOutput struct{ Body []schema.Role }

type listUsersOutput struct{ Body []schema.User }

var roleNamePattern = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

func RegisterRBAC(api huma.API, h *RBACHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "listRoles",
		Method:      http.MethodGet,
		Path:        "/v1/rbac/roles",
		Summary:     "List roles",
		Tags:        []string{"RBAC"},
	}, h.listRoles)

	huma.Register(api, huma.Operation{
		OperationID:   "createRole",
		Method:        http.MethodPost,
		Path:          "/v1/rbac/roles",
		Summary:       "Create role",
		Tags:          []string{"RBAC"},
		Errors:        []int{http.StatusConflict, http.StatusUnprocessableEntity},
		DefaultStatus: http.StatusCreated,
	}, h.createRole)

	huma.Register(api, huma.Operation{
		OperationID:   "deleteRole",
		Method:        http.MethodDelete,
		Path:          "/v1/rbac/roles/{id}",
		Summary:       "Delete role",
		Tags:          []string{"RBAC"},
		Errors:        []int{http.StatusConflict},
		DefaultStatus: http.StatusNoContent,
	}, h.deleteRole)

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

type createRoleInput struct {
	Body struct {
		Name    string  `json:"name"`
		Comment *string `json:"comment,omitempty"`
	}
}

type roleOutput struct{ Body schema.Role }

type roleIDParam struct {
	ID int64 `path:"id"`
}

func (h *RBACHandler) createRole(ctx context.Context, in *createRoleInput) (*roleOutput, error) {
	if !roleNamePattern.MatchString(in.Body.Name) {
		return nil, huma.Error422("name", "must match ^[a-z0-9_-]{1,64}$")
	}
	var comment interface{}
	if in.Body.Comment != nil && *in.Body.Comment != "" {
		comment = *in.Body.Comment
	}
	var id int64
	if h.Driver == "postgres" {
		err := h.DB.QueryRowContext(ctx, "INSERT INTO gcfm_roles(name, comment) VALUES($1, $2) RETURNING id", in.Body.Name, comment).Scan(&id)
		if err != nil {
			if isDuplicateErr(err) {
				return nil, huma.Error409Conflict("role already exists")
			}
			return nil, err
		}
	} else {
		res, err := h.DB.ExecContext(ctx, "INSERT INTO gcfm_roles(name, comment) VALUES(?, ?)", in.Body.Name, comment)
		if err != nil {
			if isDuplicateErr(err) {
				return nil, huma.Error409Conflict("role already exists")
			}
			return nil, err
		}
		id, err = res.LastInsertId()
		if err != nil {
			return nil, err
		}
	}
	rbac.ReloadEnforcer(ctx, h.DB)
	r := schema.Role{ID: id, Name: in.Body.Name}
	if in.Body.Comment != nil {
		r.Comment = *in.Body.Comment
	}
	return &roleOutput{Body: r}, nil
}

func (h *RBACHandler) deleteRole(ctx context.Context, p *roleIDParam) (*struct{}, error) {
	var q string
	if h.Driver == "postgres" {
		q = "SELECT COUNT(*) FROM gcfm_user_roles WHERE role_id=$1"
	} else {
		q = "SELECT COUNT(*) FROM gcfm_user_roles WHERE role_id=?"
	}
	var cnt int
	if err := h.DB.QueryRowContext(ctx, q, p.ID).Scan(&cnt); err != nil {
		return nil, err
	}
	if cnt > 0 {
		return nil, huma.Error409Conflict("role has users")
	}
	if h.Driver == "postgres" {
		q = "SELECT COUNT(*) FROM gcfm_role_policies WHERE role_id=$1"
	} else {
		q = "SELECT COUNT(*) FROM gcfm_role_policies WHERE role_id=?"
	}
	if err := h.DB.QueryRowContext(ctx, q, p.ID).Scan(&cnt); err != nil {
		return nil, err
	}
	if cnt > 0 {
		return nil, huma.Error409Conflict("role has policies")
	}
	if h.Driver == "postgres" {
		q = "DELETE FROM gcfm_roles WHERE id=$1"
	} else {
		q = "DELETE FROM gcfm_roles WHERE id=?"
	}
	if _, err := h.DB.ExecContext(ctx, q, p.ID); err != nil {
		return nil, err
	}
	rbac.ReloadEnforcer(ctx, h.DB)
	return &struct{}{}, nil
}

func isDuplicateErr(err error) bool {
	if err == nil {
		return false
	}
	if me, ok := err.(*mysql.MySQLError); ok {
		return me.Number == 1062
	}
	if pe, ok := err.(*pq.Error); ok {
		return string(pe.Code) == "23505"
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "conflict")
}
