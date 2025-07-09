package schema

import (
	"database/sql"
	"encoding/json"
	"time"
)

// AuditLog represents a single audit log entry
// returned by GET /v1/audit-logs
type AuditLog struct {
	ID         int            `json:"id"`
	Actor      string         `json:"actor"`
	Action     string         `json:"action"`
	TableName  string         `json:"tableName"`
	ColumnName string         `json:"columnName"`
	BeforeJSON sql.NullString `json:"-"`
	AfterJSON  sql.NullString `json:"-"`
	AppliedAt  time.Time      `json:"appliedAt"`
}

func (a AuditLog) MarshalJSON() ([]byte, error) {
	type Alias AuditLog
	return json.Marshal(&struct {
		BeforeJSON *json.RawMessage `json:"beforeJson"`
		AfterJSON  *json.RawMessage `json:"afterJson"`
		*Alias
	}{
		BeforeJSON: nullableJSON(a.BeforeJSON),
		AfterJSON:  nullableJSON(a.AfterJSON),
		Alias:      (*Alias)(&a),
	})
}

func nullableJSON(ns sql.NullString) *json.RawMessage {
	if !ns.Valid {
		return nil
	}
	raw := json.RawMessage(ns.String)
	return &raw
}
