package monitordb

import (
	"context"
	"log"

	mysqlDriver "github.com/go-sql-driver/mysql"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/pkg/crypto"
	"github.com/faciam-dev/gcfm/sdk"
)

// ScanDatabase scans the monitored database and upserts custom fields.
func ScanDatabase(ctx context.Context, repo *Repo, id int64, tenant string) error {
	d, err := repo.Get(ctx, tenant, id)
	if err != nil {
		return err
	}
	dsnBytes, err := crypto.Decrypt(d.DSNEnc)
	if err != nil {
		return err
	}
	dsn := string(dsnBytes)
	schema := schemaFromDSN(d.Driver, dsn)
	svc := sdk.New(sdk.ServiceConfig{})
	metas, err := svc.Scan(ctx, sdk.DBConfig{Driver: d.Driver, DSN: dsn, Schema: schema})
	if err != nil {
		return err
	}
	return registry.UpsertSQLByTenant(ctx, repo.DB, repo.Driver, d.TenantID, metas)
}

func schemaFromDSN(driver, dsn string) string {
	switch driver {
	case "mysql":
		if cfg, err := mysqlDriver.ParseDSN(dsn); err == nil {
			return cfg.DBName
		}
	}
	// default schema
	if driver == "postgres" {
		return "public"
	}
	return ""
}

// helper for scanning multiple databases
func ScanAll(ctx context.Context, repo *Repo, dbs []Database) {
	for _, d := range dbs {
		if err := ScanDatabase(ctx, repo, d.ID, d.TenantID); err != nil {
			log.Printf("ScanDatabase failed for ID %d (tenant %s): %v", d.ID, d.TenantID, err)
		}
	}
}
