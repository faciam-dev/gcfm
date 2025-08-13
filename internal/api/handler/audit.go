package handler

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
	"github.com/faciam-dev/gcfm/internal/auditlog"
	"github.com/faciam-dev/gcfm/internal/logger"
	"github.com/faciam-dev/gcfm/internal/tenant"
	auditutil "github.com/faciam-dev/gcfm/pkg/audit"
)

type AuditHandler struct {
	DB          *sql.DB
	Driver      string
	TablePrefix string
}

// auditLogOverfetchMultiplier controls how many extra rows are fetched when
// applying change-count filters in memory. This compensates for rows that may be
// discarded after post-processing so that the client still receives up to the
// requested limit.
const auditLogOverfetchMultiplier = 4

// auditDiffOutput represents the diff response body.
type auditDiffOutput struct {
	Body struct {
		Unified string `json:"unified"`
		Added   int    `json:"added"`
		Removed int    `json:"removed"`
	}
}

// getDiff returns unified diff for an audit record
func (h *AuditHandler) getDiff(ctx context.Context,
	p *struct {
		ID int64 `path:"id"`
	}) (*auditDiffOutput, error) {
	var before, after sql.NullString
	tbl := h.TablePrefix + "audit_logs"
	query := fmt.Sprintf("SELECT COALESCE(before_json::text,'{}'), COALESCE(after_json::text,'{}') FROM %s WHERE id = $1", tbl)
	if h.Driver == "mysql" {
		query = fmt.Sprintf("SELECT COALESCE(JSON_UNQUOTE(before_json), '{}'), COALESCE(JSON_UNQUOTE(after_json), '{}') FROM %s WHERE id = ?", tbl)
	}
	if err := h.DB.QueryRowContext(ctx, query, p.ID).Scan(&before, &after); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("not found")
		}
		return nil, err
	}

	unified, add, del := auditutil.UnifiedDiff([]byte(before.String), []byte(after.String))
	out := &auditDiffOutput{}
	out.Body.Unified = unified
	out.Body.Added = add
	out.Body.Removed = del
	return out, nil
}

type auditListParams struct {
	Limit      int         `query:"limit"`
	Cursor     string      `query:"cursor"` // base64("RFC3339Nano:id")
	Action     string      `query:"action"` // "add,update" など
	Actor      string      `query:"actor"`
	Table      string      `query:"table"`
	Column     string      `query:"column"`
	From       string      `query:"from"` // ISO
	To         string      `query:"to"`   // ISO (閉区間上端は < To+1day にします)
	MinChanges optionalInt `query:"min_changes"`
	MaxChanges optionalInt `query:"max_changes"`
	// Compatibility aliases for camelCase parameters
	MinChangesAlias optionalInt `query:"minChanges" json:"-" huma:"deprecated"`
	MaxChangesAlias optionalInt `query:"maxChanges" json:"-" huma:"deprecated"`
}

type optionalInt struct {
	Set bool
	Val int
}

func (o *optionalInt) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		o.Set = false
		o.Val = 0
		return nil
	}
	v, err := strconv.Atoi(string(b))
	if err != nil {
		return err
	}
	o.Set = true
	o.Val = v
	return nil
}

func (p *auditListParams) EffMin() *int {
	if p.MinChanges.Set {
		v := p.MinChanges.Val
		if v < 0 {
			v = 0
		}
		return &v
	}
	if p.MinChangesAlias.Set {
		v := p.MinChangesAlias.Val
		if v < 0 {
			v = 0
		}
		return &v
	}
	return nil
}

func (p *auditListParams) EffMax() *int {
	if p.MaxChanges.Set {
		v := p.MaxChanges.Val
		if v < 0 {
			v = 0
		}
		return &v
	}
	if p.MaxChangesAlias.Set {
		v := p.MaxChangesAlias.Val
		if v < 0 {
			v = 0
		}
		return &v
	}
	return nil
}

type AuditDTO struct {
	ID          int64           `json:"id"`
	Actor       string          `json:"actor"`
	Action      string          `json:"action"`
	TableName   string          `json:"tableName"`
	ColumnName  string          `json:"columnName"`
	AppliedAt   time.Time       `json:"appliedAt"`
	BeforeJson  json.RawMessage `json:"beforeJson"`
	AfterJson   json.RawMessage `json:"afterJson"`
	Summary     string          `json:"summary,omitempty"`
	ChangeCount int             `json:"changeCount"`
}

type auditListOutput struct {
	Body struct {
		Items      []AuditDTO `json:"items"`
		NextCursor *string    `json:"nextCursor,omitempty"`
	}
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
	}, h.getDiff)
}

