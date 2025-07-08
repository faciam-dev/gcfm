package migrator

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
)

// ensureVersionTable creates the version table if it doesn't exist and inserts
// an initial row with version=0. It is safe to call multiple times.
func (m *Migrator) ensureVersionTable(ctx context.Context, db *sql.DB) error {
	tbl := m.versionTable()
	_, err := db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
        version INT PRIMARY KEY,
        applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`, tbl))
	if err != nil {
		return err
	}
	// ensure semver column exists; older versions may lack it
	_, err = db.ExecContext(ctx, fmt.Sprintf(
		`ALTER TABLE %s ADD COLUMN IF NOT EXISTS semver VARCHAR(32);`, tbl))
	if err != nil && !isDuplicateColumnErr(err) {
		return err
	}

	// insert the zero row if not present
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s(version) VALUES(0)`, tbl)); err != nil && !isDuplicateEntryErr(err) {
		return err
	}
	return nil
}

func isDuplicateColumnErr(err error) bool {
	if err == nil {
		return false
	}
	if me, ok := err.(*mysql.MySQLError); ok {
		return me.Number == 1060
	}
	if pe, ok := err.(*pq.Error); ok {
		return string(pe.Code) == "42701"
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists")
}

func isDuplicateEntryErr(err error) bool {
	if err == nil {
		return false
	}
	if me, ok := err.(*mysql.MySQLError); ok {
		return me.Number == 1062
	}
	if pe, ok := err.(*pq.Error); ok {
		return string(pe.Code) == "23505"
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "conflict")
}
