package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	"github.com/faciam-dev/gcfm/internal/customfield/snapshot"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/tenant"
	"github.com/faciam-dev/gcfm/sdk"
)

// SnapshotHandler provides snapshot endpoints.
type SnapshotHandler struct {
	DB       *sql.DB
	Driver   string
	DSN      string
	Recorder *audit.Recorder
}

type snapshotListOutput struct{ Body []schema.Snapshot }
type snapshotListParams struct{}

type snapshotCreateInput struct{ Body schema.SnapshotCreateRequest }

type snapshotCreateOutput struct{ Body schema.Snapshot }

type snapshotDiffParams struct {
	Ver   string `path:"ver"`
	Other string `path:"other"`
}

type snapshotDiffOutput struct{ Body string }

type snapshotApplyParams struct {
	Ver string `path:"ver"`
}

func RegisterSnapshot(api huma.API, h *SnapshotHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "listSnapshots",
		Method:      http.MethodGet,
		Path:        "/v1/snapshots",
		Summary:     "List registry snapshots",
		Tags:        []string{"Snapshot"},
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "createSnapshotV2",
		Method:      http.MethodPost,
		Path:        "/v1/snapshots",
		Summary:     "Create registry snapshot",
		Tags:        []string{"Snapshot"},
	}, h.create)

	huma.Register(api, huma.Operation{
		OperationID: "diffSnapshots",
		Method:      http.MethodGet,
		Path:        "/v1/snapshots/{ver}/diff/{other}",
		Summary:     "Diff two snapshots",
		Tags:        []string{"Snapshot"},
	}, h.diff)

	huma.Register(api, huma.Operation{
		OperationID: "applySnapshot",
		Method:      http.MethodPost,
		Path:        "/v1/snapshots/{ver}/apply",
		Summary:     "Apply registry snapshot",
		Tags:        []string{"Snapshot"},
	}, h.apply)
}

func (h *SnapshotHandler) list(ctx context.Context, _ *snapshotListParams) (*snapshotListOutput, error) {
	tid := tenant.FromContext(ctx)
	recs, err := snapshot.List(ctx, h.DB, h.Driver, tid, 20)
	if err != nil {
		return nil, err
	}
	out := make([]schema.Snapshot, len(recs))
	for i, r := range recs {
		out[i] = schema.Snapshot{ID: r.ID, Semver: r.Semver, TakenAt: r.TakenAt, Author: r.Author}
	}
	return &snapshotListOutput{Body: out}, nil
}

func (h *SnapshotHandler) create(ctx context.Context, in *snapshotCreateInput) (*snapshotCreateOutput, error) {
        tid := tenant.FromContext(ctx)
        // generate YAML from registry DB for snapshot
        data, err := snapshot.SnapshotYaml(ctx, h.DB, h.Driver, tid)
        if err != nil {
                return nil, err
        }
        comp, err := snapshot.Encode(data)
	if err != nil {
		return nil, err
	}
	last, err := snapshot.LatestSemver(ctx, h.DB, h.Driver, tid)
	if err != nil {
		return nil, err
	}
	bump := strings.ToLower(in.Body.Bump)
	if bump == "" {
		bump = "patch"
	}
	ver := in.Body.Semver
	if ver == "" {
		ver = snapshot.NextSemver(last, bump)
	}
	rec, err := snapshot.Insert(ctx, h.DB, h.Driver, tid, ver, "", comp)
	if err != nil {
		return nil, err
	}
	// audit log diff against previous snapshot
	var summary string
	if last != "0.0.0" {
		prev, err := snapshot.Get(ctx, h.DB, h.Driver, tid, last)
		if err == nil {
			prevY, _ := snapshot.Decode(prev.YAML)
			ch, err := snapshot.DiffYaml(prevY, data)
			if err == nil {
				rep := sdk.CalculateDiff(ch)
				summary = fmt.Sprintf("+%d -%d", rep.Added, rep.Deleted)
			}
		}
	}
	actor := middleware.UserFromContext(ctx)
	_ = h.Recorder.WriteAction(ctx, actor, "snapshot", rec.Semver, summary)
	return &snapshotCreateOutput{Body: schema.Snapshot{ID: rec.ID, Semver: rec.Semver, TakenAt: rec.TakenAt}}, nil
}

func (h *SnapshotHandler) diff(ctx context.Context, p *snapshotDiffParams) (*snapshotDiffOutput, error) {
	tid := tenant.FromContext(ctx)
	a, err := snapshot.Get(ctx, h.DB, h.Driver, tid, p.Ver)
	if err != nil {
		return nil, err
	}
	b, err := snapshot.Get(ctx, h.DB, h.Driver, tid, p.Other)
	if err != nil {
		return nil, err
	}
	ya, _ := snapshot.Decode(a.YAML)
	yb, _ := snapshot.Decode(b.YAML)
	diff := sdk.UnifiedDiff(string(ya), string(yb))
	return &snapshotDiffOutput{Body: diff}, nil
}

func (h *SnapshotHandler) apply(ctx context.Context, p *snapshotApplyParams) (*struct{}, error) {
	tid := tenant.FromContext(ctx)
	rec, err := snapshot.Get(ctx, h.DB, h.Driver, tid, p.Ver)
	if err != nil {
		return nil, err
	}
	data, err := snapshot.Decode(rec.YAML)
	if err != nil {
		return nil, err
	}
	current, err := snapshot.SnapshotYaml(ctx, h.DB, h.Driver, tid)
	if err != nil {
		return nil, err
	}
	svc := sdk.New(sdk.ServiceConfig{Recorder: h.Recorder})
	actor := middleware.UserFromContext(ctx)
	if _, err := svc.Apply(ctx, sdk.DBConfig{Driver: h.Driver, DSN: h.DSN, Schema: "public"}, data, sdk.ApplyOptions{Actor: actor}); err != nil {
		return nil, err
	}
	ch, err := snapshot.DiffYaml(current, data)
	if err == nil {
		rep := sdk.CalculateDiff(ch)
		summary := fmt.Sprintf("+%d -%d", rep.Added, rep.Deleted)
		_ = h.Recorder.WriteAction(ctx, actor, "rollback", p.Ver, summary)
	}
	return &struct{}{}, nil
}
