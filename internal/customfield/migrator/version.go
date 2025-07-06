package migrator

import (
	"context"
	"database/sql"
	"fmt"
)

func (m *Migrator) ensureVersionTable(ctx context.Context, db *sql.DB) error {
	table := fmt.Sprintf("%sregistry_schema_version", m.tablePrefix)
	create := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
        version INT PRIMARY KEY,
        semver VARCHAR(20) NOT NULL,
        applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )`, table)
	if _, err := db.ExecContext(ctx, create); err != nil {
		return err
	}

	// ensure semver column exists if table was created previously without it
	var col string
	q := `SELECT column_name FROM information_schema.columns WHERE table_name=$1 AND column_name='semver'`
	err := db.QueryRowContext(ctx, q, table).Scan(&col)
	if err == sql.ErrNoRows {
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN semver VARCHAR(20) NOT NULL DEFAULT ''`, table)); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	var n int
	row := db.QueryRowContext(ctx, fmt.Sprintf("SELECT 1 FROM %s WHERE version=0", table))
	if err := row.Scan(&n); err != nil {
		if err != sql.ErrNoRows {
			return err
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s(version, semver) VALUES (0, '0.0')`, table)); err != nil {
			return err
		}
	}
	return nil
}
