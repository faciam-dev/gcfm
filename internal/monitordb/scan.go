package monitordb

import (
	"context"
	"log"

	mysqlDriver "github.com/go-sql-driver/mysql"

	"github.com/faciam-dev/gcfm/internal/customfield"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/server/reserved"
	"github.com/faciam-dev/gcfm/pkg/crypto"
	"github.com/faciam-dev/gcfm/sdk"
)

// SkipInfo contains information about skipped fields during scanning.
type SkipInfo struct {
	Table  string
	Column string
	Reason string
}

// ScanDatabase scans the monitored database and upserts custom fields, returning
// the number of tables, inserted and updated fields along with any skipped columns.
func ScanDatabase(ctx context.Context, repo *Repo, id int64, tenant string) (tables, inserted, updated int, skipped []SkipInfo, err error) {
	d, err := repo.Get(ctx, tenant, id)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	dsnBytes, err := crypto.Decrypt(d.DSNEnc)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	dsn := string(dsnBytes)
	schema := schemaFromDSN(d.Driver, dsn)
	svc := sdk.New(sdk.ServiceConfig{})
	metas, err := svc.Scan(ctx, sdk.DBConfig{Driver: d.Driver, DSN: dsn, Schema: schema})
	if err != nil {
		return 0, 0, 0, nil, err
	}
	var (
		filtered []registry.FieldMeta
		tblSet   = make(map[string]struct{})
	)
	for _, m := range metas {
		tblSet[m.TableName] = struct{}{}
		if reserved.Is(m.TableName) {
			skipped = append(skipped, SkipInfo{Table: m.TableName, Column: m.ColumnName, Reason: "reserved"})
			continue
		}
		if m.Validator != "" {
			if _, ok := customfield.GetValidator(m.Validator); !ok {
				skipped = append(skipped, SkipInfo{Table: m.TableName, Column: m.ColumnName, Reason: "validator"})
				continue
			}
		}
		m.DBID = id
		filtered = append(filtered, m)
	}
	tables = len(tblSet)
	inserted, updated, err = registry.UpsertSQLByTenant(ctx, repo.DB, repo.Driver, d.TenantID, filtered)
	return tables, inserted, updated, skipped, err
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
		tables, inserted, updated, skipped, err := ScanDatabase(ctx, repo, d.ID, d.TenantID)
		if err != nil {
			log.Printf("ScanDatabase failed for ID %d (tenant %s): %v", d.ID, d.TenantID, err)
			continue
		}
		log.Printf("ScanDatabase success for ID %d: tables=%d inserted=%d updated=%d skipped=%d", d.ID, tables, inserted, updated, len(skipped))
	}
}
