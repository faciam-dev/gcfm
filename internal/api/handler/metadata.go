package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	humago "github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/customfield/monitordb"
	huma "github.com/faciam-dev/gcfm/internal/huma"
	"github.com/faciam-dev/gcfm/internal/tenant"
)

type tableInfo struct {
	Table    string `json:"table"`
	Reserved bool   `json:"reserved"`
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
	target, err := sql.Open(mdb.Driver, mdb.DSN)
	if err != nil {
		return nil, err
	}
	defer target.Close()

	var rows *sql.Rows
	switch mdb.Driver {
	case "mysql":
		rows, err = target.QueryContext(ctx, `
          SELECT table_name
            FROM information_schema.tables
           WHERE table_schema = DATABASE()
           ORDER BY table_name`)
	case "postgres":
		schema := mdb.Schema
		if schema == "" {
			schema = "public"
		}
		rows, err = target.QueryContext(ctx, `
          SELECT table_name
            FROM information_schema.tables
           WHERE table_schema = $1
           ORDER BY table_name`, schema)
	default:
		return nil, huma.Error422("driver", "unsupported driver")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reserved := map[string]bool{
		"schema_migrations":     true,
		"tenants":               true,
		"tenant_confirm_tokens": true,
		"casbin_rule":           true,
		"roles":                 true,
		"subjects":              true,
		"oauth_tokens":          true,
		"api_keys":              true,
	}

	var out []tableInfo
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, tableInfo{Table: t, Reserved: reserved[t]})
	}
	return &listTablesOutput{Body: out}, nil
}
