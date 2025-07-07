package migrator

import (
	"context"
	"database/sql"
	"fmt"
)

// ensureVersionTable creates the version table if it doesn't exist and inserts
// an initial row with version=0. It is safe to call multiple times.
func (m *Migrator) ensureVersionTable(ctx context.Context, db *sql.DB) error {
	tbl := fmt.Sprintf("%sregistry_schema_version", m.tablePrefix)
	_, err := db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
        version INT PRIMARY KEY,
        applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`, tbl))
	if err != nil {
		return err
	}
	// insert the zero row if not present
	_, _ = db.ExecContext(ctx, fmt.Sprintf(
		`INSERT INTO %s(version) VALUES(0) ON CONFLICT (version) DO NOTHING;`, tbl))
	return nil
}
