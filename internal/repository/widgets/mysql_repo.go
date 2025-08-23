package widgetsrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MySQLRepo implements Repo for MySQL databases.
type MySQLRepo struct{ DB *sql.DB }

// NewMySQLRepo creates a new MySQLRepo.
func NewMySQLRepo(db *sql.DB) Repo { return &MySQLRepo{DB: db} }

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

// List returns widgets matching the filter.
func (r *MySQLRepo) List(ctx context.Context, f Filter) ([]Row, int, error) {
	q := "SELECT id,name,version,type,scopes,enabled,description,capabilities,homepage,meta,tenant_scope,tenants,updated_at FROM gcfm_widgets WHERE 1=1"
	args := []any{}
	if len(f.ScopeIn) > 0 {
		q += " AND tenant_scope IN (" + placeholders(len(f.ScopeIn)) + ")"
		for _, s := range f.ScopeIn {
			args = append(args, s)
		}
	}
	if f.Q != "" {
		q += " AND (id LIKE ? OR name LIKE ? OR description LIKE ?)"
		like := "%" + f.Q + "%"
		args = append(args, like, like, like)
	}
	if f.Tenant != "" {
		q += " AND (tenant_scope='system' OR JSON_CONTAINS(tenants, JSON_ARRAY(?)))"
		args = append(args, f.Tenant)
	}
	q += " ORDER BY updated_at DESC"
	if f.Limit > 0 {
		q += " LIMIT ? OFFSET ?"
		args = append(args, f.Limit, f.Offset)
	}
	rows, err := r.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []Row
	for rows.Next() {
		var rr Row
		var scopes, caps, tenants, meta []byte
		var desc, home sql.NullString
		if err := rows.Scan(&rr.ID, &rr.Name, &rr.Version, &rr.Type, &scopes, &rr.Enabled, &desc, &caps, &home, &meta, &rr.TenantScope, &tenants, &rr.UpdatedAt); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(scopes, &rr.Scopes)
		_ = json.Unmarshal(caps, &rr.Capabilities)
		_ = json.Unmarshal(tenants, &rr.Tenants)
		if desc.Valid {
			rr.Description = &desc.String
		}
		if home.Valid {
			rr.Homepage = &home.String
		}
		if len(meta) > 0 {
			_ = json.Unmarshal(meta, &rr.Meta)
		}
		items = append(items, rr)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	cq := "SELECT COUNT(*) FROM gcfm_widgets WHERE 1=1"
	cargs := []any{}
	if len(f.ScopeIn) > 0 {
		cq += " AND tenant_scope IN (" + placeholders(len(f.ScopeIn)) + ")"
		for _, s := range f.ScopeIn {
			cargs = append(cargs, s)
		}
	}
	if f.Q != "" {
		cq += " AND (id LIKE ? OR name LIKE ? OR description LIKE ?)"
		like := "%" + f.Q + "%"
		cargs = append(cargs, like, like, like)
	}
	if f.Tenant != "" {
		cq += " AND (tenant_scope='system' OR JSON_CONTAINS(tenants, JSON_ARRAY(?)))"
		cargs = append(cargs, f.Tenant)
	}
	var total int
	if err := r.DB.QueryRowContext(ctx, cq, cargs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetETagAndLastMod returns an ETag and last modified timestamp for the filtered set.
func (r *MySQLRepo) GetETagAndLastMod(ctx context.Context, f Filter) (string, time.Time, error) {
	q := "SELECT COALESCE(LOWER(HEX(SHA2(GROUP_CONCAT(id,'@',version,'#',DATE_FORMAT(updated_at,'%Y-%m-%dT%H:%i:%s.%fZ') ORDER BY id SEPARATOR ''),256))), ''), COALESCE(MAX(updated_at), '1970-01-01') FROM gcfm_widgets WHERE 1=1"
	args := []any{}
	if len(f.ScopeIn) > 0 {
		q += " AND tenant_scope IN (" + placeholders(len(f.ScopeIn)) + ")"
		for _, s := range f.ScopeIn {
			args = append(args, s)
		}
	}
	if f.Q != "" {
		q += " AND (id LIKE ? OR name LIKE ? OR description LIKE ?)"
		like := "%" + f.Q + "%"
		args = append(args, like, like, like)
	}
	if f.Tenant != "" {
		q += " AND (tenant_scope='system' OR JSON_CONTAINS(tenants, JSON_ARRAY(?)))"
		args = append(args, f.Tenant)
	}
	var etag sql.NullString
	var last time.Time
	if err := r.DB.QueryRowContext(ctx, q, args...).Scan(&etag, &last); err != nil {
		return "", time.Time{}, err
	}
	if etag.Valid && etag.String != "" {
		return fmt.Sprintf("\"%s\"", etag.String), last, nil
	}
	return "\"\"", last, nil
}

// Upsert inserts or updates a widget.
func (r *MySQLRepo) Upsert(ctx context.Context, rr Row) error {
	scopes, _ := json.Marshal(rr.Scopes)
	caps, _ := json.Marshal(rr.Capabilities)
	tenants, _ := json.Marshal(rr.Tenants)
	meta, _ := json.Marshal(rr.Meta)
	_, err := r.DB.ExecContext(ctx, `
        INSERT INTO gcfm_widgets (id,name,version,type,scopes,enabled,description,capabilities,homepage,meta,tenant_scope,tenants,updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,NOW(6))
        ON DUPLICATE KEY UPDATE
            name=VALUES(name), version=VALUES(version), type=VALUES(type),
            scopes=VALUES(scopes), enabled=VALUES(enabled), description=VALUES(description),
            capabilities=VALUES(capabilities), homepage=VALUES(homepage), meta=VALUES(meta),
            tenant_scope=VALUES(tenant_scope), tenants=VALUES(tenants), updated_at=NOW(6)
    `, rr.ID, rr.Name, rr.Version, rr.Type, scopes, rr.Enabled, rr.Description, caps, rr.Homepage, meta, rr.TenantScope, tenants)
	return err
}

// Remove deletes a widget.
func (r *MySQLRepo) Remove(ctx context.Context, id string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM gcfm_widgets WHERE id=?`, id)
	return err
}

// GetByID retrieves a widget by ID.
func (r *MySQLRepo) GetByID(ctx context.Context, id string) (Row, error) {
	var rr Row
	var scopes, caps, tenants, meta []byte
	var desc, home sql.NullString
	err := r.DB.QueryRowContext(ctx, `SELECT id,name,version,type,scopes,enabled,description,capabilities,homepage,meta,tenant_scope,tenants,updated_at FROM gcfm_widgets WHERE id=?`, id).Scan(
		&rr.ID, &rr.Name, &rr.Version, &rr.Type, &scopes, &rr.Enabled, &desc, &caps, &home, &meta, &rr.TenantScope, &tenants, &rr.UpdatedAt)
	if err != nil {
		return Row{}, err
	}
	_ = json.Unmarshal(scopes, &rr.Scopes)
	_ = json.Unmarshal(caps, &rr.Capabilities)
	_ = json.Unmarshal(tenants, &rr.Tenants)
	if desc.Valid {
		rr.Description = &desc.String
	}
	if home.Valid {
		rr.Homepage = &home.String
	}
	if len(meta) > 0 {
		_ = json.Unmarshal(meta, &rr.Meta)
	}
	return rr, nil
}
