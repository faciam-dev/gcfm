package sdk

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"gopkg.in/yaml.v3"

	"github.com/faciam-dev/gcfm/internal/customfield/migrator"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/customfield/registry/codec"
)

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
	if err := yaml.Unmarshal(data, &hdr); err == nil {
		mig := migrator.New()
		if req, ok := mig.SemVerToInt(hdr.Version); ok {
			drv := cfg.Driver
			if drv == "" {
				drv = detectDriver(cfg.DSN)
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
				if cur < req {
					return DiffReport{}, fmt.Errorf("registry schema %s required, current %s", hdr.Version, mig.SemVer(cur))
				}
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
		drv = detectDriver(cfg.DSN)
	}
	switch drv {
	case "postgres", "mysql":
		db, err := sql.Open(drv, cfg.DSN)
		if err != nil {
			return rep, err
		}
		defer db.Close()
		if err := registry.DeleteSQL(ctx, db, drv, dels); err != nil {
			return rep, err
		}
		if err := registry.UpsertSQL(ctx, db, drv, upserts); err != nil {
			return rep, err
		}
	case "mongo":
		cli, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.DSN))
		if err != nil {
			return rep, err
		}
		defer cli.Disconnect(ctx)
		if err := registry.DeleteMongo(ctx, cli, registry.DBConfig{Schema: cfg.Schema}, dels); err != nil {
			return rep, err
		}
		if err := registry.UpsertMongo(ctx, cli, registry.DBConfig{Schema: cfg.Schema}, upserts); err != nil {
			return rep, err
		}
	default:
		return rep, fmt.Errorf("unsupported driver: %s", drv)
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
