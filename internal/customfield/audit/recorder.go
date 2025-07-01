package audit

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

// Recorder writes audit logs to the database.
type Recorder struct {
	DB     *sql.DB
	Driver string // mysql or postgres
}

// Write records a single field change.
func (r *Recorder) Write(ctx context.Context, actor string, old, new *registry.FieldMeta) error {
	if r == nil || r.DB == nil {
		return nil
	}
	var action string
	switch {
	case old == nil && new != nil:
		action = "add"
	case old != nil && new == nil:
		action = "delete"
	default:
		action = "update"
	}
	var before, after []byte
	var err error
	if old != nil {
		before, err = json.Marshal(old)
		if err != nil {
			return err
		}
	}
	if new != nil {
		after, err = json.Marshal(new)
		if err != nil {
			return err
		}
	}
	table := ""
	column := ""
	if new != nil {
		table = new.TableName
		column = new.ColumnName
	}
	if table == "" && old != nil {
		table = old.TableName
		column = old.ColumnName
	}
	q := "INSERT INTO audit_logs(actor, action, table_name, column_name, before_json, after_json) VALUES (?,?,?,?,?,?)"
	if r.Driver == "postgres" {
		q = "INSERT INTO audit_logs(actor, action, table_name, column_name, before_json, after_json) VALUES ($1,$2,$3,$4,$5,$6)"
	}
	var beforeArg interface{}
	if before != nil {
		beforeArg = string(before)
	}
	var afterArg interface{}
	if after != nil {
		afterArg = string(after)
	}
	_, err = r.DB.ExecContext(ctx, q, actor, action, table, column, beforeArg, afterArg)
	return err
}
