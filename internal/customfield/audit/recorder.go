package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/logger"
	"github.com/faciam-dev/gcfm/internal/metrics"
	audutil "github.com/faciam-dev/gcfm/pkg/audit"
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
	unified, addCnt, delCnt := audutil.UnifiedDiff(before, after)
	summary := fmt.Sprintf("+%d -%d", addCnt, delCnt)
	beforeNorm := audutil.NormalizeJSON(before)
	afterNorm := audutil.NormalizeJSON(after)
	lines := strings.Split(unified, "\n")
	if len(lines) > 20 {
		lines = lines[:20]
	}
	logger.L.Debug("audit diff", "summary", summary, "before", beforeNorm, "after", afterNorm, "diff", strings.Join(lines, "\n"), "added", addCnt, "removed", delCnt)

	q := "INSERT INTO gcfm_audit_logs(actor, action, table_name, column_name, before_json, after_json, added_count, removed_count, change_count) VALUES (?,?,?,?,?,?,?,?,?)"
	if r.Driver == "postgres" {
		q = "INSERT INTO gcfm_audit_logs(actor, action, table_name, column_name, before_json, after_json, added_count, removed_count, change_count) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)"
	}
	var beforeJSON sql.NullString
	if before != nil {
		beforeJSON = sql.NullString{String: string(before), Valid: true}
	} else {
		beforeJSON = sql.NullString{Valid: false}
	}
	var afterJSON sql.NullString
	if after != nil {
		afterJSON = sql.NullString{String: string(after), Valid: true}
	} else {
		afterJSON = sql.NullString{Valid: false}
	}
	_, err = r.DB.ExecContext(ctx, q, actor, action, table, column, beforeJSON, afterJSON, addCnt, delCnt, addCnt+delCnt)
	if err == nil {
		metrics.AuditEvents.WithLabelValues(action).Inc()
	} else {
		metrics.AuditErrors.WithLabelValues(action).Inc()
	}
	return err
}

// WriteAction inserts a generic audit log entry for high level actions like
// snapshot creation or rollback. The diffSummary may be any short description
// such as "+2 -0".
func (r *Recorder) WriteAction(ctx context.Context, actor, action, targetVer, diffSummary string) error {
	if r == nil || r.DB == nil {
		return nil
	}
	q := "INSERT INTO gcfm_audit_logs(actor, action, table_name, column_name, before_json, after_json, added_count, removed_count, change_count) VALUES (?,?,?,?,?,?,?,?,?)"
	if r.Driver == "postgres" {
		q = "INSERT INTO gcfm_audit_logs(actor, action, table_name, column_name, before_json, after_json, added_count, removed_count, change_count) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)"
	}
	before := sql.NullString{Valid: diffSummary != "", String: diffSummary}
	_, err := r.DB.ExecContext(ctx, q, actor, action, "registry", targetVer, before, sql.NullString{Valid: false}, 0, 0, 0)
	if err == nil {
		metrics.AuditEvents.WithLabelValues(action).Inc()
	} else {
		metrics.AuditErrors.WithLabelValues(action).Inc()
	}
	return err
}

// WriteJSON writes an audit log entry with arbitrary JSON payload.
func (r *Recorder) WriteJSON(ctx context.Context, actor, action string, payload any) error {
	if r == nil || r.DB == nil {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	q := "INSERT INTO gcfm_audit_logs(actor, action, table_name, column_name, before_json, after_json, added_count, removed_count, change_count) VALUES (?,?,?,?,?,?,?,?,?)"
	if r.Driver == "postgres" {
		q = "INSERT INTO gcfm_audit_logs(actor, action, table_name, column_name, before_json, after_json, added_count, removed_count, change_count) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)"
	}
	_, err = r.DB.ExecContext(ctx, q, actor, action, sql.NullString{Valid: false}, sql.NullString{Valid: false}, sql.NullString{Valid: false}, string(data), 0, 0, 0)
	if err == nil {
		metrics.AuditEvents.WithLabelValues(action).Inc()
	} else {
		metrics.AuditErrors.WithLabelValues(action).Inc()
	}
	return err
}

// WriteTableJSON writes an audit log entry for a specific table with the given payload.
func (r *Recorder) WriteTableJSON(ctx context.Context, actor, action, table string, payload any) error {
	if r == nil || r.DB == nil {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	q := "INSERT INTO gcfm_audit_logs(actor, action, table_name, column_name, before_json, after_json, added_count, removed_count, change_count) VALUES (?,?,?,?,?,?,?,?,?)"
	if r.Driver == "postgres" {
		q = "INSERT INTO gcfm_audit_logs(actor, action, table_name, column_name, before_json, after_json, added_count, removed_count, change_count) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)"
	}
	_, err = r.DB.ExecContext(ctx, q, actor, action, table, sql.NullString{Valid: false}, sql.NullString{Valid: false}, string(data), 0, 0, 0)
	if err == nil {
		metrics.AuditEvents.WithLabelValues(action).Inc()
	} else {
		metrics.AuditErrors.WithLabelValues(action).Inc()
	}
	return err
}
