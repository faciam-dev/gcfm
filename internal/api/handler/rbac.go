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
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

// RBACHandler provides role and user listing endpoints.
type RBACHandler struct {
	DB           *sql.DB
	Driver       string
	Dialect      ormdriver.Dialect
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

	rolesTbl := h.t("roles")
	q := query.New(h.DB, rolesTbl, h.Dialect).
		Select("id", "name", "comment").
		OrderBy("id", "asc").
		WithContext(ctx)
	var rRows []struct {
		ID      int64
		Name    string
		Comment sql.NullString
	}
	if err := q.Get(&rRows); err != nil {
		return nil, err
	}
	roles := make([]schema.Role, 0, len(rRows))
	for _, row := range rRows {
		r := schema.Role{ID: row.ID, Name: row.Name}
		if row.Comment.Valid {
			r.Comment = row.Comment.String
		}
		roles = append(roles, r)
	}

	polTbl := h.t("role_policies")
	pq := query.New(h.DB, polTbl, h.Dialect).
		Select("role_id", "path", "method").
		WithContext(ctx)
	var pRows []struct {
		RoleID int64 `goq:"role_id"`
		Path   string
		Method string
	}
	if err := pq.Get(&pRows); err != nil {
		return nil, err
	}
	byRole := make(map[int64][]schema.Policy)
	for _, row := range pRows {
		byRole[row.RoleID] = append(byRole[row.RoleID], schema.Policy{Path: row.Path, Method: row.Method})
	}

	cTbl := h.t("user_roles") + " as ur"
	cq := query.New(h.DB, cTbl, h.Dialect).
		Select("ur.role_id").
		SelectRaw("COUNT(*) as count").
		Join(h.t("users")+" as u", "ur.user_id", "=", "u.id").
		Where("u.tenant_id", tid).
		GroupBy("ur.role_id").
		WithContext(ctx)
	var cRows []struct {
		RoleID int64 `goq:"role_id"`
		Count  int64 `goq:"count"`
	}
	if err := cq.Get(&cRows); err != nil {
		return nil, err
	}
	counts := make(map[int64]int64, len(cRows))
	for _, row := range cRows {
		counts[row.RoleID] = row.Count
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

	base := func() *query.Query {
		q := query.New(h.DB, h.t("users")+" as u", h.Dialect).Where("u.tenant_id", tid)
		if p.Search != "" {
			if _, ok := h.Dialect.(ormdriver.PostgresDialect); ok {
				q.WhereRaw("u.username ILIKE :s", map[string]any{"s": "%" + p.Search + "%"})
			} else {
				q.WhereRaw("u.username LIKE :s", map[string]any{"s": "%" + p.Search + "%"})
			}
		}
		if p.ExcludeRoleID > 0 {
			sub := query.New(h.DB, h.t("user_roles")+" as ur", h.Dialect).
				Select("1").
				WhereColumn("ur.user_id", "u.id").
				Where("ur.role_id", p.ExcludeRoleID)
			q.WhereNotExists(sub)
		}
		return q
	}

	countQ := base().SelectRaw("COUNT(*) as cnt").WithContext(ctx)
	var cr struct {
		Cnt int64 `db:"cnt"`
	}
	if err := countQ.First(&cr); err != nil {
		return nil, err
	}
	total := cr.Cnt

	offset := (page - 1) * per
	rowsQuery := base().
		Select("u.id").
		Select("u.username").
		OrderBy(sortCol, strings.ToUpper(order)).
		Limit(per).
		Offset(offset).
		WithContext(ctx)

	type row struct {
		ID       int64  `db:"id"`
		Username string `db:"username"`
	}
	var dbRows []row
	if err := rowsQuery.Get(&dbRows); err != nil {
		return nil, err
	}

	users := make([]schema.UserBrief, len(dbRows))
	ids := make([]int64, len(dbRows))
	for i, r := range dbRows {
		users[i] = schema.UserBrief{ID: r.ID, Username: r.Username}
		ids[i] = r.ID
	}

	if len(ids) > 0 {
		type roleRow struct {
			UserID int64  `db:"user_id"`
			Name   string `db:"name"`
		}
		var roleRows []roleRow
		rq := query.New(h.DB, h.t("user_roles")+" as ur", h.Dialect).
			Select("ur.user_id").
			Select("r.name").
			Join(h.t("roles")+" as r", "ur.role_id", "=", "r.id").
			WhereIn("ur.user_id", ids).
			OrderBy("r.name", "asc").
			WithContext(ctx)
		if err := rq.Get(&roleRows); err != nil {
			return nil, err
		}
		rolesMap := make(map[int64][]string, len(ids))
		for _, rr := range roleRows {
			rolesMap[rr.UserID] = append(rolesMap[rr.UserID], rr.Name)
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
	var comment any
	if in.Body.Comment != nil && *in.Body.Comment != "" {
		comment = *in.Body.Comment
	}
	id, err := query.New(h.DB, h.t("roles"), h.Dialect).
		WithContext(ctx).
		InsertGetId(map[string]any{"name": in.Body.Name, "comment": comment})
	if err != nil {
		if isDuplicateErr(err) {
			return nil, huma.Error409Conflict("role already exists")
		}
		return nil, err
	}
	scheduleEnforcerReload(ctx, h.DB)
	r := schema.Role{ID: id, Name: in.Body.Name}
	if in.Body.Comment != nil {
		r.Comment = *in.Body.Comment
	}
	return &roleOutput{Body: r}, nil
}

func (h *RBACHandler) deleteRole(ctx context.Context, p *roleIDParam) (*struct{}, error) {
	var row struct {
		Count int64 `db:"count"`
	}
	q := query.New(h.DB, h.t("user_roles"), h.Dialect).
		SelectRaw("COUNT(*) as count").
		Where("role_id", p.ID).
		WithContext(ctx)
	if err := q.First(&row); err != nil {
		return nil, err
	}
	if row.Count > 0 {
		return nil, huma.Error409Conflict("role has users")
	}
	q = query.New(h.DB, h.t("role_policies"), h.Dialect).
		SelectRaw("COUNT(*) as count").
		Where("role_id", p.ID).
		WithContext(ctx)
	if err := q.First(&row); err != nil {
		return nil, err
	}
	if row.Count > 0 {
		return nil, huma.Error409Conflict("role has policies")
	}
	if _, err := query.New(h.DB, h.t("roles"), h.Dialect).
		Where("id", p.ID).
		Delete(); err != nil {
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
	q := query.New(h.DB, h.t("users")+" as u", h.Dialect).
		Select("u.id", "u.username").
		Join(h.t("user_roles")+" as ur", "ur.user_id", "=", "u.id").
		Where("ur.role_id", p.ID).
		Where("u.tenant_id", tid).
		OrderBy("u.username", "asc").
		WithContext(ctx)
	users := []schema.UserBrief{}
	if err := q.Get(&users); err != nil {
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
	rows := []struct {
		ID int64 `db:"user_id"`
	}{}
	if err := query.New(tx, h.t("user_roles")+" as ur", h.Dialect).
		Select("ur.user_id").
		Join(h.t("users")+" as u", "ur.user_id", "=", "u.id").
		Where("ur.role_id", in.ID).
		Where("u.tenant_id", tid).
		WithContext(ctx).
		Get(&rows); err != nil {
		return nil, err
	}
	existing := map[int64]struct{}{}
	for _, r := range rows {
		existing[r.ID] = struct{}{}
	}
	newSet := map[int64]struct{}{}
	for _, id := range in.Body.UserIDs {
		newSet[id] = struct{}{}
	}
	for id := range existing {
		if _, ok := newSet[id]; !ok {
			if _, err := query.New(tx, h.t("user_roles"), h.Dialect).
				WithContext(ctx).
				Where("role_id", in.ID).
				Where("user_id", id).
				Delete(); err != nil {
				return nil, err
			}
		}
	}
	for id := range newSet {
		if _, ok := existing[id]; ok {
			continue
		}
		var c struct {
			Count int `db:"count"`
		}
		if err := query.New(tx, h.t("users"), h.Dialect).
			SelectRaw("COUNT(*) as count").
			Where("id", id).
			Where("tenant_id", tid).
			WithContext(ctx).
			First(&c); err != nil {
			return nil, err
		}
		if c.Count == 0 {
			return nil, huma.Error422("userIds", fmt.Sprintf("user %d not found", id))
		}
		if _, err := query.New(tx, h.t("user_roles"), h.Dialect).
			WithContext(ctx).
			InsertOrIgnore([]map[string]any{{"user_id": id, "role_id": in.ID}}); err != nil {
			return nil, err
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
	q := query.New(h.DB, h.t("role_policies"), h.Dialect).
		Select("path", "method").
		Where("role_id", p.ID).
		OrderBy("path", "asc").
		OrderBy("method", "asc").
		WithContext(ctx)
	ps := []schema.Policy{}
	if err := q.Get(&ps); err != nil {
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
	_, err := query.New(h.DB, h.t("role_policies"), h.Dialect).
		WithContext(ctx).
		Insert(map[string]any{"role_id": in.ID, "path": in.Body.Path, "method": in.Body.Method})
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
	res, err := query.New(h.DB, h.t("role_policies"), h.Dialect).
		WithContext(ctx).
		Where("role_id", p.ID).
		Where("path", p.Path).
		Where("method", p.Method).
		Delete()
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
