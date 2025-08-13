package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"github.com/faciam-dev/gcfm/internal/api/schema"
	audit "github.com/faciam-dev/gcfm/internal/customfield/audit"
	huma "github.com/faciam-dev/gcfm/internal/huma"
	"github.com/faciam-dev/gcfm/internal/rbac"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/tenant"
)

// RBACHandler provides role and user listing endpoints.
type RBACHandler struct {
	DB           *sql.DB
	Driver       string
	Recorder     *audit.Recorder
	PasswordCost int
	TablePrefix  string
}

func (h *RBACHandler) t(name string) string { return h.TablePrefix + name }

type listRolesOutput struct{ Body []schema.Role }

type listUsersOutput struct{ Body schema.UsersPage }

type createUserInput struct {
	Body struct {
		Username string   `json:"username"`
		Password string   `json:"password"`
		Roles    []string `json:"roles,omitempty"`
	}
}

type userOutput struct{ Body schema.User }

var roleNamePattern = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

var (
	reloadMu    sync.Mutex
	reloadTimer *time.Timer
)

func scheduleEnforcerReload(ctx context.Context, db *sql.DB) {
	reloadMu.Lock()
	defer reloadMu.Unlock()
	if reloadTimer != nil {
		reloadTimer.Stop()
	}
	reloadTimer = time.AfterFunc(100*time.Millisecond, func() {
		rbac.ReloadEnforcer(context.Background(), db)
	})
}

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
	}, h.ListUsers)

	huma.Register(api, huma.Operation{
		OperationID:   "createUser",
		Method:        http.MethodPost,
		Path:          "/v1/rbac/users",
		Summary:       "Create user",
		Tags:          []string{"RBAC"},
		Errors:        []int{http.StatusConflict, http.StatusUnprocessableEntity, http.StatusBadRequest},
		DefaultStatus: http.StatusCreated,
	}, h.createUser)

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
	rows, err := h.DB.QueryContext(ctx, fmt.Sprintf("SELECT id, name, comment FROM %s ORDER BY id", h.t("roles")))
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
	pRows, err = h.DB.QueryContext(ctx, fmt.Sprintf("SELECT role_id, path, method FROM %s", h.t("role_policies")))
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
		cq = fmt.Sprintf("SELECT ur.role_id, COUNT(*) FROM %s ur JOIN %s u ON ur.user_id=u.id WHERE u.tenant_id=$1 GROUP BY ur.role_id", h.t("user_roles"), h.t("users"))
		cRows, err = h.DB.QueryContext(ctx, cq, tid)
	} else {
		cq = fmt.Sprintf("SELECT ur.role_id, COUNT(*) FROM %s ur JOIN %s u ON ur.user_id=u.id WHERE u.tenant_id=? GROUP BY ur.role_id", h.t("user_roles"), h.t("users"))
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

func (h *RBACHandler) ListUsers(ctx context.Context, p *schema.ListUsersParams) (*listUsersOutput, error) {
	tid := tenant.FromContext(ctx)
	if tid == "" {
		return nil, huma.Error422("missing tenant", "X-Tenant-ID header is required")
	}

	page := p.Page
	if page < 1 {
		page = 1
	}
	per := p.PerPage
	if per <= 0 {
		per = 20
	}
	if per > 200 {
		per = 200
	}

	sort := p.Sort
	if sort == "" {
		sort = "username"
	}
	order := strings.ToLower(p.Order)
	if order == "" {
		order = "asc"
	}

	var sortCol string
	switch sort {
	case "username":
		sortCol = "u.username"
	case "created_at":
		sortCol = "u.created_at"
	default:
		msg := "sort must be one of [username, created_at]"
		return nil, huma.NewError(http.StatusBadRequest, msg, &huma.ErrorDetail{Location: "sort", Message: msg, Value: sort})
	}
	if order != "asc" && order != "desc" {
		msg := "order must be one of [asc, desc]"
		return nil, huma.NewError(http.StatusBadRequest, msg, &huma.ErrorDetail{Location: "order", Message: msg, Value: order})
	}

	i := 0
	ph := func() string {
		i++
		if h.Driver == "postgres" {
			return fmt.Sprintf("$%d", i)
		}
		return "?"
	}

	var where []string
	var args []any

	where = append(where, fmt.Sprintf("u.tenant_id = %s", ph()))
	args = append(args, tid)

	if p.Search != "" {
		if h.Driver == "postgres" {
			where = append(where, fmt.Sprintf("u.username ILIKE %s", ph()))
		} else {
			where = append(where, fmt.Sprintf("u.username LIKE %s", ph()))
		}
		args = append(args, "%"+p.Search+"%")
	}

	if p.ExcludeRoleID > 0 {
		where = append(where, fmt.Sprintf("NOT EXISTS (SELECT 1 FROM %s ur WHERE ur.user_id = u.id AND ur.role_id = %s)", h.t("user_roles"), ph()))
		args = append(args, p.ExcludeRoleID)
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}

	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s u %s`, h.t("users"), whereSQL)
	var total int64
	if err := h.DB.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (page - 1) * per
	listSQL := fmt.Sprintf(`
               SELECT u.id, u.username
                 FROM %s u
                 %s
                ORDER BY %s %s
                LIMIT %s OFFSET %s`, h.t("users"), whereSQL, sortCol, strings.ToUpper(order), ph(), ph())
	listArgs := append(append([]any{}, args...), per, offset)

	rows, err := h.DB.QueryContext(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]schema.UserBrief, 0, per)
	ids := make([]int64, 0, per)
	for rows.Next() {
		var u schema.UserBrief
		if err := rows.Scan(&u.ID, &u.Username); err != nil {
			return nil, err
		}
		users = append(users, u)
		ids = append(ids, u.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(ids) > 0 {
		rolesMap := make(map[int64][]string, len(ids))
		const batchSize = 100
		for start := 0; start < len(ids); start += batchSize {
			end := start + batchSize
			if end > len(ids) {
				end = len(ids)
			}
			batch := ids[start:end]
			inPH := make([]string, len(batch))
			args := make([]any, len(batch))
			for j, id := range batch {
				if h.Driver == "postgres" {
					inPH[j] = fmt.Sprintf("$%d", j+1)
				} else {
					inPH[j] = "?"
				}
				args[j] = id
			}
			roleSQL := fmt.Sprintf(`
                       SELECT ur.user_id, r.name
                         FROM %s ur
                         JOIN %s r ON r.id = ur.role_id
                        WHERE ur.user_id IN (%s)
                        ORDER BY r.name`, h.t("user_roles"), h.t("roles"), strings.Join(inPH, ","))
			rrows, err := h.DB.QueryContext(ctx, roleSQL, args...)
			if err != nil {
				return nil, err
			}
			for rrows.Next() {
				var uid int64
				var rname string
				if err := rrows.Scan(&uid, &rname); err != nil {
					rrows.Close()
					return nil, err
				}
				rolesMap[uid] = append(rolesMap[uid], rname)
			}
			if err := rrows.Close(); err != nil {
				return nil, err
			}
			if err := rrows.Err(); err != nil {
				return nil, err
			}
		}
		for i := range users {
			if rs, ok := rolesMap[users[i].ID]; ok {
				users[i].Roles = rs
			}
		}
	}

	out := schema.UsersPage{
		Items:   users,
		Total:   total,
		Page:    page,
		PerPage: per,
	}
	return &listUsersOutput{Body: out}, nil
}

func (h *RBACHandler) createUser(ctx context.Context, in *createUserInput) (*userOutput, error) {
	tid := tenant.FromContext(ctx)
	if tid == "" {
		return nil, huma.Error422("missing tenant", "X-Tenant-ID header is required")
	}
	if l := len(in.Body.Username); l < 3 || l > 64 || !usernamePattern.MatchString(in.Body.Username) {
		return nil, huma.NewError(http.StatusBadRequest, "invalid username", &huma.ErrorDetail{Location: "username", Message: "must be 3-64 chars and match ^[a-zA-Z0-9._-]+$"})
	}
	if l := len(in.Body.Password); l < 8 || l > 128 {
		return nil, huma.NewError(http.StatusBadRequest, "invalid password", &huma.ErrorDetail{Location: "password", Message: "must be 8-128 chars"})
	}
	seen := make(map[string]struct{})
	roles := make([]string, 0, len(in.Body.Roles))
	for _, r := range in.Body.Roles {
		if l := len(r); l < 1 || l > 64 {
			return nil, huma.NewError(http.StatusBadRequest, "invalid role", &huma.ErrorDetail{Location: "roles", Message: "each must be 1-64 chars"})
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		roles = append(roles, r)
	}

	roleIDs := make([]int64, 0, len(roles))
	missing := []string{}

	if len(roles) > 0 {
		// Build query and args for IN clause
		var (
			query    string
			args     []interface{}
			nameToID = make(map[string]int64, len(roles))
		)

		if h.Driver == "postgres" {
			placeholders := make([]string, len(roles))
			for i := range roles {
				placeholders[i] = fmt.Sprintf("$%d", i+1)
			}
			query = fmt.Sprintf("SELECT id, name FROM %s WHERE name IN (%s)", h.t("roles"), strings.Join(placeholders, ","))
			for _, r := range roles {
				args = append(args, r)
			}
		} else {
			placeholders := make([]string, len(roles))
			for i := range roles {
				placeholders[i] = "?"
			}
			query = fmt.Sprintf("SELECT id, name FROM %s WHERE name IN (%s)", h.t("roles"), strings.Join(placeholders, ","))
			for _, r := range roles {
				args = append(args, r)
			}
		}

		rows, err := h.DB.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var id int64
			var name string
			if err := rows.Scan(&id, &name); err != nil {
				return nil, err
			}
			nameToID[name] = id
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

		for _, name := range roles {
			id, ok := nameToID[name]
			if !ok {
				missing = append(missing, name)
				continue
			}
			roleIDs = append(roleIDs, id)
		}
	}

	if len(missing) > 0 {
		msg := fmt.Sprintf("missing roles: %s", strings.Join(missing, ","))
		return nil, huma.NewError(http.StatusUnprocessableEntity, msg, &huma.ErrorDetail{Location: "roles", Message: msg})
	}

	tx, err := h.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	cost := h.PasswordCost
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Body.Password), cost)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	var (
		id         int64
		created    time.Time
		rawCreated any
	)
	if h.Driver == "postgres" {
		err = tx.QueryRowContext(ctx, fmt.Sprintf("INSERT INTO %s(tenant_id, username, password_hash) VALUES($1,$2,$3) RETURNING id, created_at", h.t("users")), tid, in.Body.Username, hash).Scan(&id, &rawCreated)
		if err == nil {
			created, err = ParseAuditTime(rawCreated)
		}
	} else {
		res, execErr := tx.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s(tenant_id, username, password_hash) VALUES(?,?,?)", h.t("users")), tid, in.Body.Username, hash)
		if execErr != nil {
			err = execErr
		} else {
			id, err = res.LastInsertId()
			if err == nil {
				err = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT created_at FROM %s WHERE id=?", h.t("users")), id).Scan(&rawCreated)
				if err == nil {
					created, err = ParseAuditTime(rawCreated)
				}
			}
		}
	}
	if err != nil {
		_ = tx.Rollback()
		if isDuplicateErr(err) {
			return nil, huma.Error409Conflict("username already exists in this tenant")
		}
		return nil, err
	}
	if len(roleIDs) > 0 {
		var (
			valueStrings []string
			valueArgs    []interface{}
		)
		if h.Driver == "postgres" {
			argPos := 1
			for _, rid := range roleIDs {
				valueStrings = append(valueStrings, fmt.Sprintf("($%d,$%d)", argPos, argPos+1))
				valueArgs = append(valueArgs, id, rid)
				argPos += 2
			}
			stmt := fmt.Sprintf("INSERT INTO %s(user_id, role_id) VALUES %s", h.t("user_roles"), strings.Join(valueStrings, ","))
			if _, err := tx.ExecContext(ctx, stmt, valueArgs...); err != nil {
				_ = tx.Rollback()
				return nil, err
			}
		} else {
			for _, rid := range roleIDs {
				valueStrings = append(valueStrings, "(?, ?)")
				valueArgs = append(valueArgs, id, rid)
			}
			stmt := fmt.Sprintf("INSERT INTO %s(user_id, role_id) VALUES %s", h.t("user_roles"), strings.Join(valueStrings, ","))
			if _, err := tx.ExecContext(ctx, stmt, valueArgs...); err != nil {
				_ = tx.Rollback()
				return nil, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	scheduleEnforcerReload(ctx, h.DB)
	actor := middleware.UserFromContext(ctx)
	if h.Recorder != nil {
		payload := map[string]any{"id": id, "tenant_id": tid, "username": in.Body.Username, "roles": roles}
		_ = h.Recorder.WriteTableJSON(ctx, actor, "CREATE", h.t("users"), payload)
	}
	out := schema.User{ID: id, TenantID: tid, Username: in.Body.Username, Roles: roles, CreatedAt: created}
	return &userOutput{Body: out}, nil
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
		err := h.DB.QueryRowContext(ctx, fmt.Sprintf("INSERT INTO %s(name, comment) VALUES($1, $2) RETURNING id", h.t("roles")), in.Body.Name, comment).Scan(&id)
		if err != nil {
			if isDuplicateErr(err) {
				return nil, huma.Error409Conflict("role already exists")
			}
			return nil, err
		}
	} else {
		res, err := h.DB.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s(name, comment) VALUES(?, ?)", h.t("roles")), in.Body.Name, comment)
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
	scheduleEnforcerReload(ctx, h.DB)
	r := schema.Role{ID: id, Name: in.Body.Name}
	if in.Body.Comment != nil {
		r.Comment = *in.Body.Comment
	}
	return &roleOutput{Body: r}, nil
}

func (h *RBACHandler) deleteRole(ctx context.Context, p *roleIDParam) (*struct{}, error) {
	var q string
	if h.Driver == "postgres" {
		q = fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE role_id=$1", h.t("user_roles"))
	} else {
		q = fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE role_id=?", h.t("user_roles"))
	}
	var cnt int
	if err := h.DB.QueryRowContext(ctx, q, p.ID).Scan(&cnt); err != nil {
		return nil, err
	}
	if cnt > 0 {
		return nil, huma.Error409Conflict("role has users")
	}
	if h.Driver == "postgres" {
		q = fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE role_id=$1", h.t("role_policies"))
	} else {
		q = fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE role_id=?", h.t("role_policies"))
	}
	if err := h.DB.QueryRowContext(ctx, q, p.ID).Scan(&cnt); err != nil {
		return nil, err
	}
	if cnt > 0 {
		return nil, huma.Error409Conflict("role has policies")
	}
	if h.Driver == "postgres" {
		q = fmt.Sprintf("DELETE FROM %s WHERE id=$1", h.t("roles"))
	} else {
		q = fmt.Sprintf("DELETE FROM %s WHERE id=?", h.t("roles"))
	}
	if _, err := h.DB.ExecContext(ctx, q, p.ID); err != nil {
		return nil, err
	}
	scheduleEnforcerReload(ctx, h.DB)
	return &struct{}{}, nil
}

type roleMembersOutput struct {
	Body struct {
		RoleID int64              `json:"roleId"`
		Users  []schema.UserBrief `json:"users"`
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
		q = fmt.Sprintf("SELECT u.id, u.username FROM %s u JOIN %s ur ON ur.user_id=u.id WHERE ur.role_id=$1 AND u.tenant_id=$2 ORDER BY u.username", h.t("users"), h.t("user_roles"))
	} else {
		q = fmt.Sprintf("SELECT u.id, u.username FROM %s u JOIN %s ur ON ur.user_id=u.id WHERE ur.role_id=? AND u.tenant_id=? ORDER BY u.username", h.t("users"), h.t("user_roles"))
	}
	rows, err := h.DB.QueryContext(ctx, q, p.ID, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []schema.UserBrief{}
	for rows.Next() {
		var u schema.UserBrief
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
		q = fmt.Sprintf("SELECT ur.user_id FROM %s ur JOIN %s u ON ur.user_id=u.id WHERE ur.role_id=$1 AND u.tenant_id=$2", h.t("user_roles"), h.t("users"))
	} else {
		q = fmt.Sprintf("SELECT ur.user_id FROM %s ur JOIN %s u ON ur.user_id=u.id WHERE ur.role_id=? AND u.tenant_id=?", h.t("user_roles"), h.t("users"))
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
				if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE role_id=$1 AND user_id=$2", h.t("user_roles")), in.ID, id); err != nil {
					return nil, err
				}
			} else {
				if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE role_id=? AND user_id=?", h.t("user_roles")), in.ID, id); err != nil {
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
			if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id=$1 AND tenant_id=$2", h.t("users")), id, tid).Scan(&cnt); err != nil {
				return nil, err
			}
		} else {
			if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id=? AND tenant_id=?", h.t("users")), id, tid).Scan(&cnt); err != nil {
				return nil, err
			}
		}
		if cnt == 0 {
			return nil, huma.Error422("userIds", fmt.Sprintf("user %d not found", id))
		}
		if h.Driver == "postgres" {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s(user_id, role_id) VALUES($1,$2) ON CONFLICT DO NOTHING", h.t("user_roles")), id, in.ID); err != nil {
				return nil, err
			}
		} else {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("INSERT IGNORE INTO %s(user_id, role_id) VALUES(?, ?)", h.t("user_roles")), id, in.ID); err != nil {
				return nil, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	scheduleEnforcerReload(ctx, h.DB)
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
		q = fmt.Sprintf("SELECT path, method FROM %s WHERE role_id=$1 ORDER BY path, method", h.t("role_policies"))
	} else {
		q = fmt.Sprintf("SELECT path, method FROM %s WHERE role_id=? ORDER BY path, method", h.t("role_policies"))
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
		q = fmt.Sprintf("INSERT INTO %s(role_id, path, method) VALUES($1,$2,$3)", h.t("role_policies"))
		_, err = h.DB.ExecContext(ctx, q, in.ID, in.Body.Path, in.Body.Method)
	} else {
		q = fmt.Sprintf("INSERT INTO %s(role_id, path, method) VALUES(?,?,?)", h.t("role_policies"))
		_, err = h.DB.ExecContext(ctx, q, in.ID, in.Body.Path, in.Body.Method)
	}
	if err != nil {
		if isDuplicateErr(err) {
			return nil, huma.Error409Conflict("policy exists")
		}
		return nil, err
	}
	scheduleEnforcerReload(ctx, h.DB)
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
		q = fmt.Sprintf("DELETE FROM %s WHERE role_id=$1 AND path=$2 AND method=$3", h.t("role_policies"))
	} else {
		q = fmt.Sprintf("DELETE FROM %s WHERE role_id=? AND path=? AND method=?", h.t("role_policies"))
	}
	res, err := h.DB.ExecContext(ctx, q, p.ID, p.Path, p.Method)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		scheduleEnforcerReload(ctx, h.DB)
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
