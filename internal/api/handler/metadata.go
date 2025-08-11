package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	humago "github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/customfield/monitordb"
	huma "github.com/faciam-dev/gcfm/internal/huma"
	"github.com/faciam-dev/gcfm/internal/tenant"
	md "github.com/faciam-dev/gcfm/pkg/metadata"
)

type tableInfo struct {
	Schema string `json:"schema,omitempty"`
	Name   string `json:"name"`
	Full   string `json:"qualified"`
}

type listTablesOutput struct{ Body []tableInfo }

// db_id from query params
// listTablesParams defines the query parameter for listing tables.
type listTablesParams struct {
	DBID int64 `query:"db_id" required:"true"`
}

// RegisterMetadata registers metadata endpoints.
func RegisterMetadata(api humago.API, h *MetadataHandler) {
	humago.Register(api, humago.Operation{
		OperationID: "listTables",
		Method:      http.MethodGet,
		Path:        "/v1/metadata/tables",
		Summary:     "List tables",
		Tags:        []string{"Metadata"},
	}, h.listTables)
}

// MetadataHandler handles metadata endpoints.
type MetadataHandler struct{ DB *sql.DB }

// listTables returns tables from the monitored database identified by db_id.
func (h *MetadataHandler) listTables(ctx context.Context, p *listTablesParams) (*listTablesOutput, error) {
	tid := tenant.FromContext(ctx)
	mdb, err := monitordb.GetByID(ctx, h.DB, tid, p.DBID)
	if err != nil {
		if errors.Is(err, monitordb.ErrNotFound) {
			return nil, huma.Error422("db_id", "database not found")
		}
		return nil, huma.Error422("db_id", err.Error())
	}
	conn, err := sql.Open(mdb.Driver, mdb.DSN)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	raw, err := listPhysicalTables(ctx, conn, mdb.Driver)
	if err != nil {
		return nil, err
	}

	filtered := md.FilterTables(mdb.Driver, raw)

	var out []tableInfo
	for _, t := range filtered {
		out = append(out, tableInfo{Schema: t.Schema, Name: t.Name, Full: t.Qualified})
	}
	return &listTablesOutput{Body: out}, nil
}

func listPhysicalTables(ctx context.Context, db *sql.DB, driver string) ([]md.TableInfo, error) {
	const base = `
SELECT table_schema, table_name
  FROM information_schema.tables
 WHERE table_type = 'BASE TABLE'`

	var rows *sql.Rows
	var err error

	switch strings.ToLower(driver) {
	case "postgres":
		q := base + `
   AND table_schema NOT IN ('pg_catalog','information_schema','pg_toast')
   AND table_schema NOT LIKE 'pg_temp_%'
 ORDER BY table_schema, table_name`
		rows, err = db.QueryContext(ctx, q)
	case "mysql":
		q := base + `
   AND table_schema = DATABASE()
 ORDER BY table_schema, table_name`
		rows, err = db.QueryContext(ctx, q)
	default:
		rows, err = db.QueryContext(ctx, base+` ORDER BY table_schema, table_name`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []md.TableInfo
	for rows.Next() {
		var s, n string
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		ti := md.TableInfo{Schema: s, Name: n}
		if s != "" && s != "public" {
			ti.Qualified = s + "." + n
		} else {
			ti.Qualified = n
		}
		list = append(list, ti)
	}
	return list, rows.Err()
}
