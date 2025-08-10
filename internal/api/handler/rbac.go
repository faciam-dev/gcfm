package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"

	"github.com/faciam-dev/gcfm/internal/api/schema"
	audit "github.com/faciam-dev/gcfm/internal/customfield/audit"
	huma "github.com/faciam-dev/gcfm/internal/huma"
	"github.com/faciam-dev/gcfm/internal/rbac"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/tenant"
)

// RBACHandler provides role and user listing endpoints.
type RBACHandler struct {
	DB       *sql.DB
	Driver   string
	Recorder *audit.Recorder
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
		Path:        "/v1/rbac/users",
		Summary:     "List users",
		Tags:        []string{"RBAC"},
	}, h.listUsers)

	huma.Register(api, huma.Operation{
		OperationID: "getRoleMembers",
		Method:      http.MethodGet,
		Path:        "/v1/rbac/roles/{id}/members",
		Summary:     "Get role members",
		Tags:        []string{"RBAC"},
	}, h.getRoleMembers)

	huma.Register(api, huma.Operation{
		OperationID: "putRoleMembers",
		Method:      http.MethodPut,
		Path:        "/v1/rbac/roles/{id}/members",
		Summary:     "Replace role members",
		Tags:        []string{"RBAC"},
	}, h.putRoleMembers)

	huma.Register(api, huma.Operation{
		OperationID: "getRolePolicies",
		Method:      http.MethodGet,
		Path:        "/v1/rbac/roles/{id}/policies",
		Summary:     "List role policies",
		Tags:        []string{"RBAC"},
	}, h.getRolePolicies)

	huma.Register(api, huma.Operation{
		OperationID:   "addRolePolicy",
		Method:        http.MethodPost,
		Path:          "/v1/rbac/roles/{id}/policies",
		Summary:       "Add role policy",
		Tags:          []string{"RBAC"},
		Errors:        []int{http.StatusConflict, http.StatusUnprocessableEntity},
		DefaultStatus: http.StatusCreated,
	}, h.addRolePolicy)

	huma.Register(api, huma.Operation{
		OperationID:   "deleteRolePolicy",
		Method:        http.MethodDelete,
		Path:          "/v1/rbac/roles/{id}/policies",
		Summary:       "Delete role policy",
		Tags:          []string{"RBAC"},
		DefaultStatus: http.StatusNoContent,
	}, h.deleteRolePolicy)
}

func (h *RBACHandler) listRoles(ctx context.Context, _ *struct{}) (*listRolesOutput, error) {
	tid := tenant.FromContext(ctx)
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
	var cRows *sql.Rows
	var cq string
	if h.Driver == "postgres" {
		cq = "SELECT ur.role_id, COUNT(*) FROM gcfm_user_roles ur JOIN gcfm_users u ON ur.user_id=u.id WHERE u.tenant_id=$1 GROUP BY ur.role_id"
		cRows, err = h.DB.QueryContext(ctx, cq, tid)
	} else {
		cq = "SELECT ur.role_id, COUNT(*) FROM gcfm_user_roles ur JOIN gcfm_users u ON ur.user_id=u.id WHERE u.tenant_id=? GROUP BY ur.role_id"
		cRows, err = h.DB.QueryContext(ctx, cq, tid)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = cRows.Close() }()
	counts := make(map[int64]int64)
	for cRows.Next() {
		var id, n int64
		if err := cRows.Scan(&id, &n); err != nil {
			return nil, err
		}
		counts[id] = n
	}
	if err := cRows.Err(); err != nil {
		return nil, err
	}
	for i := range roles {
		roles[i].Policies = byRole[roles[i].ID]
		roles[i].Members = counts[roles[i].ID]
	}
	return &listRolesOutput{Body: roles}, nil
}

type listUsersParams struct {
	Search        string `query:"search"`
	Page          int    `query:"page"`
	PerPage       int    `query:"per_page"`
	ExcludeRoleID int64  `query:"exclude_role_id"`
}

