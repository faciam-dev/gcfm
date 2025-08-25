package widgetsrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

// MySQLRepo implements Repo for MySQL databases using goquent ORM.
type MySQLRepo struct {
	DB          *sql.DB
	TablePrefix string
}

// NewMySQLRepo creates a new MySQLRepo.
func NewMySQLRepo(db *sql.DB, prefix string) Repo { return &MySQLRepo{DB: db, TablePrefix: prefix} }

func (r *MySQLRepo) table() string { return r.TablePrefix + "widgets" }

// applyFilters applies the given filter to the query builder.
func (r *MySQLRepo) applyFilters(q *query.Query, f Filter) {
	if len(f.ScopeIn) > 0 {
		q.WhereIn("tenant_scope", f.ScopeIn)
	}
	if f.Q != "" {
		like := "%" + f.Q + "%"
		q.WhereGroup(func(g *query.Query) {
			g.WhereRaw("id LIKE :s", map[string]any{"s": like}).
				OrWhereRaw("name LIKE :s", map[string]any{"s": like}).
				OrWhereRaw("description LIKE :s", map[string]any{"s": like})
		})
	}
	if f.Tenant != "" {
		q.WhereGroup(func(g *query.Query) {
			g.Where("tenant_scope", "system").
				OrWhereRaw("JSON_CONTAINS(tenants, JSON_ARRAY(:t))", map[string]any{"t": f.Tenant}).
				OrWhereRaw("(tenant_scope = 'tenant' AND JSON_LENGTH(tenants) = 0)", nil)
		})
	}
}

// List returns widgets matching the filter.
func (r *MySQLRepo) List(ctx context.Context, f Filter) ([]Row, int, error) {
	q := query.New(r.DB, r.table(), ormdriver.MySQLDialect{}).
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
		Scopes       []byte         `db:"scopes"`
		Enabled      bool           `db:"enabled"`
		Description  sql.NullString `db:"description"`
		Capabilities []byte         `db:"capabilities"`
		Homepage     sql.NullString `db:"homepage"`
		Meta         []byte         `db:"meta"`
		TenantScope  string         `db:"tenant_scope"`
		Tenants      []byte         `db:"tenants"`
		UpdatedAt    time.Time      `db:"updated_at"`
	}
	var rs []dbRow
	if err := q.WithContext(ctx).Get(&rs); err != nil {
		return nil, 0, err
	}
	items := make([]Row, 0, len(rs))
	for _, r0 := range rs {
		var rr Row
		rr.ID = r0.ID
		rr.Name = r0.Name
		rr.Version = r0.Version
		rr.Type = r0.Type
		if err := json.Unmarshal(r0.Scopes, &rr.Scopes); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal scopes for id %s: %w", r0.ID, err)
		}
		rr.Enabled = r0.Enabled
		if r0.Description.Valid {
			rr.Description = &r0.Description.String
		}
		if err := json.Unmarshal(r0.Capabilities, &rr.Capabilities); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal capabilities for id %s: %w", r0.ID, err)
		}
		if r0.Homepage.Valid {
			rr.Homepage = &r0.Homepage.String
		}
		if len(r0.Meta) > 0 {
			if err := json.Unmarshal(r0.Meta, &rr.Meta); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal meta for id %s: %w", r0.ID, err)
			}
		}
		rr.TenantScope = r0.TenantScope
		if err := json.Unmarshal(r0.Tenants, &rr.Tenants); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal tenants for id %s: %w", r0.ID, err)
		}
		rr.UpdatedAt = r0.UpdatedAt
		items = append(items, rr)
	}

	cq := query.New(r.DB, r.table(), ormdriver.MySQLDialect{})
	r.applyFilters(cq, f)
	cnt, err := cq.WithContext(ctx).Count("*")
	if err != nil {
		return nil, 0, err
	}
	return items, int(cnt), nil
}

