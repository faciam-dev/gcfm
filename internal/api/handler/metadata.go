package handler

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
	"github.com/faciam-dev/gcfm/internal/server/reserved"
	"github.com/faciam-dev/gcfm/internal/tenant"
)

// MetadataHandler handles metadata endpoints.
type MetadataHandler struct {
	DB     *sql.DB
	Driver string
}

// tablesParams is the query parameters for list tables.
type tablesParams struct {
	DBID int64 `query:"db_id" required:"true"`
}

type tablesOutput struct{ Body []schema.TableMeta }

// RegisterMetadata registers metadata endpoints.
func RegisterMetadata(api huma.API, h *MetadataHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "listTables",
		Method:      http.MethodGet,
		Path:        "/v1/metadata/tables",
		Summary:     "List tables",
		Tags:        []string{"Metadata"},
	}, h.listTables)
}

func (h *MetadataHandler) listTables(ctx context.Context, p *tablesParams) (*tablesOutput, error) {
	tenantID := tenant.FromContext(ctx)
	var query string
	switch h.Driver {
	case "postgres":
		query = `SELECT DISTINCT table_name FROM gcfm_custom_fields WHERE db_id=$1 AND tenant_id=$2 ORDER BY table_name`
	default:
		query = "SELECT DISTINCT table_name FROM gcfm_custom_fields WHERE db_id=? AND tenant_id=? ORDER BY table_name"
	}
	rows, err := h.DB.QueryContext(ctx, query, p.DBID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []schema.TableMeta
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, schema.TableMeta{Table: name, Reserved: reserved.Is(name)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &tablesOutput{Body: out}, nil
}
