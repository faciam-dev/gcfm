package widgetsrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type Filter struct {
	Tenant  string
	ScopeIn []string
	Q       string
	Limit   int
	Offset  int
}

type Row struct {
	ID           string
	Name         string
	Version      string
	Type         string
	Scopes       []string
	Enabled      bool
	Description  *string
	Capabilities []string
	Homepage     *string
	Meta         map[string]any
	TenantScope  string
	Tenants      []string
	UpdatedAt    time.Time
}

type Repo interface {
	List(ctx context.Context, f Filter) ([]Row, int, error)
	GetETagAndLastMod(ctx context.Context, f Filter) (string, time.Time, error)
	Upsert(ctx context.Context, r Row) error
	Remove(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (Row, error)
}

type PGRepo struct {
	DB *sql.DB
}

func NewPGRepo(db *sql.DB) Repo {
	return &PGRepo{DB: db}
}

const baseFilter = `
WITH base AS (
  SELECT * FROM gcfm_widgets
  WHERE (cardinality($1::text[]) = 0 OR tenant_scope = ANY($1))
    AND ($2 = '' OR id ILIKE '%'||$2||'%' OR name ILIKE '%'||$2||'%' OR description ILIKE '%'||$2||'%')
),
filtered AS (
  SELECT * FROM base
  WHERE CASE
    WHEN $3 = '' THEN TRUE
    WHEN EXISTS (SELECT 1 FROM unnest(tenants) t WHERE t = $3) THEN TRUE
    WHEN tenant_scope = 'system' THEN TRUE
    ELSE FALSE
  END
)
`

func (r *PGRepo) List(ctx context.Context, f Filter) ([]Row, int, error) {
	args := []any{pq.Array(f.ScopeIn), f.Q, f.Tenant}
	query := baseFilter + `
SELECT id, name, version, type, scopes, enabled, description, capabilities, homepage, meta, tenant_scope, tenants, updated_at
FROM filtered
ORDER BY updated_at DESC`
	if f.Limit > 0 {
		query += " LIMIT $4 OFFSET $5"
		args = append(args, f.Limit, f.Offset)
	}
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []Row
	for rows.Next() {
		var (
			scopes, caps, tenants pq.StringArray
			desc, home            sql.NullString
			metaBytes             []byte
			rr                    Row
		)
		if err := rows.Scan(&rr.ID, &rr.Name, &rr.Version, &rr.Type, &scopes, &rr.Enabled, &desc, &caps, &home, &metaBytes, &rr.TenantScope, &tenants, &rr.UpdatedAt); err != nil {
			return nil, 0, err
		}
		rr.Scopes = []string(scopes)
		rr.Capabilities = []string(caps)
		rr.Tenants = []string(tenants)
		if desc.Valid {
			rr.Description = &desc.String
		}
		if home.Valid {
			rr.Homepage = &home.String
		}
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &rr.Meta)
		}
		items = append(items, rr)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	// total
	var total int
	if err := r.DB.QueryRowContext(ctx, baseFilter+" SELECT count(*) FROM filtered", pq.Array(f.ScopeIn), f.Q, f.Tenant).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *PGRepo) GetETagAndLastMod(ctx context.Context, f Filter) (string, time.Time, error) {
	var etag sql.NullString
	var last time.Time
	err := r.DB.QueryRowContext(ctx, baseFilter+`
SELECT coalesce(encode(digest(string_agg(id||'@'||version||'#'||to_char(updated_at,'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"') ORDER BY id), 'sha256'),'hex'),'') AS etag,
       coalesce(MAX(updated_at), 'epoch') AS last_mod
FROM filtered`, pq.Array(f.ScopeIn), f.Q, f.Tenant).Scan(&etag, &last)
	if err != nil {
		return "", time.Time{}, err
	}
	if etag.Valid {
		return fmt.Sprintf("\"%s\"", etag.String), last, nil
	}
	return "\"\"", last, nil
}

func (r *PGRepo) Upsert(ctx context.Context, rr Row) error {
	metaBytes, _ := json.Marshal(rr.Meta)
	_, err := r.DB.ExecContext(ctx, `
INSERT INTO gcfm_widgets (id, name, version, type, scopes, enabled, description, capabilities, homepage, meta, tenant_scope, tenants, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12, now())
ON CONFLICT (id) DO UPDATE SET
  name=$2, version=$3, type=$4, scopes=$5, enabled=$6, description=$7,
  capabilities=$8, homepage=$9, meta=$10, tenant_scope=$11, tenants=$12,
  updated_at=now()`,
		rr.ID, rr.Name, rr.Version, rr.Type, pq.Array(rr.Scopes), rr.Enabled, rr.Description, pq.Array(rr.Capabilities), rr.Homepage, metaBytes, rr.TenantScope, pq.Array(rr.Tenants))
	return err
}

func (r *PGRepo) Remove(ctx context.Context, id string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM gcfm_widgets WHERE id=$1`, id)
	return err
}

func (r *PGRepo) GetByID(ctx context.Context, id string) (Row, error) {
	var (
		scopes, caps, tenants pq.StringArray
		desc, home            sql.NullString
		metaBytes             []byte
		rr                    Row
	)
	err := r.DB.QueryRowContext(ctx, `SELECT id, name, version, type, scopes, enabled, description, capabilities, homepage, meta, tenant_scope, tenants, updated_at FROM gcfm_widgets WHERE id=$1`, id).Scan(
		&rr.ID, &rr.Name, &rr.Version, &rr.Type, &scopes, &rr.Enabled, &desc, &caps, &home, &metaBytes, &rr.TenantScope, &tenants, &rr.UpdatedAt)
	if err != nil {
		return Row{}, err
	}
	rr.Scopes = []string(scopes)
	rr.Capabilities = []string(caps)
	rr.Tenants = []string(tenants)
	if desc.Valid {
		rr.Description = &desc.String
	}
	if home.Valid {
		rr.Homepage = &home.String
	}
	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &rr.Meta)
	}
	return rr, nil
}
