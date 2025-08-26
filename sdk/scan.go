package sdk

import (
	"context"
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/faciam-dev/gcfm/pkg/registry"
	mongoscanner "github.com/faciam-dev/gcfm/pkg/driver/mongo"
	mysqlscanner "github.com/faciam-dev/gcfm/pkg/driver/mysql"
	pscanner "github.com/faciam-dev/gcfm/pkg/driver/postgres"
	"github.com/faciam-dev/gcfm/pkg/util"
)

// Scan reads the database schema and returns registry metadata.
func (s *service) Scan(ctx context.Context, cfg DBConfig) ([]registry.FieldMeta, error) {
	drv := cfg.Driver
	if drv == "" {
		var err error
		drv, err = util.DetectDriver(cfg.DSN)
		if err != nil {
			return nil, err
		}
	}
	switch drv {
	case "postgres":
		db, err := sql.Open("postgres", cfg.DSN)
		if err != nil {
			return nil, err
		}
		defer func() { _ = db.Close() }()
		sc := pscanner.NewScanner(db)
		return sc.Scan(ctx, registry.DBConfig{DSN: cfg.DSN, Schema: cfg.Schema, TablePrefix: cfg.TablePrefix})
	case "mongo":
		cli, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.DSN))
		if err != nil {
			return nil, err
		}
		defer func() { _ = cli.Disconnect(ctx) }()
		sc := mongoscanner.NewScanner(cli)
		return sc.Scan(ctx, registry.DBConfig{Schema: cfg.Schema, TablePrefix: cfg.TablePrefix})
	case "sqlmock":
		db, err := sql.Open("sqlmock", cfg.DSN)
		if err != nil {
			return nil, err
		}
		defer func() { _ = db.Close() }()
		return registry.LoadSQL(ctx, db, registry.DBConfig{Schema: cfg.Schema, Driver: cfg.Driver, TablePrefix: cfg.TablePrefix})
	default:
		db, err := sql.Open("mysql", cfg.DSN)
		if err != nil {
			return nil, err
		}
		defer func() { _ = db.Close() }()
		sc := mysqlscanner.NewScanner(db)
		return sc.Scan(ctx, registry.DBConfig{DSN: cfg.DSN, Schema: cfg.Schema, TablePrefix: cfg.TablePrefix})
	}
}
