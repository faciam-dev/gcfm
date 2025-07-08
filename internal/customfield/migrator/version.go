package migrator

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
		`ALTER TABLE %s ADD COLUMN semver VARCHAR(32);`, tbl))
	if err != nil {
		msg := strings.ToLower(err.Error())
		if !strings.Contains(msg, "duplicate column") && !strings.Contains(msg, "already exists") {
			return err
		}
	}

	// insert the zero row if not present
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s(version) VALUES(0)`, tbl)); err != nil {
		msg := strings.ToLower(err.Error())
		if !strings.Contains(msg, "duplicate") && !strings.Contains(msg, "conflict") {
			return err
		}
	}
	return nil
}
