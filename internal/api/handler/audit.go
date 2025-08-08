package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
	"github.com/faciam-dev/gcfm/internal/auditlog"
	"github.com/pmezard/go-difflib/difflib"
)

type AuditHandler struct {
	DB     *sql.DB
	Driver string
}

// getDiff returns unified diff for an audit record
func (h *AuditHandler) getDiff(ctx context.Context,
	p *struct {
		ID int64 `path:"id"`
	}) (*huma.StreamResponse, error) {
	var before, after sql.NullString
	query := `SELECT COALESCE(before_json::text,'{}'), COALESCE(after_json::text,'{}') FROM gcfm_audit_logs WHERE id = $1`
	if h.Driver == "mysql" {
		query = `SELECT COALESCE(JSON_UNQUOTE(before_json), '{}'), COALESCE(JSON_UNQUOTE(after_json), '{}') FROM gcfm_audit_logs WHERE id = ?`
	}
	if err := h.DB.QueryRowContext(ctx, query, p.ID).Scan(&before, &after); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("not found")
		}
		return nil, err
	}

	prettify := func(js string) string {
		var buf bytes.Buffer
		if err := json.Indent(&buf, []byte(js), "", "  "); err != nil {
			return js
		}
		return buf.String()
	}
	left := prettify(before.String)
	right := prettify(after.String)

	ud := difflib.UnifiedDiff{
		A:        difflib.SplitLines(left),
		B:        difflib.SplitLines(right),
		FromFile: "before",
		ToFile:   "after",
		Context:  3,
	}
	text, err := difflib.GetUnifiedDiffString(ud)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to generate diff: " + err.Error())
	}
	return &huma.StreamResponse{Body: func(hctx huma.Context) {
		hctx.SetHeader("Content-Type", "text/plain")
		if _, err := hctx.BodyWriter().Write([]byte(text)); err != nil {
			log.Printf("error writing diff response: %v", err)
		}
	}}, nil
}

type auditParams struct {
	Limit int       `query:"limit"`
	Since time.Time `query:"since"`
}

type auditOutput struct {
	Body []schema.AuditLog
}

type auditGetParams struct {
	ID int64 `path:"id"`
}

type auditGetOutput struct {
	Body schema.AuditLog
}

func RegisterAudit(api huma.API, h *AuditHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "listAuditLogs",
		Method:      http.MethodGet,
		Path:        "/v1/audit-logs",
		Summary:     "List audit logs",
		Tags:        []string{"Audit"},
	}, h.list)

	huma.Register(api, huma.Operation{
		OperationID: "getAuditLogByID",
		Method:      http.MethodGet,
		Path:        "/v1/audit-logs/{id}",
		Summary:     "Get audit log by ID",
		Tags:        []string{"Audit"},
	}, h.get)

	// Register diff endpoint
	huma.Register(api, huma.Operation{
		OperationID: "getAuditDiff",
		Method:      http.MethodGet,
		Path:        "/v1/audit-logs/{id}/diff",
		Summary:     "Get unified diff for an audit log",
		Tags:        []string{"Audit"},
		Responses: map[string]*huma.Response{
			"200": {
				Content: map[string]*huma.MediaType{
					"text/plain": {Schema: &huma.Schema{Type: "string"}},
				},
			},
		},
	}, h.getDiff)
}

func (h *AuditHandler) list(ctx context.Context, p *auditParams) (*auditOutput, error) {
	limit := p.Limit
	if limit == 0 {
		limit = 100
	}
	placeholder := "$1"
	joinCond := "u.id::text = l.actor"
	if h.Driver == "mysql" {
		placeholder = "?"
		joinCond = "u.id = CAST(l.actor AS UNSIGNED)"
	}
	query := `SELECT l.id, COALESCE(u.username, l.actor) AS actor, l.action, l.table_name, l.column_name, l.before_json, l.after_json, l.applied_at FROM gcfm_audit_logs l LEFT JOIN gcfm_users u ON ` + joinCond + ` ORDER BY l.id DESC LIMIT ` + placeholder
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
		switch l.Action {
		case "snapshot", "rollback":
			if beforeJSON.Valid {
				l.Summary = beforeJSON.String
			}
		default:
			var addCnt, delCnt int
			if l.Action == "add" {
				addCnt = 1
			} else if l.Action == "delete" {
				delCnt = 1
			}
			l.Summary = fmt.Sprintf("+%d -%d", addCnt, delCnt)
			l.BeforeJSON = beforeJSON
			l.AfterJSON = afterJSON
		}
		l.DiffURL = fmt.Sprintf("/v1/audit-logs/%d/diff", l.ID)
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &auditOutput{Body: logs}, nil
}

func (h *AuditHandler) get(ctx context.Context, p *auditGetParams) (*auditGetOutput, error) {
	repo := auditlog.Repo{DB: h.DB, Driver: h.Driver}
	rec, err := repo.FindByID(ctx, p.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("not found")
		}
		return nil, err
	}
	t, err := ParseAuditTime(rec.AppliedAt)
	if err != nil {
		return nil, err
	}
	log := schema.AuditLog{
		ID:         int(rec.ID),
		Actor:      rec.Actor,
		Action:     rec.Action,
		TableName:  rec.TableName,
		ColumnName: rec.ColumnName,
		AppliedAt:  t,
	}
	switch rec.Action {
	case "snapshot", "rollback":
		if rec.BeforeJSON.Valid {
			log.Summary = rec.BeforeJSON.String
		}
	default:
		var addCnt, delCnt int
		if rec.Action == "add" {
			addCnt = 1
		} else if rec.Action == "delete" {
			delCnt = 1
		}
		log.Summary = fmt.Sprintf("+%d -%d", addCnt, delCnt)
		log.BeforeJSON = rec.BeforeJSON
		log.AfterJSON = rec.AfterJSON
	}
	log.DiffURL = fmt.Sprintf("/v1/audit-logs/%d/diff", log.ID)
	return &auditGetOutput{Body: log}, nil
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
		return time.Time{}, errors.New("unsupported time type")
	}
}

func parseAuditTimeString(s string) (time.Time, error) {
	layouts := []string{time.RFC3339Nano, "2006-01-02 15:04:05", time.RFC3339}
	for _, l := range layouts {
		if ts, err := time.Parse(l, s); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, errors.New("cannot parse time: " + s)
}
