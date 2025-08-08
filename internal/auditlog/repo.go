package auditlog

import (
	"context"
	"database/sql"
)

// Record represents a single audit log entry in the database.
type Record struct {
	ID         int64
	Actor      string
	Action     string
	TableName  string
	ColumnName string
	BeforeJSON sql.NullString
	AfterJSON  sql.NullString
	AppliedAt  any
}

// Repo provides access to audit log records.
type Repo struct {
	DB     *sql.DB
	Driver string
}

const findByIDQueryPg = `
SELECT l.id, COALESCE(u.username, l.actor) AS actor, l.action, l.table_name, l.column_name,
       l.before_json, l.after_json, l.applied_at
FROM gcfm_audit_logs l
LEFT JOIN gcfm_users u ON u.id::text = l.actor
WHERE l.id=$1`

const findByIDQueryMy = `
SELECT l.id, COALESCE(u.username, l.actor) AS actor, l.action, l.table_name, l.column_name,
       l.before_json, l.after_json, l.applied_at
FROM gcfm_audit_logs l
LEFT JOIN gcfm_users u ON CAST(u.id AS CHAR) = l.actor
WHERE l.id=?`

// FindByID returns a record by its ID.
func (r *Repo) FindByID(ctx context.Context, id int64) (Record, error) {
	if r == nil || r.DB == nil {
		return Record{}, sql.ErrConnDone
	}
	q := findByIDQueryPg
	if r.Driver == "mysql" {
		q = findByIDQueryMy
	}
	var rec Record
	err := r.DB.QueryRowContext(ctx, q, id).Scan(
		&rec.ID, &rec.Actor, &rec.Action, &rec.TableName, &rec.ColumnName,
		&rec.BeforeJSON, &rec.AfterJSON, &rec.AppliedAt,
	)
	return rec, err
}
