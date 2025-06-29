package schema

import "time"

// AuditLog represents a single audit log entry
// returned by GET /v1/audit-logs
type AuditLog struct {
	ID         int       `json:"id"`
	Actor      string    `json:"actor"`
	Action     string    `json:"action"`
	TableName  string    `json:"tableName"`
	ColumnName string    `json:"columnName"`
	BeforeJSON string    `json:"beforeJson,omitempty"`
	AfterJSON  string    `json:"afterJson,omitempty"`
	AppliedAt  time.Time `json:"appliedAt"`
}
