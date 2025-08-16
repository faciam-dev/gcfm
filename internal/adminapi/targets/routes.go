package targets

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/lib/pq"

	huma "github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	metapkg "github.com/faciam-dev/gcfm/meta"
)

// Deps defines external dependencies for handlers.
type Deps struct {
	Meta metapkg.MetaStore
	Rec  *audit.Recorder
	Auth func(scopes ...string) func(huma.Context, func(huma.Context))
}

// RegisterRoutes registers the admin target management routes.
func RegisterRoutes(api huma.API, deps Deps) {
	r := huma.NewGroup(api, "/admin/targets")
	if deps.Auth != nil {
		r.UseMiddleware(deps.Auth("admin:targets"))
	}

	huma.Get(r, "/", listHandler(deps))
	huma.Post(r, "/", createHandler(deps), func(o *huma.Operation) {
		o.DefaultStatus = http.StatusCreated
		o.Middlewares = append(o.Middlewares, deps.Auth("admin:targets:write"))
	})
	huma.Get(r, "/{key}", getHandler(deps))
	huma.Put(r, "/{key}", putHandler(deps), func(o *huma.Operation) {
		o.Middlewares = append(o.Middlewares, deps.Auth("admin:targets:write"))
	})
	huma.Patch(r, "/{key}", patchHandler(deps), func(o *huma.Operation) {
		o.Middlewares = append(o.Middlewares, deps.Auth("admin:targets:write"))
	})
	huma.Delete(r, "/{key}", deleteHandler(deps), func(o *huma.Operation) {
		o.DefaultStatus = http.StatusNoContent
		o.Middlewares = append(o.Middlewares, deps.Auth("admin:targets:write"))
	})
	huma.Post(r, "/{key}/default", setDefaultHandler(deps), func(o *huma.Operation) {
		o.DefaultStatus = http.StatusNoContent
		o.Middlewares = append(o.Middlewares, deps.Auth("admin:targets:write"))
	})

	v := huma.NewGroup(api, "/admin/targets/version")
	if deps.Auth != nil {
		v.UseMiddleware(deps.Auth("admin:targets"))
	}
	huma.Get(v, "", getVersionHandler(deps))
	huma.Post(v, "/bump", bumpVersionHandler(deps), func(o *huma.Operation) {
		o.Middlewares = append(o.Middlewares, deps.Auth("admin:targets:write"))
	})
}

// ---- handler wrapper ----
type handler struct {
	Deps
}

func listHandler(d Deps) func(context.Context, *targetListParams) (*targetListOutput, error) {
	h := handler{d}
	return h.list
}

func createHandler(d Deps) func(context.Context, *targetCreateInput) (*targetOutput, error) {
	h := handler{d}
	return h.create
}

func getHandler(d Deps) func(context.Context, *targetKeyParams) (*targetOutput, error) {
	h := handler{d}
	return h.get
}

func putHandler(d Deps) func(context.Context, *targetPutInput) (*targetOutput, error) {
	h := handler{d}
	return h.put
}

func patchHandler(d Deps) func(context.Context, *targetPatchInput) (*targetOutput, error) {
	h := handler{d}
	return h.patch
}

func deleteHandler(d Deps) func(context.Context, *targetDeleteParams) (*etagOnly, error) {
	h := handler{d}
	return h.del
}

func setDefaultHandler(d Deps) func(context.Context, *targetDeleteParams) (*etagOnly, error) {
	h := handler{d}
	return h.setDefault
}

func getVersionHandler(d Deps) func(context.Context, *struct{}) (*versionOutput, error) {
	h := handler{d}
	return func(ctx context.Context, _ *struct{}) (*versionOutput, error) { return h.getVersion(ctx) }
}

func bumpVersionHandler(d Deps) func(context.Context, *ifMatchHeader) (*versionBodyOutput, error) {
	h := handler{d}
	return h.bumpVersion
}