// GetETagAndLastMod returns an ETag and last modified timestamp for the filtered set.
func (r *MySQLRepo) GetETagAndLastMod(ctx context.Context, f Filter) (string, time.Time, error) {
	q := query.New(r.DB, r.table(), ormdriver.MySQLDialect{}).
		SelectRaw("COALESCE(LOWER(HEX(SHA2(GROUP_CONCAT(id,'@',version,'#',DATE_FORMAT(updated_at,'%Y-%m-%dT%H:%i:%s.%fZ') ORDER BY id SEPARATOR ''),256))), '') AS etag").
		SelectRaw("COALESCE(MAX(updated_at), DATE('1970-01-01')) AS last_mod")
	r.applyFilters(q, f)
	var res struct {
		ETag sql.NullString `db:"etag"`
		Last time.Time      `db:"last_mod"`
	}
	if err := q.WithContext(ctx).First(&res); err != nil {
		return "", time.Time{}, err
	}
	if res.ETag.Valid && res.ETag.String != "" {
		return fmt.Sprintf("\"%s\"", res.ETag.String), res.Last, nil
	}
	return "\"\"", res.Last, nil
}

// Upsert inserts or updates a widget.
func (r *MySQLRepo) Upsert(ctx context.Context, rr Row) error {
	scopes, _ := json.Marshal(rr.Scopes)
	caps, _ := json.Marshal(rr.Capabilities)
	tenants, _ := json.Marshal(rr.Tenants)
	meta, _ := json.Marshal(rr.Meta)
	data := map[string]any{
		"id":           rr.ID,
		"name":         rr.Name,
		"version":      rr.Version,
		"type":         rr.Type,
		"scopes":       scopes,
		"enabled":      rr.Enabled,
		"description":  rr.Description,
		"capabilities": caps,
		"homepage":     rr.Homepage,
		"meta":         meta,
		"tenant_scope": rr.TenantScope,
		"tenants":      tenants,
		"updated_at":   time.Now(),
	}
	_, err := query.New(r.DB, r.table(), ormdriver.MySQLDialect{}).WithContext(ctx).
		Upsert([]map[string]any{data}, []string{"id"}, []string{"name", "version", "type", "scopes", "enabled", "description", "capabilities", "homepage", "meta", "tenant_scope", "tenants", "updated_at"})
	return err
}

// Remove deletes a widget.
func (r *MySQLRepo) Remove(ctx context.Context, id string) error {
	_, err := query.New(r.DB, r.table(), ormdriver.MySQLDialect{}).Where("id", id).WithContext(ctx).Delete()
	return err
}

// GetByID retrieves a widget by ID.
func (r *MySQLRepo) GetByID(ctx context.Context, id string) (Row, error) {
	q := query.New(r.DB, r.table(), ormdriver.MySQLDialect{}).
		Select("id", "name", "version", "type", "scopes", "enabled", "description", "capabilities", "homepage", "meta", "tenant_scope", "tenants", "updated_at").
		Where("id", id)
	var r0 struct {
		ID           string         `db:"id"`
		Name         string         `db:"name"`
		Version      string         `db:"version"`
		Type         string         `db:"type"`
		Scopes       []byte         `db:"scopes"`
		Enabled      bool           `db:"enabled"`
		Description  sql.NullString `db:"description"`
		Capabilities []byte         `db:"capabilities"`
		Homepage     sql.NullString `db:"homepage"`
		Meta         []byte         `db:"meta"`
		TenantScope  string         `db:"tenant_scope"`
		Tenants      []byte         `db:"tenants"`
		UpdatedAt    time.Time      `db:"updated_at"`
	}
	if err := q.WithContext(ctx).First(&r0); err != nil {
		return Row{}, err
	}
	var rr Row
	rr.ID = r0.ID
	rr.Name = r0.Name
	rr.Version = r0.Version
	rr.Type = r0.Type
	if err := json.Unmarshal(r0.Scopes, &rr.Scopes); err != nil {
		return Row{}, fmt.Errorf("failed to unmarshal scopes: %w", err)
	}
	rr.Enabled = r0.Enabled
	if r0.Description.Valid {
		rr.Description = &r0.Description.String
	}
	if err := json.Unmarshal(r0.Capabilities, &rr.Capabilities); err != nil {
		return Row{}, fmt.Errorf("failed to unmarshal capabilities: %w", err)
	}
	if r0.Homepage.Valid {
		rr.Homepage = &r0.Homepage.String
	}
	if len(r0.Meta) > 0 {
		if err := json.Unmarshal(r0.Meta, &rr.Meta); err != nil {
			return Row{}, fmt.Errorf("failed to unmarshal meta: %w", err)
		}
	}
	rr.TenantScope = r0.TenantScope
	if err := json.Unmarshal(r0.Tenants, &rr.Tenants); err != nil {
		return Row{}, fmt.Errorf("failed to unmarshal tenants: %w", err)
	}
	rr.UpdatedAt = r0.UpdatedAt
	return rr, nil
}
