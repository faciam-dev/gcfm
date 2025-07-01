package handler

import (
	"context"
	"database/sql"
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
	defer rows.Close()
	var logs []schema.AuditLog
	for rows.Next() {
		var l schema.AuditLog
		if err := rows.Scan(&l.ID, &l.Actor, &l.Action, &l.TableName, &l.ColumnName, &l.BeforeJSON, &l.AfterJSON, &l.AppliedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return &auditOutput{Body: logs}, nil
}