// ---- parameter & output types ----
type targetListParams struct {
	Label  []string `query:"label" explode:"true"`
	Q      string   `query:"q"`
	Limit  int      `query:"limit" default:"50" minimum:"1" maximum:"200"`
	Cursor string   `query:"cursor"`
}

type targetListOutput struct {
	ETag string `header:"ETag"`
	Body schema.TargetsList
}

type targetCreateInput struct {
	IfMatch string `header:"If-Match"`
	Body    schema.TargetInput
}

type targetOutput struct {
	ETag string        `header:"ETag"`
	Body schema.Target `json:"body"`
}

type targetKeyParams struct {
	Key string `path:"key"`
}

type targetPutInput struct {
	targetKeyParams
	IfMatch string `header:"If-Match"`
	Body    schema.TargetInput
}

type targetPatchInput struct {
	targetKeyParams
	IfMatch string `header:"If-Match"`
	Body    schema.TargetPatch
}

type targetDeleteParams struct {
	targetKeyParams
	IfMatch string `header:"If-Match"`
}

type etagOnly struct {
	ETag string `header:"ETag"`
}

type versionOutput struct {
	ETag string `header:"ETag"`
	Body struct {
		Version    string `json:"version"`
		DefaultKey string `json:"defaultKey"`
	}
}

type versionBodyOutput struct {
	ETag string `header:"ETag"`
	Body struct {
		Version string `json:"version"`
	}
}

type ifMatchHeader struct {
	IfMatch string `header:"If-Match"`
}

// ---- helpers ----
var labelRe = regexp.MustCompile(`^[\-._:/a-zA-Z0-9=]+$`)

func validateLabels(labels []string) error {
	for _, l := range labels {
		if !labelRe.MatchString(l) {
			return errors.New("invalid label")
		}
	}
	return nil
}

func validateDSN(driver, dsn string) error {
	if driver == "" || dsn == "" {
		return errors.New("driver and dsn required")
	}
	if driver == "mysql" && !strings.HasPrefix(dsn, "mysql://") {
		return errors.New("mysql dsn must start with mysql://")
	}
	if driver == "postgres" && !strings.HasPrefix(dsn, "postgres://") {
		return errors.New("postgres dsn must start with postgres://")
	}
	return nil
}

func checkIfMatch(ifMatch, current string) error {
	if ifMatch == "" {
		return nil
	}
	if ifMatch != current {
		return huma.Error412PreconditionFailed("etag mismatch")
	}
	return nil
}

func mapStoreError(err error) error {
	if err == nil {
		return nil
	}
	if isConflictError(err) {
		return huma.Error409Conflict("conflict")
	}
	return err
}

func isConflictError(err error) bool {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		return me.Number == 1062
	}
	var pe *pq.Error
	if errors.As(err, &pe) {
		return string(pe.Code) == "23505"
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique")
}

func rollbackIfNeeded(tx *sql.Tx) {
	_ = tx.Rollback()
}

func toSchema(r metapkg.TargetRowWithLabels) schema.Target {
	return schema.Target{
		Key:           r.Key,
		Driver:        r.Driver,
		DSN:           r.DSN,
		Schema:        r.Schema,
		Labels:        r.Labels,
		MaxOpenConns:  r.MaxOpenConns,
		MaxIdleConns:  r.MaxIdleConns,
		ConnMaxIdleMs: int(r.ConnMaxIdle / time.Millisecond),
		ConnMaxLifeMs: int(r.ConnMaxLife / time.Millisecond),
		IsDefault:     r.IsDefault,
	}
}

