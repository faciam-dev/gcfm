package handler

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
	"github.com/faciam-dev/gcfm/internal/driver/mysql"
	"github.com/faciam-dev/gcfm/internal/driver/postgres"
	"github.com/faciam-dev/gcfm/internal/metadata"
)

// MetadataHandler handles metadata endpoints.
type MetadataHandler struct {
	DB     *sql.DB
	Driver string
}

// tablesParams is the query parameters for list tables.
type tablesParams struct {
	Schema string `query:"schema"`
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
	schemaName := p.Schema
	if schemaName == "" {
		switch h.Driver {
		case "postgres":
			schemaName = "public"
		case "mysql":
			if err := h.DB.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&schemaName); err != nil {
				return nil, err
			}
		default:
			return nil, huma.Error400BadRequest("unsupported driver")
		}
	}
	var tables []metadata.Table
	var err error
	switch h.Driver {
	case "postgres":
		tables, err = postgres.ListTables(ctx, h.DB, schemaName)
	case "mysql":
		tables, err = mysql.ListTables(ctx, h.DB, schemaName)
	default:
		return nil, huma.Error400BadRequest("unsupported driver")
	}
	if err != nil {
		return nil, err
	}
	out := make([]schema.TableMeta, 0, len(tables))
	for _, t := range tables {
		out = append(out, schema.TableMeta{Table: t.Name, Comment: t.Comment})
	}
	return &tablesOutput{Body: out}, nil
}
