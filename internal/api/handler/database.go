package handler

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
	"github.com/faciam-dev/gcfm/internal/monitordb"
	"github.com/faciam-dev/gcfm/internal/tenant"
	"github.com/faciam-dev/gcfm/pkg/crypto"
)

// DatabaseHandler manages monitored databases via REST.
type DatabaseHandler struct {
	Repo *monitordb.Repo
}

type createDBInput struct{ Body schema.CreateDatabase }
type createDBOutput struct{ Body schema.Database }

type listDBOutput struct{ Body []schema.Database }

type idParam struct {
	ID int64 `path:"id"`
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
		DefaultStatus: http.StatusAccepted,
	}, h.scan)
}

func (h *DatabaseHandler) create(ctx context.Context, in *createDBInput) (*createDBOutput, error) {
	tid := tenant.FromContext(ctx)
	enc, err := crypto.Encrypt([]byte(in.Body.DSN))
	if err != nil {
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

func (h *DatabaseHandler) scan(ctx context.Context, in *idParam) (*struct{}, error) {
	tid := tenant.FromContext(ctx)
	if err := monitordb.ScanDatabase(ctx, h.Repo, in.ID, tid); err != nil {
		return nil, err
	}
	return &struct{}{}, nil
}
