package handler

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
	"github.com/faciam-dev/gcfm/internal/customfield/snapshot"
	sdk "github.com/faciam-dev/gcfm/sdk"
)

type RegistryHandler struct {
	DB     *sql.DB
	Driver string
	DSN    string
}

type applyInput struct {
	Body schema.ApplyRequest
}

type applyOutput struct {
	Body any
}

type snapshotInput struct {
	Body schema.SnapshotRequest
}

func RegisterRegistry(api huma.API, h *RegistryHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "applyRegistry",
		Method:      http.MethodPost,
		Path:        "/v1/apply",
		Summary:     "Apply registry yaml",
		Tags:        []string{"Registry"},
	}, h.apply)
	huma.Register(api, huma.Operation{
		OperationID: "createSnapshot",
		Method:      http.MethodPost,
		Path:        "/v1/snapshot",
		Summary:     "Create registry snapshot",
		Tags:        []string{"Registry"},
	}, h.snapshot)
}

func (h *RegistryHandler) apply(ctx context.Context, in *applyInput) (*applyOutput, error) {
	svc := sdk.New(sdk.ServiceConfig{})
	rep, err := svc.Apply(ctx, sdk.DBConfig{Driver: h.Driver, DSN: h.DSN, Schema: "public"}, []byte(in.Body.YAML), sdk.ApplyOptions{DryRun: in.Body.DryRun})
	if err != nil {
		return nil, err
	}
	return &applyOutput{Body: rep}, nil
}

func (h *RegistryHandler) snapshot(ctx context.Context, in *snapshotInput) (*struct{}, error) {
	dest := "."
	if in.Body.Dest != "" {
		dest = in.Body.Dest
	}
	if err := snapshot.Export(ctx, h.DB, "public", snapshot.LocalDir{Path: dest}); err != nil {
		return nil, err
	}
	return &struct{}{}, nil
}
