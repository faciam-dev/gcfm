package monitordb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	ccrypto "github.com/faciam-dev/gcfm/internal/customfield/crypto"
)

type Record struct {
	ID     int64
	Driver string
	DSN    string
	Schema string // postgres schema, empty for MySQL
	DSNEnc []byte
}

var ErrNotFound = errors.New("monitored database not found")

func GetByID(ctx context.Context, db *sql.DB, driver, prefix, tenant string, id int64) (Record, error) {
	tbl := prefix + "monitored_databases"
	if prefix == "" {
		tbl = "gcfm_monitored_databases"
	}
	q := fmt.Sprintf(`SELECT id, driver, dsn, COALESCE(schema_name,''), dsn_enc FROM %s WHERE id=? AND tenant_id=?`, tbl)
	if driver == "postgres" {
		q = fmt.Sprintf(`SELECT id, driver, dsn, COALESCE(schema_name,''), dsn_enc FROM %s WHERE id=$1 AND tenant_id=$2`, tbl)
	}
	var rec Record
	// if monitored_databases table lacks tenant_id, remove tenant condition accordingly
	err := db.QueryRowContext(ctx, q, id, tenant).Scan(&rec.ID, &rec.Driver, &rec.DSN, &rec.Schema, &rec.DSNEnc)
	if err == sql.ErrNoRows {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
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
