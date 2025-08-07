package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	"github.com/faciam-dev/gcfm/internal/events"
	"github.com/faciam-dev/gcfm/internal/monitordb"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/tenant"
	"github.com/faciam-dev/gcfm/pkg/crypto"
)

// DatabaseHandler manages monitored databases via REST.
type DatabaseHandler struct {
	Repo     *monitordb.Repo
	Recorder *audit.Recorder
}

type createDBInput struct{ Body schema.CreateDatabase }
type createDBOutput struct{ Body schema.Database }

type listDBOutput struct{ Body []schema.Database }

type updateDBInput struct {
	ID   int64 `path:"id"`
	Body updateDBBody
}

type updateDBBody struct {
	Name    string `json:"name"`
	Driver  string `json:"driver"`
	DSN     string `json:"dsn"`
	Comment string `json:"comment,omitempty"`
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
	id, err := h.Repo.Create(ctx, monitordb.Database{TenantID: tid, Name: in.Body.Name, Driver: in.Body.Driver, DSNEnc: enc})
	if err != nil {
		return nil, err
	}
	return &createDBOutput{Body: schema.Database{ID: id, Name: in.Body.Name, Driver: in.Body.Driver}}, nil
}

func (h *DatabaseHandler) list(ctx context.Context, _ *struct{}) (*listDBOutput, error) {
	tid := tenant.FromContext(ctx)
	dbs, err := h.Repo.List(ctx, tid)
	if err != nil {
		return nil, err
	}
	res := make([]schema.Database, len(dbs))
	for i, d := range dbs {
		res[i] = schema.Database{ID: d.ID, Name: d.Name, Driver: d.Driver, CreatedAt: d.CreatedAt}
	}
	return &listDBOutput{Body: res}, nil
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
