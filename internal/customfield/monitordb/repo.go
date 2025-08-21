package monitordb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	ccrypto "github.com/faciam-dev/gcfm/internal/customfield/crypto"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

type Record struct {
	ID     int64
	Driver string
	DSN    string
	Schema string // postgres schema, empty for MySQL
	DSNEnc []byte
}

var ErrNotFound = errors.New("monitored database not found")

func GetByID(ctx context.Context, db *sql.DB, d ormdriver.Dialect, prefix, tenant string, id int64) (Record, error) {
	tbl := prefix + "monitored_databases"
	if prefix == "" {
		tbl = "gcfm_monitored_databases"
	}

	type dbRecord struct {
		ID     int64
		Driver string
		DSN    string
		Schema sql.NullString `db:"schema_name,omitempty"`
		DSNEnc []byte         `db:"dsn_enc"`
	}

	q := query.New(db, tbl, d).
		Select("id", "driver", "dsn", "schema_name", "dsn_enc").
		Where("id", id).
		Where("tenant_id", tenant).
		WithContext(ctx)

	var tmp dbRecord
	if err := q.First(&tmp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Record{}, ErrNotFound
		}
		return Record{}, err
	}

	rec := Record{
		ID:     tmp.ID,
		Driver: tmp.Driver,
		DSN:    tmp.DSN,
		Schema: tmp.Schema.String,
		DSNEnc: tmp.DSNEnc,
	}

	// decrypt DSN if only encrypted form is available
	if rec.DSN == "" && len(rec.DSNEnc) > 0 {
		pt, derr := ccrypto.Decrypt(rec.DSNEnc)
		if derr != nil {
			return Record{}, fmt.Errorf("dsn_enc decrypt failed: %w", derr)
		}
		rec.DSN = string(pt)
	}
	if rec.DSN == "" {
		return Record{}, fmt.Errorf("monitored database (id=%d) has empty DSN and no usable dsn_enc", id)
	}
	if rec.Driver == "" {
		rec.Driver = "mysql"
	}
	return rec, nil
}

// EnsureExists inserts a placeholder monitored database record if the specified ID is missing.
func EnsureExists(ctx context.Context, db *sql.DB, d ormdriver.Dialect, prefix, tenant string, id int64) error {
	tbl := prefix + "monitored_databases"
	if prefix == "" {
		tbl = "gcfm_monitored_databases"
	}
	qtbl := quoteIdentifier(d, tbl)
	q := query.New(db, tbl, d).
		Select("id").
		Where("id", id).
		Where("tenant_id", tenant).
		WithContext(ctx)
	var tmp struct{ ID int64 }
	if err := q.First(&tmp); err == nil {
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	name := fmt.Sprintf("db_%d", id)
	driver := "mysql"
	enc, err := ccrypto.Encrypt([]byte{})
	if err != nil {
		return fmt.Errorf("ensure encrypt: %w", err)
	}
	switch d.(type) {
	case ormdriver.PostgresDialect, *ormdriver.PostgresDialect:
		driver = "postgres"
		_, err := db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (id, tenant_id, name, driver, dsn, dsn_enc) VALUES ($1,$2,$3,$4,'',$5) ON CONFLICT DO NOTHING", qtbl), id, tenant, name, driver, enc)
		if err != nil {
			return fmt.Errorf("ensure insert: %w", err)
		}
	default:
		_, err := db.ExecContext(ctx, fmt.Sprintf("INSERT IGNORE INTO %s (id, tenant_id, name, driver, dsn, dsn_enc) VALUES (?,?,?,?, '', ?)", qtbl), id, tenant, name, driver, enc)
		if err != nil {
			return fmt.Errorf("ensure insert: %w", err)
		}
	}
	return nil
}

func quoteIdentifier(d ormdriver.Dialect, ident string) string {
	switch d.(type) {
	case ormdriver.PostgresDialect, *ormdriver.PostgresDialect:
		return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
	default:
		return "`" + strings.ReplaceAll(ident, "`", "``") + "`"
	}
}
