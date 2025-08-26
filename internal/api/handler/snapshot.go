package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/pkg/schema"
	"github.com/faciam-dev/gcfm/pkg/audit"
	"github.com/faciam-dev/gcfm/pkg/snapshot"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/pkg/tenant"
	"github.com/faciam-dev/gcfm/sdk"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"gopkg.in/yaml.v3"
)

// SnapshotHandler provides snapshot endpoints.
type SnapshotHandler struct {
	DB          *sql.DB
	Driver      string
	Dialect     ormdriver.Dialect
	DSN         string
	Recorder    *audit.Recorder
	TablePrefix string
}

type snapshotListOutput struct{ Body []schema.Snapshot }
type snapshotListParams struct{}

type snapshotCreateInput struct{ Body schema.SnapshotCreateRequest }

type snapshotCreateOutput struct{ Body schema.Snapshot }

type snapshotDetailParams struct {
	Ver string `path:"ver"`
}
type snapshotDetailOutput struct {
	ContentType string `header:"Content-Type"`
	Body        []byte
}

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
		OperationID: "getSnapshot",
		Method:      http.MethodGet,
		Path:        "/v1/snapshots/{ver}",
		Summary:     "Get snapshot YAML",
		Tags:        []string{"Snapshot"},
		Responses: map[string]*huma.Response{
			"200": {
				Content: map[string]*huma.MediaType{
					"text/yaml": {Schema: &huma.Schema{Type: "string"}},
				},
			},
		},
	}, h.get)

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
	recs, err := snapshot.List(ctx, h.DB, h.Dialect, h.TablePrefix, tid, 20)
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
	reg, err := snapshot.ExportRegistry(ctx, h.DB, h.Dialect, h.TablePrefix, tid)
	if err != nil {
		return nil, err
	}
	if len(reg.Fields) == 0 {
		return nil, fmt.Errorf("registry is empty; run scan first")
	}
	data, err := yaml.Marshal(reg)
	if err != nil {
		return nil, err
	}
	comp, err := snapshot.Encode(data)
	if err != nil {
		return nil, err
	}
	last, err := snapshot.LatestSemver(ctx, h.DB, h.Dialect, h.TablePrefix, tid)
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
	rec, err := snapshot.Insert(ctx, h.DB, h.Dialect, h.TablePrefix, snapshot.SnapshotData{
		Tenant: tid,
		Semver: ver,
		Author: "",
		YAML:   comp,
	})
	if err != nil {
		return nil, err
	}
	// audit log diff against previous snapshot
	var summary string
	if last != "0.0.0" {
		prev, err := snapshot.Get(ctx, h.DB, h.Dialect, h.TablePrefix, tid, last)
		if err == nil {
			prevY, err := snapshot.Decode(prev.YAML)
			if err == nil {
				ch, err := snapshot.DiffYaml(prevY, data)
				if err == nil {
					rep := sdk.CalculateDiff(ch)
					summary = fmt.Sprintf("+%d -%d", rep.Added, rep.Deleted)
				}
			}
		}
	}
	actor := middleware.UserFromContext(ctx)
	_ = h.Recorder.WriteAction(ctx, actor, "snapshot", rec.Semver, summary)
	return &snapshotCreateOutput{Body: schema.Snapshot{ID: rec.ID, Semver: rec.Semver, TakenAt: rec.TakenAt}}, nil
}
func (h *SnapshotHandler) get(ctx context.Context, p *snapshotDetailParams) (*snapshotDetailOutput, error) {
	tid := tenant.FromContext(ctx)
	rec, err := snapshot.Get(ctx, h.DB, h.Dialect, h.TablePrefix, tid, p.Ver)
	if err != nil {
		return nil, err
	}
	y, err := snapshot.Decode(rec.YAML)
	if err != nil {
		return nil, err
	}
	return &snapshotDetailOutput{ContentType: "text/yaml", Body: y}, nil
}

func (h *SnapshotHandler) apply(ctx context.Context, p *snapshotApplyParams) (*struct{}, error) {
	tid := tenant.FromContext(ctx)
	rec, err := snapshot.Get(ctx, h.DB, h.Dialect, h.TablePrefix, tid, p.Ver)
	if err != nil {
		return nil, err
	}
	data, err := snapshot.Decode(rec.YAML)
	if err != nil {
		return nil, err
	}
	current, err := snapshot.SnapshotYaml(ctx, h.DB, h.Driver, h.TablePrefix, tid)
	if err != nil {
		return nil, err
	}
	svc := sdk.New(sdk.ServiceConfig{Recorder: h.Recorder})
	actor := middleware.UserFromContext(ctx)
	if _, err := svc.Apply(ctx, sdk.DBConfig{Driver: h.Driver, DSN: h.DSN, Schema: "public", TablePrefix: h.TablePrefix}, data, sdk.ApplyOptions{Actor: actor}); err != nil {
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
