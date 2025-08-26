package meta

import (
	"context"
	"database/sql"
	"time"

	"github.com/faciam-dev/gcfm/pkg/registry"
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

// TargetRow represents a target database connection row.
type TargetRow struct {
	Key          string
	Driver       string
	DSN          string
	Schema       string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxIdle  time.Duration
	ConnMaxLife  time.Duration
	IsDefault    bool
}

// TargetRowWithLabels combines a target row with its labels.
type TargetRowWithLabels struct {
	TargetRow
	Labels []string
}

// MetaStore abstracts persistence of metadata including custom fields and targets.
type MetaStore interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	UpsertFieldDefs(ctx context.Context, tx *sql.Tx, defs []FieldDef) error
	DeleteFieldDefs(ctx context.Context, tx *sql.Tx, defs []FieldDef) error
	ListFieldDefs(ctx context.Context, tenantID string) ([]FieldDef, error)
	RecordScanResult(ctx context.Context, tx *sql.Tx, res ScanResult) error

	UpsertTarget(ctx context.Context, tx *sql.Tx, t TargetRow, labels []string) error
	DeleteTarget(ctx context.Context, tx *sql.Tx, key string) error
	ListTargets(ctx context.Context) ([]TargetRowWithLabels, string, string, error)
	SetDefaultTarget(ctx context.Context, tx *sql.Tx, key string) error
	BumpTargetsVersion(ctx context.Context, tx *sql.Tx) (string, error)
}
