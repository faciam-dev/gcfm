package audit

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
)

// Record represents an audit log record with diff information.
type Record struct {
	Diff string
}

// Get retrieves an audit log record by ID and returns its diff text.
func Get(ctx context.Context, db *sql.DB, id int64) (Record, error) {
	if db == nil {
		return Record{}, sql.ErrConnDone
	}
	// Determine placeholder based on driver type.
	placeholder := "$1"
	switch fmt.Sprintf("%T", db.Driver()) {
	case "*mysql.MySQLDriver":
		placeholder = "?"
	}
	query := fmt.Sprintf("SELECT diff FROM gcfm_audit_logs WHERE id=%s", placeholder)
	var raw []byte
	if err := db.QueryRowContext(ctx, query, id).Scan(&raw); err != nil {
		return Record{}, err
	}
	data := raw
	if isGzip(data) {
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err == nil {
			defer zr.Close()
			if d, err := io.ReadAll(zr); err == nil {
				data = d
			}
		}
	}
	return Record{Diff: string(data)}, nil
}

func isGzip(b []byte) bool {
	return len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b
}
