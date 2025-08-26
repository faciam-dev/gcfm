package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	humago "github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/pkg/monitordb"
	huma "github.com/faciam-dev/gcfm/internal/huma"
	"github.com/faciam-dev/gcfm/pkg/tenant"
	md "github.com/faciam-dev/gcfm/pkg/metadata"
	pkgutil "github.com/faciam-dev/gcfm/pkg/util"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

type tableInfo struct {
	Schema string `json:"schema,omitempty"`
	Name   string `json:"name"`
	Full   string `json:"qualified"`
}

type tablesOutput struct {
	Body struct {
		Items []tableInfo `json:"items"`
	}
}

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
type MetadataHandler struct {
	DB          *sql.DB
	Dialect     ormdriver.Dialect
	TablePrefix string
}

// listTables returns tables from the monitored database identified by db_id.
func (h *MetadataHandler) listTables(ctx context.Context, p *listTablesParams) (*tablesOutput, error) {
	tid := tenant.FromContext(ctx)
	mdb, err := monitordb.GetByID(ctx, h.DB, h.Dialect, h.TablePrefix, tid, p.DBID)
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

	dialect := pkgutil.DialectFromDriver(mdb.Driver)
	if _, ok := dialect.(pkgutil.UnsupportedDialect); ok {
		return nil, huma.Error422("db_id", "unsupported driver")
	}
	raw, err := listPhysicalTables(ctx, conn, dialect)
	if err != nil {
		return nil, err
	}

	filtered := md.FilterTables(mdb.Driver, raw)

	out := &tablesOutput{}
	for _, t := range filtered {
		out.Body.Items = append(out.Body.Items, tableInfo{Schema: t.Schema, Name: t.Name, Full: t.Qualified})
	}
	return out, nil
}

func listPhysicalTables(ctx context.Context, db *sql.DB, dialect ormdriver.Dialect) ([]md.TableInfo, error) {
	q := query.New(db, "information_schema.tables", dialect).
		SelectRaw("table_schema AS table_schema").
		SelectRaw("table_name AS table_name").
		Where("table_type", "BASE TABLE").
		OrderBy("table_schema", "asc").
		OrderBy("table_name", "asc")
	switch dialect.(type) {
	case ormdriver.PostgresDialect:
		q.WhereRaw("table_schema NOT IN ('pg_catalog','information_schema','pg_toast')", nil).
			WhereRaw("table_schema NOT LIKE 'pg_temp_%'", nil)
	case ormdriver.MySQLDialect:
		q.WhereRaw("table_schema = DATABASE()", nil)
	}
	type row struct {
		Schema string `db:"table_schema"`
		Name   string `db:"table_name"`
	}
	var rs []row
	if err := q.WithContext(ctx).Get(&rs); err != nil {
		return nil, err
	}
	list := make([]md.TableInfo, 0, len(rs))
	for _, r := range rs {
		ti := md.TableInfo{Schema: r.Schema, Name: r.Name}
		if r.Schema != "" && r.Schema != "public" {
			ti.Qualified = r.Schema + "." + r.Name
		} else {
			ti.Qualified = r.Name
		}
		list = append(list, ti)
	}
	return list, nil
}