func (h *AuditHandler) list(ctx context.Context, p *auditListParams) (_ *auditListOutput, err error) {
	defer func() {
		if err != nil {
			logger.L.Error("list audit logs", "err", err)
		}
	}()

	tid := tenant.FromContext(ctx)

	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	min := p.EffMin()
	max := p.EffMax()
	pass := func(v int) bool {
		if min != nil && v < *min {
			return false
		}
		if max != nil && v > *max {
			return false
		}
		return true
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
	args = append(args, limit*auditLogOverfetchMultiplier+1)

	query := fmt.Sprintf(`
      SELECT id, actor, action, table_name, column_name,
             `+coalesceBefore+` AS before_json,
             `+coalesceAfter+` AS after_json,
             added_count, removed_count, change_count,
             applied_at
        FROM %s
      `+where+`
        ORDER BY applied_at DESC, id DESC
        LIMIT `+limitPlaceholder, tbl)

	rows, err := h.DB.QueryContext(ctx, query, args...)
	if err != nil {
		logger.L.Error("query audit logs", "err", err)
		return nil, err
	}
	defer rows.Close()

	items := make([]AuditDTO, 0, limit)
	var nextCursor *string
	var lastReturnedID int64
	var lastReturnedApplied time.Time
	more := false

	for rows.Next() {
		var it AuditDTO
		var bj, aj sql.RawBytes
		var addCnt, delCnt, chCnt int
		var applied any
		if err := rows.Scan(&it.ID, &it.Actor, &it.Action, &it.TableName, &it.ColumnName, &bj, &aj, &addCnt, &delCnt, &chCnt, &applied); err != nil {
			logger.L.Error("scan audit row", "err", err)
			return nil, err
		}
		t, err := ParseAuditTime(applied)
		if err != nil {
			logger.L.Error("parse audit time", "err", err, "value", applied)
			return nil, err
		}
		it.AppliedAt = t
		it.BeforeJson = append([]byte(nil), bj...)
		it.AfterJson = append([]byte(nil), aj...)
		if chCnt == 0 && it.Action != "snapshot" && it.Action != "rollback" {
			// diff unavailable for legacy records; should be backfilled via migration
			it.Summary = "diff unavailable"
		} else if it.Action == "snapshot" || it.Action == "rollback" {
			it.Summary = string(bj)
		} else {
			it.Summary = fmt.Sprintf("+%d -%d", addCnt, delCnt)
		}
		it.ChangeCount = chCnt

		if !pass(chCnt) {
			continue
		}
		if len(items) < limit {
			items = append(items, it)
			lastReturnedID = it.ID
			lastReturnedApplied = it.AppliedAt
		} else {
			more = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		logger.L.Error("iterate audit rows", "err", err)
		return nil, err
	}

	if more {
		c := encodeCursor(lastReturnedApplied, lastReturnedID)
		nextCursor = &c
	}

	return &auditListOutput{
		Body: struct {
			Items      []AuditDTO `json:"items"`
			NextCursor *string    `json:"nextCursor,omitempty"`
		}{
			Items:      items,
			NextCursor: nextCursor,
		},
	}, nil
}

func (h *AuditHandler) get(ctx context.Context, p *auditGetParams) (*auditGetOutput, error) {
	repo := auditlog.Repo{DB: h.DB, Driver: h.Driver, TablePrefix: h.TablePrefix}
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
	enrichAuditLog(&log, rec.BeforeJSON, rec.AfterJSON, rec.AddedCount, rec.RemovedCount)
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
	layouts := []string{time.RFC3339Nano, "2006-01-02 15:04:05.000000000", "2006-01-02 15:04:05", time.RFC3339}
	for _, l := range layouts {
		if ts, err := time.Parse(l, s); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %s", s)
}

func enrichAuditLog(l *schema.AuditLog, beforeJSON, afterJSON sql.NullString, addCnt, delCnt int) {
	if addCnt+delCnt == 0 && l.Action != "snapshot" && l.Action != "rollback" {
		_, addCnt, delCnt = auditutil.UnifiedDiff([]byte(beforeJSON.String), []byte(afterJSON.String))
	}
	switch l.Action {
	case "snapshot", "rollback":
		if beforeJSON.Valid {
			l.Summary = beforeJSON.String
		}
	default:
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
	s := string(b)
	idx := strings.LastIndex(s, ":")
	if idx < 0 {
		return time.Time{}, 0, fmt.Errorf("invalid cursor format: missing timestamp separator ':' in %q", s)
	}
	ts, err := time.Parse(time.RFC3339Nano, s[:idx])
	if err != nil {
		return time.Time{}, 0, err
	}
	id, err := strconv.ParseInt(s[idx+1:], 10, 64)
	if err != nil {
		return time.Time{}, 0, err
	}
	return ts, id, nil
}
