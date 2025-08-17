package auditlog

import (
	"context"
	"database/sql"

	qbapi "github.com/faciam-dev/goquent-query-builder/api"
	"github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

// Record represents a single audit log entry in the database.
type Record struct {
	ID           int64
	Actor        string
	Action       string
	TableName    string
	ColumnName   string
	BeforeJSON   sql.NullString
	AfterJSON    sql.NullString
	AddedCount   int
	RemovedCount int
	ChangeCount  int
	AppliedAt    any
}

// Repo provides access to audit log records.
type Repo struct {
	DB          *sql.DB
	Dialect     driver.Dialect
	TablePrefix string
}

// FindByID returns a record by its ID.
func (r *Repo) FindByID(ctx context.Context, id int64) (Record, error) {
	if r == nil || r.DB == nil {
		return Record{}, sql.ErrConnDone
	}
	logs := r.TablePrefix + "audit_logs"
	users := r.TablePrefix + "users"

	q := query.New(r.DB, logs+" l", r.Dialect).
		Select("l.id").
		SelectRaw("COALESCE(u.username, l.actor) AS actor").
		Select("l.action").
		SelectRaw("COALESCE(l.table_name, '') AS table_name").
		SelectRaw("COALESCE(l.column_name, '') AS column_name").
		Select("l.before_json", "l.after_json", "l.added_count", "l.removed_count", "l.change_count", "l.applied_at").
		LeftJoinQuery(users+" u", func(b *qbapi.JoinClauseQueryBuilder) {
			if _, ok := r.Dialect.(driver.PostgresDialect); ok {
				b.On("u.id::text", "=", "l.actor")
			} else {
				b.On("u.id", "=", "CAST(l.actor AS UNSIGNED)")
			}
		}).
		Where("l.id", id).
		WithContext(ctx)

	var rec Record
	if err := q.First(&rec); err != nil {
		return rec, err
	}
	return rec, nil
}
