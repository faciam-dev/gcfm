package auditlog

import (
	"context"
	"database/sql"

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
	BeforeJSON   sql.NullString `db:"before_json"`
	AfterJSON    sql.NullString `db:"after_json"`
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
	isPg := false
	actorSub := "(SELECT username FROM " + users + " u WHERE "
	if _, ok := r.Dialect.(driver.PostgresDialect); ok {
		actorSub += "u.id::text = l.actor"
		isPg = true
	} else {
		actorSub += "u.id = CAST(l.actor AS UNSIGNED)"
	}
	actorSub += ")"

	coalesceBefore := "COALESCE(l.before_json, JSON_OBJECT())"
	coalesceAfter := "COALESCE(l.after_json , JSON_OBJECT())"
	if isPg {
		coalesceBefore = "COALESCE(l.before_json, '{}'::jsonb)"
		coalesceAfter = "COALESCE(l.after_json , '{}'::jsonb)"
	}

	q := query.New(r.DB, logs+" as l", r.Dialect).
		Select("l.id").
		SelectRaw("COALESCE("+actorSub+", l.actor) as actor").
		Select("l.action").
		SelectRaw("COALESCE(l.table_name, '') as table_name").
		SelectRaw("COALESCE(l.column_name, '') as column_name").
		SelectRaw(coalesceBefore+" as before_json").
		SelectRaw(coalesceAfter+" as after_json").
		Select("l.added_count", "l.removed_count", "l.change_count", "l.applied_at").
		Where("l.id", id).
		WithContext(ctx)

	var rec Record
	if err := q.First(&rec); err != nil {
		return rec, err
	}
	return rec, nil
}
