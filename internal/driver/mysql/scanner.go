package mysql

import (
	"database/sql"

	"github.com/faciam-dev/gcfm/internal/driver/sqlscan"
)

const scanQuery = "SELECT TABLE_NAME, COLUMN_NAME, DATA_TYPE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME != '%s' ORDER BY TABLE_NAME, ORDINAL_POSITION"

type Scanner = sqlscan.Scanner

func NewScanner(db *sql.DB) *Scanner {
	return sqlscan.New(db, scanQuery)
}
