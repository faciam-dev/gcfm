package widgetsrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
	"github.com/lib/pq"
)

// PGRepo implements Repo for PostgreSQL databases using goquent ORM.
type PGRepo struct{ DB *sql.DB }

// NewPGRepo creates a new PGRepo.
func NewPGRepo(db *sql.DB) Repo { return &PGRepo{DB: db} }

func (r *PGRepo) table() string { return "gcfm_widgets" }

func (r *PGRepo) applyFilters(q *query.Query, f Filter) {
	if len(f.ScopeIn) > 0 {
		q.WhereIn("tenant_scope", pq.Array(f.ScopeIn))
	}
	if f.Q != "" {
		like := "%" + f.Q + "%"
		q.WhereGroup(func(g *query.Query) {
			g.WhereRaw("id ILIKE :s", map[string]any{"s": like}).
				OrWhereRaw("name ILIKE :s", map[string]any{"s": like}).
				OrWhereRaw("description ILIKE :s", map[string]any{"s": like})
		})
	}
	if f.Tenant != "" {
		q.WhereGroup(func(g *query.Query) {
			g.Where("tenant_scope", "system").
				OrWhereRaw(":t = ANY(tenants)", map[string]any{"t": f.Tenant}).
				OrWhereRaw("(tenant_scope = 'tenant' AND array_length(tenants, 1) = 0)", nil)
		})
	}
}

// List returns widgets matching the filter.
func (r *PGRepo) List(ctx context.Context, f Filter) ([]Row, int, error) {
	q := query.New(r.DB, r.table(), ormdriver.PostgresDialect{}).
		Select("id", "name", "version", "type", "scopes", "enabled", "description", "capabilities", "homepage", "meta", "tenant_scope", "tenants", "updated_at")
	r.applyFilters(q, f)
	q.OrderBy("updated_at", "desc")
	if f.Limit > 0 {
		q.Limit(f.Limit).Offset(f.Offset)
	}
	type dbRow struct {
		ID           string         `db:"id"`
		Name         string         `db:"name"`
		Version      string         `db:"version"`
		Type         string         `db:"type"`
		Scopes       pq.StringArray `db:"scopes"`
		Enabled      bool           `db:"enabled"`
		Description  sql.NullString `db:"description"`
		Capabilities pq.StringArray `db:"capabilities"`
		Homepage     sql.NullString `db:"homepage"`
		Meta         []byte         `db:"meta"`
		TenantScope  string         `db:"tenant_scope"`
		Tenants      pq.StringArray `db:"tenants"`
		UpdatedAt    time.Time      `db:"updated_at"`
	}
	var rs []dbRow
	if err := q.WithContext(ctx).Get(&rs); err != nil {
		return nil, 0, err
	}
	items := make([]Row, 0, len(rs))
	for _, r0 := range rs {
		rr := Row{
			ID:           r0.ID,
			Name:         r0.Name,
			Version:      r0.Version,
			Type:         r0.Type,
			Scopes:       []string(r0.Scopes),
			Enabled:      r0.Enabled,
			Capabilities: []string(r0.Capabilities),
			TenantScope:  r0.TenantScope,
			Tenants:      []string(r0.Tenants),
			UpdatedAt:    r0.UpdatedAt,
		}
		if r0.Description.Valid {
			rr.Description = &r0.Description.String
		}
		if r0.Homepage.Valid {
			rr.Homepage = &r0.Homepage.String
		}
		if len(r0.Meta) > 0 {
			_ = json.Unmarshal(r0.Meta, &rr.Meta)
		}
		items = append(items, rr)
	}

	cq := query.New(r.DB, r.table(), ormdriver.PostgresDialect{})
	r.applyFilters(cq, f)
	cnt, err := cq.WithContext(ctx).Count("*")
	if err != nil {
		return nil, 0, err
	}
	return items, int(cnt), nil
}