func (h *RBACHandler) listUsers(ctx context.Context, p *listUsersParams) (*listUsersOutput, error) {
	tid := tenant.FromContext(ctx)
	search := strings.TrimSpace(p.Search)
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PerPage <= 0 {
		p.PerPage = 50
	}
	offset := (p.Page - 1) * p.PerPage
	var q string
	var args []any
	if h.Driver == "postgres" {
		q = "SELECT id, username FROM gcfm_users WHERE tenant_id=$1"
		args = append(args, tid)
		idx := 2
		if search != "" {
			q += fmt.Sprintf(" AND username LIKE $%d", idx)
			args = append(args, "%"+search+"%")
			idx++
		}
		if p.ExcludeRoleID > 0 {
			q += fmt.Sprintf(" AND id NOT IN (SELECT user_id FROM gcfm_user_roles WHERE role_id=$%d)", idx)
			args = append(args, p.ExcludeRoleID)
			idx++
		}
		q += fmt.Sprintf(" ORDER BY username LIMIT $%d OFFSET $%d", idx, idx+1)
		args = append(args, p.PerPage, offset)
	} else {
		q = "SELECT id, username FROM gcfm_users WHERE tenant_id=?"
		args = append(args, tid)
		if search != "" {
			q += " AND username LIKE ?"
			args = append(args, "%"+search+"%")
		}
		if p.ExcludeRoleID > 0 {
			q += " AND id NOT IN (SELECT user_id FROM gcfm_user_roles WHERE role_id=?)"
			args = append(args, p.ExcludeRoleID)
		}
		q += " ORDER BY username LIMIT ? OFFSET ?"
		args = append(args, p.PerPage, offset)
	}
	rows, err := h.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []schema.User{}
	ids := []int64{}
	for rows.Next() {
		var u schema.User
		if err := rows.Scan(&u.ID, &u.Username); err != nil {
			return nil, err
		}
		users = append(users, u)
		ids = append(ids, int64(u.ID))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		var rq string
		var rargs []any
		if h.Driver == "postgres" {
			ph := make([]string, len(ids))
			for i, id := range ids {
				ph[i] = fmt.Sprintf("$%d", i+1)
				rargs = append(rargs, id)
			}
			rq = fmt.Sprintf("SELECT ur.user_id, r.name FROM gcfm_user_roles ur JOIN gcfm_roles r ON ur.role_id=r.id WHERE ur.user_id IN (%s)", strings.Join(ph, ","))
		} else {
			ph := make([]string, len(ids))
			for i, id := range ids {
				ph[i] = "?"
				rargs = append(rargs, id)
			}
			rq = fmt.Sprintf("SELECT ur.user_id, r.name FROM gcfm_user_roles ur JOIN gcfm_roles r ON ur.role_id=r.id WHERE ur.user_id IN (%s)", strings.Join(ph, ","))
		}
		rrows, err := h.DB.QueryContext(ctx, rq, rargs...)
		if err != nil {
			return nil, err
		}
		defer rrows.Close()
		rolesByUser := make(map[int64][]string)
		for rrows.Next() {
			var uid int64
			var role string
			if err := rrows.Scan(&uid, &role); err != nil {
				return nil, err
			}
			rolesByUser[uid] = append(rolesByUser[uid], role)
		}
		if err := rrows.Err(); err != nil {
			return nil, err
		}
		for i := range users {
			if r, ok := rolesByUser[int64(users[i].ID)]; ok {
				users[i].Roles = r
			}
		}
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

type roleMembersOutput struct {
	Body struct {
		RoleID int64         `json:"roleId"`
		Users  []schema.User `json:"users"`
	}
}

type roleMembersInput struct {
	ID   int64 `path:"id"`
	Body struct {
		UserIDs []int64 `json:"userIds"`
	}
}

func (h *RBACHandler) getRoleMembers(ctx context.Context, p *roleIDParam) (*roleMembersOutput, error) {
	tid := tenant.FromContext(ctx)
	var q string
	if h.Driver == "postgres" {
		q = "SELECT u.id, u.username FROM gcfm_users u JOIN gcfm_user_roles ur ON ur.user_id=u.id WHERE ur.role_id=$1 AND u.tenant_id=$2 ORDER BY u.username"
	} else {
		q = "SELECT u.id, u.username FROM gcfm_users u JOIN gcfm_user_roles ur ON ur.user_id=u.id WHERE ur.role_id=? AND u.tenant_id=? ORDER BY u.username"
	}
	rows, err := h.DB.QueryContext(ctx, q, p.ID, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []schema.User{}
	for rows.Next() {
		var u schema.User
		if err := rows.Scan(&u.ID, &u.Username); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := &roleMembersOutput{}
	out.Body.RoleID = p.ID
	out.Body.Users = users
	return out, nil
}

func (h *RBACHandler) putRoleMembers(ctx context.Context, in *roleMembersInput) (*struct{}, error) {
	tid := tenant.FromContext(ctx)
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var q string
	if h.Driver == "postgres" {
		q = "SELECT ur.user_id FROM gcfm_user_roles ur JOIN gcfm_users u ON ur.user_id=u.id WHERE ur.role_id=$1 AND u.tenant_id=$2"
	} else {
		q = "SELECT ur.user_id FROM gcfm_user_roles ur JOIN gcfm_users u ON ur.user_id=u.id WHERE ur.role_id=? AND u.tenant_id=?"
	}
	rows, err := tx.QueryContext(ctx, q, in.ID, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	existing := map[int64]struct{}{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		existing[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	newSet := map[int64]struct{}{}
	for _, id := range in.Body.UserIDs {
		newSet[id] = struct{}{}
	}
	for id := range existing {
		if _, ok := newSet[id]; !ok {
			if h.Driver == "postgres" {
				if _, err := tx.ExecContext(ctx, "DELETE FROM gcfm_user_roles WHERE role_id=$1 AND user_id=$2", in.ID, id); err != nil {
					return nil, err
				}
			} else {
				if _, err := tx.ExecContext(ctx, "DELETE FROM gcfm_user_roles WHERE role_id=? AND user_id=?", in.ID, id); err != nil {
					return nil, err
				}
			}
		}
	}
	for id := range newSet {
		if _, ok := existing[id]; ok {
			continue
		}
		var cnt int
		if h.Driver == "postgres" {
			if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM gcfm_users WHERE id=$1 AND tenant_id=$2", id, tid).Scan(&cnt); err != nil {
				return nil, err
			}
		} else {
			if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM gcfm_users WHERE id=? AND tenant_id=?", id, tid).Scan(&cnt); err != nil {
				return nil, err
			}
		}
		if cnt == 0 {
			return nil, huma.Error422("userIds", fmt.Sprintf("user %d not found", id))
		}
		if h.Driver == "postgres" {
			if _, err := tx.ExecContext(ctx, "INSERT INTO gcfm_user_roles(user_id, role_id) VALUES($1,$2) ON CONFLICT DO NOTHING", id, in.ID); err != nil {
				return nil, err
			}
		} else {
			if _, err := tx.ExecContext(ctx, "INSERT IGNORE INTO gcfm_user_roles(user_id, role_id) VALUES(?, ?)", id, in.ID); err != nil {
				return nil, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	rbac.ReloadEnforcer(ctx, h.DB)
	actor := middleware.UserFromContext(ctx)
	if h.Recorder != nil {
		payload := map[string]any{"object": "role-members", "role_id": in.ID, "user_ids": in.Body.UserIDs}
		_ = h.Recorder.WriteJSON(ctx, actor, "rbac", payload)
	}
	return &struct{}{}, nil
}

type listPoliciesOutput struct{ Body []schema.Policy }

type policyInput struct {
	ID   int64 `path:"id"`
	Body schema.Policy
}

type policyParams struct {
	ID     int64  `path:"id"`
	Path   string `query:"path"`
	Method string `query:"method"`
}

func (h *RBACHandler) getRolePolicies(ctx context.Context, p *roleIDParam) (*listPoliciesOutput, error) {
	var q string
	if h.Driver == "postgres" {
		q = "SELECT path, method FROM gcfm_role_policies WHERE role_id=$1 ORDER BY path, method"
	} else {
		q = "SELECT path, method FROM gcfm_role_policies WHERE role_id=? ORDER BY path, method"
	}
	rows, err := h.DB.QueryContext(ctx, q, p.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ps := []schema.Policy{}
	for rows.Next() {
		var pol schema.Policy
		if err := rows.Scan(&pol.Path, &pol.Method); err != nil {
			return nil, err
		}
		ps = append(ps, pol)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &listPoliciesOutput{Body: ps}, nil
}

func (h *RBACHandler) addRolePolicy(ctx context.Context, in *policyInput) (*schema.Policy, error) {
	if !strings.HasPrefix(in.Body.Path, "/") || len(in.Body.Path) > 128 {
		return nil, huma.Error422("path", "must start with '/' and be <=128 chars")
	}
	switch in.Body.Method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return nil, huma.Error422("method", "invalid")
	}
	var q string
	var err error
	if h.Driver == "postgres" {
		q = "INSERT INTO gcfm_role_policies(role_id, path, method) VALUES($1,$2,$3)"
		_, err = h.DB.ExecContext(ctx, q, in.ID, in.Body.Path, in.Body.Method)
	} else {
		q = "INSERT INTO gcfm_role_policies(role_id, path, method) VALUES(?,?,?)"
		_, err = h.DB.ExecContext(ctx, q, in.ID, in.Body.Path, in.Body.Method)
	}
	if err != nil {
		if isDuplicateErr(err) {
			return nil, huma.Error409Conflict("policy exists")
		}
		return nil, err
	}
	rbac.ReloadEnforcer(ctx, h.DB)
	actor := middleware.UserFromContext(ctx)
	if h.Recorder != nil {
		payload := map[string]any{"object": "role-policies", "role_id": in.ID, "path": in.Body.Path, "method": in.Body.Method}
		_ = h.Recorder.WriteJSON(ctx, actor, "rbac", payload)
	}
	return &in.Body, nil
}

func (h *RBACHandler) deleteRolePolicy(ctx context.Context, p *policyParams) (*struct{}, error) {
	if p.Path == "" || !strings.HasPrefix(p.Path, "/") || len(p.Path) > 128 {
		return nil, huma.Error422("path", "invalid")
	}
	switch p.Method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return nil, huma.Error422("method", "invalid")
	}
	var q string
	if h.Driver == "postgres" {
		q = "DELETE FROM gcfm_role_policies WHERE role_id=$1 AND path=$2 AND method=$3"
	} else {
		q = "DELETE FROM gcfm_role_policies WHERE role_id=? AND path=? AND method=?"
	}
	res, err := h.DB.ExecContext(ctx, q, p.ID, p.Path, p.Method)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		rbac.ReloadEnforcer(ctx, h.DB)
		actor := middleware.UserFromContext(ctx)
		if h.Recorder != nil {
			payload := map[string]any{"object": "role-policies", "role_id": p.ID, "path": p.Path, "method": p.Method}
			_ = h.Recorder.WriteJSON(ctx, actor, "rbac", payload)
		}
	}
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
