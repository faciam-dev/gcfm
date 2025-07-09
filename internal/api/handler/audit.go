package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
)

type AuditHandler struct {
	DB     *sql.DB
	Driver string
}

type auditParams struct {
	Limit int       `query:"limit"`
	Since time.Time `query:"since"`
}

type auditOutput struct {
	Body []schema.AuditLog
}

func RegisterAudit(api huma.API, h *AuditHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "listAuditLogs",
		Method:      http.MethodGet,
		Path:        "/v1/audit-logs",
		Summary:     "List audit logs",
		Tags:        []string{"Audit"},
	}, h.list)
}

func (h *AuditHandler) list(ctx context.Context, p *auditParams) (*auditOutput, error) {
	limit := p.Limit
	if limit == 0 {
		limit = 100
	}
	placeholder := "$1"
	if h.Driver == "mysql" {
		placeholder = "?"
	}
	query := `SELECT id, actor, action, table_name, column_name, before_json, after_json, applied_at FROM gcfm_audit_logs ORDER BY id DESC LIMIT ` + placeholder
	rows, err := h.DB.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var logs []schema.AuditLog
	for rows.Next() {
		var l schema.AuditLog
		var beforeJSON, afterJSON sql.NullString
		var appliedAt any
		if err := rows.Scan(&l.ID, &l.Actor, &l.Action, &l.TableName, &l.ColumnName, &beforeJSON, &afterJSON, &appliedAt); err != nil {
			return nil, err
		}
		t, err := ParseAuditTime(appliedAt)
		if err != nil {
			return nil, err
		}
		l.AppliedAt = t
		l.BeforeJSON = beforeJSON
		l.AfterJSON = afterJSON
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &auditOutput{Body: logs}, nil
}

// ParseAuditTime converts a value returned from the database into a time.Time.
// Drivers like the MySQL driver may return []byte or string for TIMESTAMP
// columns when parseTime is disabled.
func ParseAuditTime(v any) (time.Time, error) {
	switch t := v.(type) {
	case time.Time:
		return t, nil
	case []byte:
		return parseAuditTimeString(string(t))
	case string:
		return parseAuditTimeString(t)
	default:
		return time.Time{}, fmt.Errorf("unsupported time type %T", v)
	}
}

func parseAuditTimeString(s string) (time.Time, error) {
	layouts := []string{time.RFC3339Nano, "2006-01-02 15:04:05", time.RFC3339}
	for _, l := range layouts {
		if ts, err := time.Parse(l, s); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", s)
}
