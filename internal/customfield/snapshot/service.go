package snapshot

import (
	"context"
	"database/sql"

	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/customfield/registry/codec"
	sdk "github.com/faciam-dev/gcfm/sdk"
)

// SnapshotYaml dumps registry metadata as YAML using the provided DB connection.
func SnapshotYaml(ctx context.Context, db *sql.DB, driver, tenant string) ([]byte, error) {
	metas, err := registry.LoadSQL(ctx, db, registry.DBConfig{Schema: "public", Driver: driver})
	if err != nil {
		return nil, err
	}
	return codec.EncodeYAML(metas)
}

// ApplyYaml applies the given YAML to the registry using Service.Apply.
func ApplyYaml(ctx context.Context, dsn, driver, tenant string, yaml []byte, rec *audit.Recorder) (sdk.DiffReport, error) {
	svc := sdk.New(sdk.ServiceConfig{Recorder: rec})
	return svc.Apply(ctx, sdk.DBConfig{Driver: driver, DSN: dsn, Schema: "public"}, yaml, sdk.ApplyOptions{})
}

// DiffYaml returns the registry changes between two YAML documents.
func DiffYaml(a, b []byte) ([]registry.Change, error) {
	fa, err := codec.DecodeYAML(a)
	if err != nil {
		return nil, err
	}
	fb, err := codec.DecodeYAML(b)
	if err != nil {
		return nil, err
	}
	return registry.Diff(fa, fb), nil
}
