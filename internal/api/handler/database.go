package handler

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/faciam-dev/gcfm/internal/domain/capability"
	"github.com/faciam-dev/gcfm/internal/events"
	huma "github.com/faciam-dev/gcfm/internal/huma"
	"github.com/faciam-dev/gcfm/internal/monitordb"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	capuses "github.com/faciam-dev/gcfm/internal/usecase/capability"
	"github.com/faciam-dev/gcfm/pkg/audit"
	"github.com/faciam-dev/gcfm/pkg/crypto"
	cfmdb "github.com/faciam-dev/gcfm/pkg/monitordb"
	"github.com/faciam-dev/gcfm/pkg/schema"
	"github.com/faciam-dev/gcfm/pkg/tenant"
	pkgutil "github.com/faciam-dev/gcfm/pkg/util"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

// DatabaseHandler manages monitored databases via REST.
type DatabaseHandler struct {
	Repo         *monitordb.Repo
	Recorder     *audit.Recorder
	Enf          *casbin.Enforcer
	Capabilities capuses.Service
}

type createDBInput struct{ Body schema.CreateDatabase }
type createDBOutput struct{ Body schema.Database }

type listDBOutput struct{ Body []schema.Database }

type updateDBInput struct {
	ID   int64 `path:"id"`
	Body updateDBBody
}

type updateDBBody struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
	DSN    string `json:"dsn"`
}

type dbOutput struct{ Body schema.Database }

type idParam struct {
	ID int64 `path:"id"`
}

type scanParams struct {
	ID      int64 `path:"id"`
	Verbose bool  `query:"verbose"`
}

type scanOutput struct{ Body schema.ScanResult }

type dbTablesParams struct {
	ID int64 `path:"id"`
}

type dbTablesOutput struct{ Body []string }

type capabilitiesOutput struct{ Body capability.Capabilities }

// GET /v1/databases/{id}/tables returns the list of tables for the monitored DB specified by db_id
func (h *DatabaseHandler) listTables(ctx context.Context, p *dbTablesParams) (*dbTablesOutput, error) {
	tid := tenant.FromContext(ctx)
	mdb, err := cfmdb.GetByID(ctx, h.Repo.DB, h.Repo.Dialect, h.Repo.TablePrefix, tid, p.ID)
	if err != nil {
		if errors.Is(err, cfmdb.ErrNotFound) {
			return nil, huma.Error422("id", "database not found")
		}
		return nil, huma.Error422("id", err.Error())
	}
	target, err := sql.Open(mdb.Driver, mdb.DSN)
	if err != nil {
		return nil, err
	}
	defer target.Close()

	dialect := pkgutil.DialectFromDriver(mdb.Driver)
	if _, ok := dialect.(pkgutil.UnsupportedDialect); ok {
		return nil, huma.Error422("driver", "unsupported driver")
	}

	type tbl struct {
		Name string `db:"table_name"`
	}
	q := query.New(target, "information_schema.tables", dialect).
		Select("table_name").
		OrderBy("table_name", "asc")
	switch dialect.(type) {
	case ormdriver.PostgresDialect:
		schema := mdb.Schema
		if schema == "" {
			schema = "public"
		}
		q.Where("table_schema", schema)
	case ormdriver.MySQLDialect:
		q.WhereRaw("table_schema = DATABASE()", nil)
	}
	var rows []tbl
	if err := q.WithContext(ctx).Get(&rows); err != nil {
		return nil, err
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Name
	}
	return &dbTablesOutput{Body: out}, nil
}

func (h *DatabaseHandler) capabilities(ctx context.Context, p *idParam) (*capabilitiesOutput, error) {
	if h.Capabilities == nil {
		return nil, huma.NewError(http.StatusNotImplemented, "capability service not configured")
	}
	tid := tenant.FromContext(ctx)
	caps, err := h.Capabilities.Get(ctx, tid, p.ID)
	if err != nil {
		if errors.Is(err, cfmdb.ErrNotFound) {
			return nil, huma.Error422("id", "database not found")
		}
		if errors.Is(err, capuses.ErrAdapterNotFound) {
			return nil, huma.Error422("driver", "capabilities not available for driver")
		}
		return nil, err
	}
	return &capabilitiesOutput{Body: caps}, nil
}

// RegisterDatabase registers database endpoints.
func RegisterDatabase(api huma.API, h *DatabaseHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "listDatabases",
		Method:      http.MethodGet,
		Path:        "/v1/databases",
		Summary:     "List monitored databases",
		Tags:        []string{"Database"},
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID:   "createDatabase",
		Method:        http.MethodPost,
		Path:          "/v1/databases",
		Summary:       "Add monitored database",
		Tags:          []string{"Database"},
		DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "updateDatabase",
		Method:      http.MethodPut,
		Path:        "/v1/databases/{id}",
		Summary:     "Update monitored database",
		Tags:        []string{"Database"},
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID:   "deleteDatabase",
		Method:        http.MethodDelete,
		Path:          "/v1/databases/{id}",
		Summary:       "Delete monitored database",
		Tags:          []string{"Database"},
		DefaultStatus: http.StatusNoContent,
	}, h.delete)
	huma.Register(api, huma.Operation{
		OperationID:   "scanDatabase",
		Method:        http.MethodPost,
		Path:          "/v1/databases/{id}/scan",
		Summary:       "Scan monitored database",
		Tags:          []string{"Database"},
		DefaultStatus: http.StatusOK,
	}, h.scan)

	huma.Register(api, huma.Operation{
		OperationID: "databaseCapabilities",
		Method:      http.MethodGet,
		Path:        "/v1/databases/{id}/capabilities",
		Summary:     "List driver capabilities",
		Tags:        []string{"Database"},
	}, h.capabilities)

	huma.Register(api, huma.Operation{
		OperationID: "listDbTables",
		Method:      http.MethodGet,
		Path:        "/v1/databases/{id}/tables",
		Summary:     "List tables in monitored database",
		Tags:        []string{"Database"},
	}, h.listTables)
}