func matchLabels(labels []string, queries []string) bool {
	for _, q := range queries {
		found := false
		for _, l := range labels {
			if l == q {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func matchQuery(r metapkg.TargetRowWithLabels, q string) bool {
	q = strings.ToLower(q)
	fields := []string{r.Key, r.Driver, r.DSN, r.Schema}
	fields = append(fields, r.Labels...)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

func timeFromMs(ms int) time.Duration {
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

func (h handler) find(ctx context.Context, key string) (*metapkg.TargetRowWithLabels, string, error) {
	rows, ver, _, err := h.Meta.ListTargets(ctx)
	if err != nil {
		return nil, "", err
	}
	for _, r := range rows {
		if r.Key == key {
			return &r, ver, nil
		}
	}
	return nil, ver, nil
}

// ---- handler implementations ----
func (h handler) list(ctx context.Context, p *targetListParams) (*targetListOutput, error) {
	rows, ver, _, err := h.Meta.ListTargets(ctx)
	if err != nil {
		return nil, err
	}
	filtered := make([]metapkg.TargetRowWithLabels, 0, len(rows))
	for _, r := range rows {
		if !matchLabels(r.Labels, p.Label) {
			continue
		}
		if p.Q != "" && !matchQuery(r, p.Q) {
			continue
		}
		filtered = append(filtered, r)
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Key < filtered[j].Key })
	start := 0
	if p.Cursor != "" {
		for i, r := range filtered {
			if r.Key > p.Cursor {
				start = i
				break
			}
		}
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	out := &targetListOutput{ETag: ver}
	for i := start; i < len(filtered) && len(out.Body.Items) < limit; i++ {
		out.Body.Items = append(out.Body.Items, toSchema(filtered[i]))
	}
	if start+len(out.Body.Items) < len(filtered) {
		out.Body.NextCursor = filtered[start+len(out.Body.Items)].Key
	}
	return out, nil
}

func (h handler) get(ctx context.Context, p *targetKeyParams) (*targetOutput, error) {
	rows, ver, _, err := h.Meta.ListTargets(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.Key == p.Key {
			return &targetOutput{ETag: ver, Body: toSchema(r)}, nil
		}
	}
	return nil, huma.Error404NotFound("not found")
}

func (h handler) create(ctx context.Context, in *targetCreateInput) (*targetOutput, error) {
	if err := validateDSN(in.Body.Driver, in.Body.DSN); err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	if err := validateLabels(in.Body.Labels); err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	_, ver, _, err := h.Meta.ListTargets(ctx)
	if err != nil {
		return nil, err
	}
	if err := checkIfMatch(in.IfMatch, ver); err != nil {
		return nil, err
	}
	row, newVer, err := createOrUpsert(ctx, h.Meta, in.Body)
	if err != nil {
		return nil, err
	}
	actor := middleware.UserFromContext(ctx)
	if h.Rec != nil {
		_ = h.Rec.WriteJSON(ctx, actor, "admin.targets.upsert", map[string]any{
			"actor":   actor,
			"key":     in.Body.Key,
			"before":  nil,
			"after":   in.Body,
			"labels":  in.Body.Labels,
			"version": newVer,
		})
	}
	return &targetOutput{ETag: newVer, Body: toSchema(row)}, nil
}

func (h handler) put(ctx context.Context, in *targetPutInput) (*targetOutput, error) {
	if in.Body.Key == "" {
		in.Body.Key = in.Key
	}
	if err := validateDSN(in.Body.Driver, in.Body.DSN); err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	if err := validateLabels(in.Body.Labels); err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	existing, ver, err := h.find(ctx, in.Key)
	if err != nil {
		return nil, err
	}
	if err := checkIfMatch(in.IfMatch, ver); err != nil {
		return nil, err
	}
	if existing != nil {
		in.Body.IsDefault = existing.IsDefault
	}
	row, newVer, err := createOrUpsert(ctx, h.Meta, in.Body)
	if err != nil {
		return nil, err
	}
	actor := middleware.UserFromContext(ctx)
	if h.Rec != nil {
		_ = h.Rec.WriteJSON(ctx, actor, "admin.targets.upsert", map[string]any{
			"actor":   actor,
			"key":     in.Body.Key,
			"before":  existing,
			"after":   in.Body,
			"labels":  in.Body.Labels,
			"version": newVer,
		})
	}
	return &targetOutput{ETag: newVer, Body: toSchema(row)}, nil
}

func (h handler) patch(ctx context.Context, in *targetPatchInput) (*targetOutput, error) {
	existing, ver, err := h.find(ctx, in.Key)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, huma.Error404NotFound("not found")
	}
	if err := checkIfMatch(in.IfMatch, ver); err != nil {
		return nil, err
	}
	row := *existing
	if in.Body.Driver != nil {
		row.Driver = *in.Body.Driver
	}
	if in.Body.DSN != nil {
		row.DSN = *in.Body.DSN
	}
	if in.Body.Schema != nil {
		row.Schema = *in.Body.Schema
	}
	if in.Body.Labels != nil {
		if err := validateLabels(in.Body.Labels); err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		row.Labels = in.Body.Labels
	}
	if in.Body.Driver != nil || in.Body.DSN != nil {
		if err := validateDSN(row.Driver, row.DSN); err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
	}
	if in.Body.MaxOpenConns != nil {
		row.MaxOpenConns = *in.Body.MaxOpenConns
	}
	if in.Body.MaxIdleConns != nil {
		row.MaxIdleConns = *in.Body.MaxIdleConns
	}
	if in.Body.ConnMaxIdleMs != nil {
		row.ConnMaxIdle = timeFromMs(*in.Body.ConnMaxIdleMs)
	}
	if in.Body.ConnMaxLifeMs != nil {
		row.ConnMaxLife = timeFromMs(*in.Body.ConnMaxLifeMs)
	}
	tx, err := h.Meta.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	base := metapkg.TargetRow{
		Key:          row.Key,
		Driver:       row.Driver,
		DSN:          row.DSN,
		Schema:       row.Schema,
		MaxOpenConns: row.MaxOpenConns,
		MaxIdleConns: row.MaxIdleConns,
		ConnMaxIdle:  row.ConnMaxIdle,
		ConnMaxLife:  row.ConnMaxLife,
		IsDefault:    row.IsDefault,
	}
	if err := h.Meta.UpsertTarget(ctx, tx, base, row.Labels); err != nil {
		rollbackIfNeeded(tx)
		return nil, mapStoreError(err)
	}
	newVer, err := h.Meta.BumpTargetsVersion(ctx, tx)
	if err != nil {
		rollbackIfNeeded(tx)
		return nil, mapStoreError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, mapStoreError(err)
	}
	actor := middleware.UserFromContext(ctx)
	if h.Rec != nil {
		_ = h.Rec.WriteJSON(ctx, actor, "admin.targets.patch", map[string]any{
			"actor":   actor,
			"key":     row.Key,
			"before":  existing,
			"after":   toSchema(row),
			"labels":  row.Labels,
			"version": newVer,
		})
	}
	return &targetOutput{ETag: newVer, Body: toSchema(row)}, nil
}

func (h handler) del(ctx context.Context, p *targetDeleteParams) (*etagOnly, error) {
	existing, ver, err := h.find(ctx, p.Key)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, huma.Error404NotFound("not found")
	}
	if err := checkIfMatch(p.IfMatch, ver); err != nil {
		return nil, err
	}
	tx, err := h.Meta.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if err := h.Meta.DeleteTarget(ctx, tx, p.Key); err != nil {
		rollbackIfNeeded(tx)
		return nil, mapStoreError(err)
	}
	newVer, err := h.Meta.BumpTargetsVersion(ctx, tx)
	if err != nil {
		rollbackIfNeeded(tx)
		return nil, mapStoreError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, mapStoreError(err)
	}
	actor := middleware.UserFromContext(ctx)
	if h.Rec != nil {
		_ = h.Rec.WriteJSON(ctx, actor, "admin.targets.delete", map[string]any{
			"actor":   actor,
			"key":     p.Key,
			"before":  existing,
			"after":   nil,
			"version": newVer,
		})
	}
	return &etagOnly{ETag: newVer}, nil
}

func (h handler) setDefault(ctx context.Context, p *targetDeleteParams) (*etagOnly, error) {
	existing, ver, err := h.find(ctx, p.Key)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, huma.Error404NotFound("not found")
	}
	if err := checkIfMatch(p.IfMatch, ver); err != nil {
		return nil, err
	}
	tx, err := h.Meta.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if err := h.Meta.SetDefaultTarget(ctx, tx, p.Key); err != nil {
		rollbackIfNeeded(tx)
		return nil, mapStoreError(err)
	}
	newVer, err := h.Meta.BumpTargetsVersion(ctx, tx)
	if err != nil {
		rollbackIfNeeded(tx)
		return nil, mapStoreError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, mapStoreError(err)
	}
	actor := middleware.UserFromContext(ctx)
	if h.Rec != nil {
		_ = h.Rec.WriteJSON(ctx, actor, "admin.targets.set-default", map[string]any{
			"actor":   actor,
			"key":     p.Key,
			"version": newVer,
		})
	}
	return &etagOnly{ETag: newVer}, nil
}

func (h handler) getVersion(ctx context.Context) (*versionOutput, error) {
	_, ver, def, err := h.Meta.ListTargets(ctx)
	if err != nil {
		return nil, err
	}
	out := &versionOutput{ETag: ver}
	out.Body.Version = ver
	out.Body.DefaultKey = def
	return out, nil
}

func (h handler) bumpVersion(ctx context.Context, in *ifMatchHeader) (*versionBodyOutput, error) {
	_, ver, _, err := h.Meta.ListTargets(ctx)
	if err != nil {
		return nil, err
	}
	if err := checkIfMatch(in.IfMatch, ver); err != nil {
		return nil, err
	}
	tx, err := h.Meta.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	newVer, err := h.Meta.BumpTargetsVersion(ctx, tx)
	if err != nil {
		rollbackIfNeeded(tx)
		return nil, mapStoreError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, mapStoreError(err)
	}
	actor := middleware.UserFromContext(ctx)
	if h.Rec != nil {
		_ = h.Rec.WriteJSON(ctx, actor, "admin.targets.bump-version", map[string]any{
			"actor":   actor,
			"version": newVer,
		})
	}
	out := &versionBodyOutput{ETag: newVer}
	out.Body.Version = newVer
	return out, nil
}

// createOrUpsert performs target upsert and optional default setting.
func createOrUpsert(ctx context.Context, m metapkg.MetaStore, in schema.TargetInput) (metapkg.TargetRowWithLabels, string, error) {
	tx, err := m.BeginTx(ctx, nil)
	if err != nil {
		return metapkg.TargetRowWithLabels{}, "", mapStoreError(err)
	}
	defer rollbackIfNeeded(tx)

	row := metapkg.TargetRow{
		Key: in.Key, Driver: in.Driver, DSN: in.DSN, Schema: in.Schema,
		MaxOpenConns: in.MaxOpenConns, MaxIdleConns: in.MaxIdleConns,
		ConnMaxIdle: time.Millisecond * time.Duration(in.ConnMaxIdleMs),
		ConnMaxLife: time.Millisecond * time.Duration(in.ConnMaxLifeMs),
		IsDefault:   in.IsDefault,
	}
	if err := m.UpsertTarget(ctx, tx, row, in.Labels); err != nil {
		return metapkg.TargetRowWithLabels{}, "", mapStoreError(err)
	}
	if in.IsDefault {
		if err := m.SetDefaultTarget(ctx, tx, in.Key); err != nil {
			return metapkg.TargetRowWithLabels{}, "", mapStoreError(err)
		}
	}
	ver, err := m.BumpTargetsVersion(ctx, tx)
	if err != nil {
		return metapkg.TargetRowWithLabels{}, "", mapStoreError(err)
	}
	if err := tx.Commit(); err != nil {
		return metapkg.TargetRowWithLabels{}, "", mapStoreError(err)
	}
	return metapkg.TargetRowWithLabels{TargetRow: row, Labels: in.Labels}, ver, nil
}