// GetETagAndLastMod returns an ETag and last modified timestamp for the filtered set.
func (r *PGRepo) GetETagAndLastMod(ctx context.Context, f Filter) (string, time.Time, error) {
	q := query.New(r.DB, r.table(), ormdriver.PostgresDialect{}).
		SelectRaw("coalesce(encode(digest(string_agg(id||'@'||version||'#'||to_char(updated_at,'YYYY-MM-DD\"T\"HH24:MI:SS.MS\"Z\"') ORDER BY id), 'sha256'),'hex'),'') AS etag").
		SelectRaw("coalesce(MAX(updated_at), 'epoch') AS last_mod")
	r.applyFilters(q, f)
	var res struct {
		ETag sql.NullString `db:"etag"`
		Last time.Time      `db:"last_mod"`
	}
	if err := q.WithContext(ctx).First(&res); err != nil {
		return "", time.Time{}, err
	}
	if res.ETag.Valid {
		return fmt.Sprintf("\"%s\"", res.ETag.String), res.Last, nil
	}
	return "\"\"", res.Last, nil
}

// Upsert inserts or updates a widget.
func (r *PGRepo) Upsert(ctx context.Context, rr Row) error {
	metaBytes, _ := json.Marshal(rr.Meta)
	data := map[string]any{
		"id":           rr.ID,
		"name":         rr.Name,
		"version":      rr.Version,
		"type":         rr.Type,
		"scopes":       pq.Array(rr.Scopes),
		"enabled":      rr.Enabled,
		"description":  rr.Description,
		"capabilities": pq.Array(rr.Capabilities),
		"homepage":     rr.Homepage,
		"meta":         metaBytes,
		"tenant_scope": rr.TenantScope,
		"tenants":      pq.Array(rr.Tenants),
		"updated_at":   time.Now(),
	}
	_, err := query.New(r.DB, r.table(), ormdriver.PostgresDialect{}).WithContext(ctx).
		Upsert([]map[string]any{data}, []string{"id"}, []string{"name", "version", "type", "scopes", "enabled", "description", "capabilities", "homepage", "meta", "tenant_scope", "tenants", "updated_at"})
	return err
}

// Remove deletes a widget.
func (r *PGRepo) Remove(ctx context.Context, id string) error {
	_, err := query.New(r.DB, r.table(), ormdriver.PostgresDialect{}).Where("id", id).WithContext(ctx).Delete()
	return err
}

// GetByID retrieves a widget by ID.
func (r *PGRepo) GetByID(ctx context.Context, id string) (Row, error) {
	q := query.New(r.DB, r.table(), ormdriver.PostgresDialect{}).
		Select("id", "name", "version", "type", "scopes", "enabled", "description", "capabilities", "homepage", "meta", "tenant_scope", "tenants", "updated_at").
		Where("id", id)
	var r0 struct {
		ID           string         `db:"id"`
		Name         string         `db:"name"`
		Version      string         `db:"version"`
		Type         string         `db:"type"`
		Scopes       pq.StringArray `db:"scopes"`
		Enabled      bool           `db:"enabled"`
		Description  sql.NullString `db:"description"`
		Capabilities pq.StringArray `db:"capabilities"`
		Homepage     sql.NullString `db:"homepage"`
		Meta         []byte         `db:"meta"`
		TenantScope  string         `db:"tenant_scope"`
		Tenants      pq.StringArray `db:"tenants"`
		UpdatedAt    time.Time      `db:"updated_at"`
	}
	if err := q.WithContext(ctx).First(&r0); err != nil {
		return Row{}, err
	}
	rr := Row{
		ID:           r0.ID,
		Name:         r0.Name,
		Version:      r0.Version,
		Type:         r0.Type,
		Scopes:       []string(r0.Scopes),
		Enabled:      r0.Enabled,
		Capabilities: []string(r0.Capabilities),
		TenantScope:  r0.TenantScope,
		Tenants:      []string(r0.Tenants),
		UpdatedAt:    r0.UpdatedAt,
	}
	if r0.Description.Valid {
		rr.Description = &r0.Description.String
	}
	if r0.Homepage.Valid {
		rr.Homepage = &r0.Homepage.String
	}
	if len(r0.Meta) > 0 {
		_ = json.Unmarshal(r0.Meta, &rr.Meta)
	}
	return rr, nil
}
