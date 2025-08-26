package sdk

import (
	"context"
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/faciam-dev/gcfm/pkg/migrator"
	"github.com/faciam-dev/gcfm/pkg/util"
)

// MigrateRegistry upgrades or downgrades the registry schema to the target version.
func (s *service) MigrateRegistry(ctx context.Context, cfg DBConfig, target int) error {
	drv := cfg.Driver
	if drv == "" {
		var err error
		drv, err = util.DetectDriver(cfg.DSN)
		if err != nil {
			return err
		}
	}
	db, err := sql.Open(drv, cfg.DSN)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	m := migrator.NewWithDriverAndPrefix(drv, cfg.TablePrefix)
	cur, err := m.Current(ctx, db)
	if err != nil && err != migrator.ErrNoVersionTable {
		return err
	}
	if target == 0 {
		target = len(migrator.DefaultForDriver(drv))
	}
	if target > cur {
		return m.Up(ctx, db, target)
	}
	if target < cur {
		return m.Down(ctx, db, target)
	}
	s.logger.Infow("registry schema up-to-date", "version", m.SemVer(cur))
	return nil
}

// RegistryVersion returns the current registry schema version.
func (s *service) RegistryVersion(ctx context.Context, cfg DBConfig) (int, error) {
	drv := cfg.Driver
	if drv == "" {
		var err error
		drv, err = util.DetectDriver(cfg.DSN)
		if err != nil {
			return 0, err
		}
	}
	db, err := sql.Open(drv, cfg.DSN)
	if err != nil {
		return 0, err
	}
	defer func() { _ = db.Close() }()
	m := migrator.NewWithDriverAndPrefix(drv, cfg.TablePrefix)
	return m.Current(ctx, db)
}
