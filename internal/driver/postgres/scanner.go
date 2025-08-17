package postgres

import (
	"database/sql"

	"github.com/faciam-dev/gcfm/internal/driver/sqlscan"
)

const scanQuery = "SELECT table_name, column_name, data_type FROM information_schema.columns WHERE table_schema = $1 AND table_name != '%s' ORDER BY table_name, ordinal_position"

type Scanner = sqlscan.Scanner

func NewScanner(db *sql.DB) *Scanner {
	return sqlscan.New(db, scanQuery)
}
