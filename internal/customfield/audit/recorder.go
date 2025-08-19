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
	"github.com/faciam-dev/gcfm/internal/tenant"
	audutil "github.com/faciam-dev/gcfm/pkg/audit"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

// Recorder writes audit logs to the database.
type Recorder struct {
	DB          *sql.DB
	Dialect     ormdriver.Dialect
	TablePrefix string
}

// enableVerboseAuditLogs controls whether detailed diff information is logged
// for each audit entry. This should remain disabled in production unless
// troubleshooting is needed.
var enableVerboseAuditLogs = false

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
	if enableVerboseAuditLogs {
		logger.L.Debug("audit diff", "summary", summary, "before", beforeNorm, "after", afterNorm, "diff", strings.Join(lines, "\n"), "added", addCnt, "removed", delCnt)
	}

	tbl := r.TablePrefix + "audit_logs"
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
	_, err = query.New(r.DB, tbl, r.Dialect).WithContext(ctx).Insert(map[string]any{
		"tenant_id":     tenant.FromContext(ctx),
		"actor":         actor,
		"action":        action,
		"table_name":    table,
		"column_name":   column,
		"before_json":   beforeJSON,
		"after_json":    afterJSON,
		"added_count":   addCnt,
		"removed_count": delCnt,
		"change_count":  addCnt + delCnt,
	})
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
	tbl := r.TablePrefix + "audit_logs"
	before := sql.NullString{Valid: diffSummary != "", String: diffSummary}
	_, err := query.New(r.DB, tbl, r.Dialect).WithContext(ctx).Insert(map[string]any{
		"actor":         actor,
		"action":        action,
		"table_name":    "registry",
		"column_name":   targetVer,
		"before_json":   before,
		"after_json":    sql.NullString{Valid: false},
		"added_count":   0,
		"removed_count": 0,
		"change_count":  0,
	})
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
	tbl := r.TablePrefix + "audit_logs"
	_, err = query.New(r.DB, tbl, r.Dialect).WithContext(ctx).Insert(map[string]any{
		"actor":         actor,
		"action":        action,
		"table_name":    sql.NullString{Valid: false},
		"column_name":   sql.NullString{Valid: false},
		"before_json":   sql.NullString{Valid: false},
		"after_json":    string(data),
		"added_count":   0,
		"removed_count": 0,
		"change_count":  0,
	})
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
	tbl := r.TablePrefix + "audit_logs"
	_, err = query.New(r.DB, tbl, r.Dialect).WithContext(ctx).Insert(map[string]any{
		"actor":         actor,
		"action":        action,
		"table_name":    table,
		"column_name":   sql.NullString{Valid: false},
		"before_json":   sql.NullString{Valid: false},
		"after_json":    string(data),
		"added_count":   0,
		"removed_count": 0,
		"change_count":  0,
	})
	if err == nil {
		metrics.AuditEvents.WithLabelValues(action).Inc()
	} else {
		metrics.AuditErrors.WithLabelValues(action).Inc()
	}
	return err
}
