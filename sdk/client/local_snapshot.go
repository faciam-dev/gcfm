package client

import (
	"context"
	"database/sql"

	"github.com/faciam-dev/gcfm/internal/customfield/snapshot"
	sdk "github.com/faciam-dev/gcfm/sdk"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
)

type snapshotLocal struct {
	dsn     string
	driver  string
	schema  string
	prefix  string
	dialect ormdriver.Dialect
}

// NewLocalSnapshot returns a SnapshotClient that uses direct DB access.
func NewLocalSnapshot(dsn, driver, schema, prefix string) sdk.SnapshotClient {
	if prefix == "" {
		prefix = "gcfm_"
	}
	var dialect ormdriver.Dialect
	if driver == "postgres" {
		dialect = ormdriver.PostgresDialect{}
	} else {
		dialect = ormdriver.MySQLDialect{}
	}
	return &snapshotLocal{dsn: dsn, driver: driver, schema: schema, prefix: prefix, dialect: dialect}
}

func (l *snapshotLocal) open() (*sql.DB, error) {
	return sql.Open(l.driver, l.dsn)
}

func (l *snapshotLocal) List(ctx context.Context, tenant string) ([]sdk.Snapshot, error) {
	db, err := l.open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	recs, err := snapshot.List(ctx, db, l.dialect, l.prefix, tenant, 20)
	if err != nil {
		return nil, err
	}
	out := make([]sdk.Snapshot, len(recs))
	for i, r := range recs {
		out[i] = sdk.Snapshot{ID: r.ID, Semver: r.Semver, TakenAt: r.TakenAt, Author: r.Author}
	}
	return out, nil
}

func (l *snapshotLocal) Create(ctx context.Context, tenant, bump, msg string) (sdk.Snapshot, error) {
	db, err := l.open()
	if err != nil {
		return sdk.Snapshot{}, err
	}
	defer func() { _ = db.Close() }()
	svc := sdk.New(sdk.ServiceConfig{})
	data, err := svc.Export(ctx, sdk.DBConfig{Driver: l.driver, DSN: l.dsn, Schema: l.schema, TablePrefix: l.prefix})
	if err != nil {
		return sdk.Snapshot{}, err
	}
	comp, err := snapshot.Encode(data)
	if err != nil {
		return sdk.Snapshot{}, err
	}
	last, err := snapshot.LatestSemver(ctx, db, l.dialect, l.prefix, tenant)
	if err != nil {
		return sdk.Snapshot{}, err
	}
	if bump == "" {
		bump = "patch"
	}
	ver := snapshot.NextSemver(last, bump)
	snapshotData := snapshot.SnapshotData{
		Tenant: tenant,
		Semver: ver,
		Author: "",
		YAML:   comp,
	}
	rec, err := snapshot.Insert(ctx, db, l.dialect, l.prefix, snapshotData)
	if err != nil {
		return sdk.Snapshot{}, err
	}
	return sdk.Snapshot{ID: rec.ID, Semver: rec.Semver, TakenAt: rec.TakenAt, Author: rec.Author}, nil
}

func (l *snapshotLocal) Apply(ctx context.Context, tenant, ver string) error {
	db, err := l.open()
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	rec, err := snapshot.Get(ctx, db, l.dialect, l.prefix, tenant, ver)
	if err != nil {
		return err
	}
	data, err := snapshot.Decode(rec.YAML)
	if err != nil {
		return err
	}
	svc := sdk.New(sdk.ServiceConfig{})
	_, err = svc.Apply(ctx, sdk.DBConfig{Driver: l.driver, DSN: l.dsn, Schema: l.schema, TablePrefix: l.prefix}, data, sdk.ApplyOptions{})
	return err
}

func (l *snapshotLocal) Diff(ctx context.Context, tenant, from, to string) (string, error) {
	db, err := l.open()
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close() }()
	a, err := snapshot.Get(ctx, db, l.dialect, l.prefix, tenant, from)
	if err != nil {
		return "", err
	}
	b, err := snapshot.Get(ctx, db, l.dialect, l.prefix, tenant, to)
	if err != nil {
		return "", err
	}
	ya, _ := snapshot.Decode(a.YAML)
	yb, _ := snapshot.Decode(b.YAML)
	return sdk.UnifiedDiff(string(ya), string(yb)), nil
}

func (l *snapshotLocal) Mode() string { return "local" }