func (h *DatabaseHandler) create(ctx context.Context, in *createDBInput) (*createDBOutput, error) {
	tid := tenant.FromContext(ctx)
	enc, err := crypto.Encrypt([]byte(in.Body.DSN))
	if err != nil {
		if errors.Is(err, crypto.ErrKeyNotSet) {
			return nil, huma.NewError(http.StatusInternalServerError, err.Error())
		}
		return nil, err
	}
	id, err := h.Repo.Create(ctx, monitordb.Database{TenantID: tid, Name: in.Body.Name, Driver: in.Body.Driver, DSN: in.Body.DSN, DSNEnc: enc})
	if err != nil {
		return nil, err
	}
	return &createDBOutput{Body: schema.Database{ID: id, Name: in.Body.Name, Driver: in.Body.Driver}}, nil
}

func (h *DatabaseHandler) list(ctx context.Context, _ *struct{}) (*listDBOutput, error) {
	tid := tenant.FromContext(ctx)
	sub := middleware.UserFromContext(ctx)

	dbs, err := h.Repo.List(ctx, tid)
	if err != nil {
		return nil, err
	}
	items := make([]schema.Database, len(dbs))
	for i, d := range dbs {
		canWrite := false
		if h.Enf != nil {
			if ok, _ := h.Enf.Enforce(sub, "/v1/databases", http.MethodPost); ok {
				canWrite = true
			} else if ok, _ := h.Enf.Enforce(sub, fmt.Sprintf("/v1/databases/%d", d.ID), http.MethodPut); ok {
				canWrite = true
			} else if ok, _ := h.Enf.Enforce(sub, fmt.Sprintf("/v1/databases/%d", d.ID), http.MethodDelete); ok {
				canWrite = true
			}
		}
		var dsn []byte
		if len(d.DSNEnc) > 0 {
			dec, derr := crypto.Decrypt(d.DSNEnc)
			if derr != nil {
				return nil, huma.NewError(http.StatusInternalServerError, fmt.Sprintf("failed to decrypt DSN for database ID %d: %v", d.ID, derr))
			}
			dsn = dec
		} else {
			dsn = []byte(d.DSN)
		}
		encStr := ""
		if len(d.DSNEnc) > 0 {
			encStr = base64.StdEncoding.EncodeToString(d.DSNEnc)
		}
		items[i] = schema.Database{
			ID:        d.ID,
			Name:      d.Name,
			Driver:    d.Driver,
			DSN:       string(dsn),
			DSNEnc:    encStr,
			CreatedAt: d.CreatedAt,
		}
		if !canWrite {
			items[i].DSN = ""
			items[i].DSNEnc = ""
		}
	}
	return &listDBOutput{Body: items}, nil
}

func (h *DatabaseHandler) delete(ctx context.Context, in *idParam) (*struct{}, error) {
	tid := tenant.FromContext(ctx)
	if err := h.Repo.Delete(ctx, tid, in.ID); err != nil {
		return nil, err
	}
	return &struct{}{}, nil
}

func (h *DatabaseHandler) update(ctx context.Context, in *updateDBInput) (*dbOutput, error) {
	tid := tenant.FromContext(ctx)
	enc, err := crypto.Encrypt([]byte(in.Body.DSN))
	if err != nil {
		if errors.Is(err, crypto.ErrKeyNotSet) {
			return nil, huma.NewError(http.StatusInternalServerError, err.Error())
		}
		return nil, err
	}
	if err := h.Repo.Update(ctx, tid, in.ID, in.Body.Name, in.Body.Driver, enc); err != nil {
		return nil, err
	}
	d, err := h.Repo.Get(ctx, tid, in.ID)
	if err != nil {
		return nil, err
	}
	res := schema.Database{ID: d.ID, Name: d.Name, Driver: d.Driver, CreatedAt: d.CreatedAt}
	return &dbOutput{Body: res}, nil
}

func (h *DatabaseHandler) scan(ctx context.Context, in *scanParams) (*scanOutput, error) {
	tid := tenant.FromContext(ctx)
	tables, ins, upd, skipped, err := monitordb.ScanDatabase(ctx, h.Repo, in.ID, tid)
	if err != nil {
		return nil, err
	}
	res := schema.ScanResult{Total: ins + upd, Inserted: ins, Updated: upd, Skipped: len(skipped)}
	if in.Verbose {
		res.SkipDetail = make([]schema.SkipInfo, len(skipped))
		for i, s := range skipped {
			res.SkipDetail[i] = schema.SkipInfo{Table: s.Table, Column: s.Column, Reason: s.Reason}
		}
	}
	payload := map[string]any{"total": res.Total, "inserted": res.Inserted, "updated": res.Updated, "skipped": res.Skipped, "db_id": in.ID, "tables": tables}
	actor := middleware.UserFromContext(ctx)
	if h.Recorder != nil {
		_ = h.Recorder.WriteJSON(ctx, actor, "scan", payload)
	}
	events.Emit(ctx, events.Event{Name: "cf.scan", Time: time.Now(), Data: payload, ID: fmt.Sprintf("%d", in.ID)})
	return &scanOutput{Body: res}, nil
}
