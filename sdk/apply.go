package sdk

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"gopkg.in/yaml.v3"

	"github.com/faciam-dev/gcfm/internal/customfield/migrator"
	"github.com/faciam-dev/gcfm/internal/customfield/notifier"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/customfield/registry/codec"
	"github.com/faciam-dev/gcfm/internal/metrics"
)

func recordApplyError(table string) {
	metrics.ApplyErrors.WithLabelValues(table, "db").Inc()
}

// Apply updates the registry with the provided YAML metadata.
// Possible errors: ErrValidatorNotFound, context.Canceled, or database errors.
func (s *service) Apply(ctx context.Context, cfg DBConfig, data []byte, opts ApplyOptions) (DiffReport, error) {
	metas, err := codec.DecodeYAML(data)
	if err != nil {
		return DiffReport{}, err
	}

	var hdr struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(data, &hdr); err != nil {
		return DiffReport{}, err
	}
	if hdr.Version != "" {
		mig := migrator.New()
		drv := cfg.Driver
		if drv == "" {
			var derr error
			drv, derr = detectDriver(cfg.DSN)
			if derr != nil {
				return DiffReport{}, derr
			}
		}
		if drv == "mysql" || drv == "postgres" {
			db, err := sql.Open(drv, cfg.DSN)
			if err != nil {
				return DiffReport{}, err
			}
			defer db.Close()
			cur, err := mig.Current(ctx, db)
			if err != nil && err != migrator.ErrNoVersionTable {
				return DiffReport{}, err
			}
			curSem := mig.SemVer(cur)
			if curSem == "" {
				slog.Warn("unexpected empty semver", "cur", cur)
				return DiffReport{}, fmt.Errorf("cannot map registry version %d to semver", cur)
			}
			ok, err := semverLT(curSem, hdr.Version)
			if err != nil {
				return DiffReport{}, err
			}
			if ok {
				return DiffReport{}, fmt.Errorf("registry schema %s required, current %s", hdr.Version, curSem)
			}
		}
	}

	current, err := s.Scan(ctx, cfg)
	if err != nil {
		return DiffReport{}, err
	}
	changes := registry.Diff(current, metas)

	var upserts []registry.FieldMeta
	var dels []registry.FieldMeta
	for _, c := range changes {
		switch c.Type {
		case registry.ChangeAdded:
			upserts = append(upserts, *c.New)
		case registry.ChangeDeleted:
			dels = append(dels, *c.Old)
		case registry.ChangeUpdated:
			upserts = append(upserts, *c.New)
		}
	}
	rep := CalculateDiff(changes)
	if opts.DryRun {
		return rep, nil
	}

	drv := cfg.Driver
	if drv == "" {
		var derr error
		drv, derr = detectDriver(cfg.DSN)
		if derr != nil {
			return rep, derr
		}
	}
	switch drv {
	case "postgres", "mysql":
		db, err := sql.Open(drv, cfg.DSN)
		if err != nil {
			return rep, err
		}
		defer db.Close()
		if err := registry.DeleteSQL(ctx, db, drv, dels); err != nil {
			if len(dels) > 0 {
				recordApplyError(dels[0].TableName)
			}
			return rep, err
		}
		if err := registry.UpsertSQL(ctx, db, drv, upserts); err != nil {
			if len(upserts) > 0 {
				recordApplyError(upserts[0].TableName)
			}
			return rep, err
		}
	case "sqlmock":
		db, err := sql.Open("sqlmock", cfg.DSN)
		if err != nil {
			return rep, err
		}
		defer db.Close()
		if err := registry.DeleteSQL(ctx, db, "mysql", dels); err != nil {
			if len(dels) > 0 {
				recordApplyError(dels[0].TableName)
			}
			return rep, err
		}
		if err := registry.UpsertSQL(ctx, db, "mysql", upserts); err != nil {
			if len(upserts) > 0 {
				recordApplyError(upserts[0].TableName)
			}
			return rep, err
		}
	case "mongo":
		cli, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.DSN))
		if err != nil {
			return rep, err
		}
		defer cli.Disconnect(ctx)
		if err := registry.DeleteMongo(ctx, cli, registry.DBConfig{Schema: cfg.Schema}, dels); err != nil {
			if len(dels) > 0 {
				recordApplyError(dels[0].TableName)
			}
			return rep, err
		}
		if err := registry.UpsertMongo(ctx, cli, registry.DBConfig{Schema: cfg.Schema}, upserts); err != nil {
			if len(upserts) > 0 {
				recordApplyError(upserts[0].TableName)
			}
			return rep, err
		}
	default:
		return rep, fmt.Errorf("unsupported driver: %s", drv)
	}
	if !opts.DryRun {
		for _, c := range changes {
			switch c.Type {
			case registry.ChangeAdded:
				_ = s.recorder.Write(ctx, opts.Actor, nil, c.New)
			case registry.ChangeDeleted:
				_ = s.recorder.Write(ctx, opts.Actor, c.Old, nil)
			case registry.ChangeUpdated:
				_ = s.recorder.Write(ctx, opts.Actor, c.Old, c.New)
			}
		}
		if s.notifier != nil {
			_ = s.notifier.Emit(ctx, notifier.DiffReport{Added: rep.Added, Deleted: rep.Deleted, Updated: rep.Updated})
		}
	}

	return rep, nil
}

// CalculateDiff returns counts of added, deleted and updated changes.
func CalculateDiff(changes []registry.Change) DiffReport {
	var rep DiffReport
	for _, c := range changes {
		switch c.Type {
		case registry.ChangeAdded:
			rep.Added++
		case registry.ChangeDeleted:
			rep.Deleted++
		case registry.ChangeUpdated:
			rep.Updated++
		}
	}
	return rep
}
