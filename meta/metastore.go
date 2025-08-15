package meta

import (
	"context"
	"database/sql"
	"time"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

// FieldDef represents a custom field definition.
type FieldDef = registry.FieldMeta

// ScanResult captures results from schema scans.
type ScanResult struct {
	TenantID   string
	ScanID     string
	Status     string
	StartedAt  time.Time
	FinishedAt time.Time
	Details    string
}

// Store abstracts persistence of custom field metadata.
type Store interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	UpsertFieldDefs(ctx context.Context, tx *sql.Tx, defs []FieldDef) error
	DeleteFieldDefs(ctx context.Context, tx *sql.Tx, defs []FieldDef) error
	ListFieldDefs(ctx context.Context, tenantID string) ([]FieldDef, error)
	RecordScanResult(ctx context.Context, tx *sql.Tx, res ScanResult) error
}
