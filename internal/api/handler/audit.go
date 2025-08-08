package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
	"github.com/faciam-dev/gcfm/internal/auditlog"
	"github.com/faciam-dev/gcfm/internal/tenant"
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

type auditListParams struct {
	Limit  int    `query:"limit"`
	Cursor string `query:"cursor"` // base64("RFC3339Nano:id")
	Action string `query:"action"` // "add,update" など
	Actor  string `query:"actor"`
	Table  string `query:"table"`
	Column string `query:"column"`
	From   string `query:"from"` // ISO
	To     string `query:"to"`   // ISO (閉区間上端は < To+1day にします)
}

type AuditDTO struct {
	ID         int64           `json:"id"`
	Actor      string          `json:"actor"`
	Action     string          `json:"action"`
	TableName  string          `json:"tableName"`
	ColumnName string          `json:"columnName"`
	AppliedAt  time.Time       `json:"appliedAt"`
	BeforeJson json.RawMessage `json:"beforeJson"`
	AfterJson  json.RawMessage `json:"afterJson"`
	Summary    string          `json:"summary,omitempty"` // 将来サーバ側計算
}

type auditListOutput struct {
	Items      []AuditDTO `json:"items"`
	NextCursor *string    `json:"nextCursor,omitempty"`
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

func (h *AuditHandler) list(ctx context.Context, p *auditListParams) (*auditListOutput, error) {
	tid := tenant.FromContext(ctx)

	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	args := []any{}
	next := func() string {
		if h.Driver == "postgres" {
			return fmt.Sprintf("$%d", len(args)+1)
		}
		return "?"
	}

	wh := []string{"tenant_id = " + next()}
	args = append(args, tid)

	if p.Actor != "" {
		wh = append(wh, "actor = "+next())
		args = append(args, p.Actor)
	}
	if p.Table != "" {
		wh = append(wh, "table_name = "+next())
		args = append(args, p.Table)
	}
	if p.Column != "" {
		wh = append(wh, "column_name = "+next())
		args = append(args, p.Column)
	}
	if p.Action != "" {
		acts := strings.Split(p.Action, ",")
		placeholders := make([]string, 0, len(acts))
		for _, a := range acts {
			a = strings.TrimSpace(a)
			if a == "" {
				continue
			}
			placeholders = append(placeholders, next())
			args = append(args, a)
		}
		if len(placeholders) > 0 {
			wh = append(wh, "action IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	if p.From != "" {
		if t, err := time.Parse(time.RFC3339, p.From); err == nil {
			wh = append(wh, "applied_at >= "+next())
			args = append(args, t)
		}
	}
	if p.To != "" {
		if t, err := time.Parse(time.RFC3339, p.To); err == nil {
			wh = append(wh, "applied_at < "+next())
			args = append(args, t.Add(24*time.Hour))
		}
	}
	if p.Cursor != "" {
		if ts, id, err := decodeCursor(p.Cursor); err == nil {
			ph1 := next()
			args = append(args, ts)
			ph2 := next()
			args = append(args, ts)
			ph3 := next()
			args = append(args, id)
			wh = append(wh, "(applied_at < "+ph1+" OR (applied_at = "+ph2+" AND id < "+ph3+"))")
		}
	}

	where := "WHERE " + strings.Join(wh, " AND ")

	coalesceBefore := "COALESCE(before_json, JSON_OBJECT())"
	coalesceAfter := "COALESCE(after_json , JSON_OBJECT())"
	if h.Driver == "postgres" {
		coalesceBefore = "COALESCE(before_json, '{}'::jsonb)"
		coalesceAfter = "COALESCE(after_json , '{}'::jsonb)"
	}

	limitPlaceholder := next()
	args = append(args, limit+1)

	query := `
      SELECT id, actor, action, table_name, column_name,
             ` + coalesceBefore + ` AS before_json,
             ` + coalesceAfter + ` AS after_json,
             applied_at
        FROM gcfm_audit_logs
      ` + where + `
        ORDER BY applied_at DESC, id DESC
        LIMIT ` + limitPlaceholder

	rows, err := h.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AuditDTO, 0, limit)
	var nextCursor *string

	for rows.Next() {
		var it AuditDTO
		var bj, aj sql.RawBytes
		var applied any
		if err := rows.Scan(&it.ID, &it.Actor, &it.Action, &it.TableName, &it.ColumnName, &bj, &aj, &applied); err != nil {
			return nil, err
		}
		t, err := ParseAuditTime(applied)
		if err != nil {
			return nil, err
		}
		it.AppliedAt = t
		it.BeforeJson = append([]byte(nil), bj...)
		it.AfterJson = append([]byte(nil), aj...)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(items) > limit {
		last := items[limit]
		c := encodeCursor(last.AppliedAt, last.ID)
		nextCursor = &c
		items = items[:limit]
	}

	return &auditListOutput{Items: items, NextCursor: nextCursor}, nil
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
	enrichAuditLog(&log, rec.BeforeJSON, rec.AfterJSON)
	return &auditGetOutput{Body: log}, nil
}

// ParseAuditTime converts a value returned from the database into a time.Time.
// Drivers like the MySQL driver may return []byte or string for TIMESTAMP
// columns when parseTime is disabled.
func ParseAuditTime(v any) (time.Time, error) {
	if v == nil {
		return time.Time{}, nil
	}
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
	layouts := []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05", time.RFC3339}
	for _, l := range layouts {
		if ts, err := time.Parse(l, s); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %s", s)
}

func enrichAuditLog(l *schema.AuditLog, beforeJSON, afterJSON sql.NullString) {
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
	}
	l.BeforeJSON = beforeJSON
	l.AfterJSON = afterJSON
	l.DiffURL = fmt.Sprintf("/v1/audit-logs/%d/diff", l.ID)
}

func encodeCursor(ts time.Time, id int64) string {
	s := fmt.Sprintf("%s:%d", ts.UTC().Format(time.RFC3339Nano), id)
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func decodeCursor(cur string) (time.Time, int64, error) {
	b, err := base64.StdEncoding.DecodeString(cur)
	if err != nil {
		return time.Time{}, 0, err
	}
	parts := strings.SplitN(string(b), ":", 2)
	if len(parts) != 2 {
		return time.Time{}, 0, fmt.Errorf("bad cursor")
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, 0, err
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return time.Time{}, 0, err
	}
	return ts, id, nil
}
