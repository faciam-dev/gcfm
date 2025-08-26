package monitordb

import (
	"context"
	"fmt"
	"log"

	mysqlDriver "github.com/go-sql-driver/mysql"

	"github.com/faciam-dev/gcfm/pkg/customfield"
	monitordbrepo "github.com/faciam-dev/gcfm/pkg/monitordb"
	"github.com/faciam-dev/gcfm/pkg/registry"
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
	if !monitordbrepo.HasDatabaseName(d.Driver, dsn) {
		return 0, 0, 0, nil, fmt.Errorf("monitored database DSN must include database name")
	}
	schema := schemaFromDSN(d.Driver, dsn)
	svc := sdk.New(sdk.ServiceConfig{})
	metas, err := svc.Scan(ctx, sdk.DBConfig{Driver: d.Driver, DSN: dsn, Schema: schema})
	if err != nil {
		return 0, 0, 0, nil, err
	}

	// Preserve existing validators so a rescan does not clear them when
	// the scanned metadata lacks validator information.
	existing, err := registry.LoadSQLByDB(ctx, repo.DB, registry.DBConfig{Driver: repo.Driver, TablePrefix: repo.TablePrefix}, d.TenantID, id)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	mergeValidators(metas, existing)

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
	inserted, updated, err = registry.UpsertSQLByTenant(ctx, repo.DB, repo.Driver, repo.TablePrefix, d.TenantID, filtered)
	return tables, inserted, updated, skipped, err
}

// fieldKey is used as a composite key for table and column names.
type fieldKey struct {
	table  string
	column string
}

func makeFieldKey(table, column string) fieldKey {
	return fieldKey{table: table, column: column}
}

// mergeValidators copies validator values from existing metadata to metas when
// the latter lack a validator. This prevents rescan operations from clearing
// validators that were previously configured for a column.
func mergeValidators(metas, existing []registry.FieldMeta) {
	if len(existing) == 0 {
		return
	}
	cache := make(map[fieldKey]string, len(existing))
	for _, e := range existing {
		if e.Validator != "" {
			cache[makeFieldKey(e.TableName, e.ColumnName)] = e.Validator
		}
	}
	for i := range metas {
		if metas[i].Validator != "" {
			continue
		}
		if v, ok := cache[makeFieldKey(metas[i].TableName, metas[i].ColumnName)]; ok {
			metas[i].Validator = v
		}
	}
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
