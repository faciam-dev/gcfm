package handler

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
	"github.com/faciam-dev/gcfm/internal/customfield/snapshot"
	sdk "github.com/faciam-dev/gcfm/sdk"
)

// snapshotBaseDir defines the directory where registry snapshots are stored.
// Any destination path provided by the user must remain inside this directory.
const snapshotBaseDir = "./snapshots"

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
	base := filepath.Clean(snapshotBaseDir)
	dest := base
	if in.Body.Dest != "" {
		p := filepath.Clean(in.Body.Dest)
		dest = filepath.Join(base, p)
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return nil, err
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return nil, err
	}
	if absDest != absBase && !strings.HasPrefix(absDest, absBase+string(os.PathSeparator)) {
		return nil, huma.Error400BadRequest("invalid dest path")
	}
	if err := snapshot.Export(ctx, h.DB, "public", snapshot.LocalDir{Path: absDest}); err != nil {
		return nil, err
	}
	return &struct{}{}, nil
}
