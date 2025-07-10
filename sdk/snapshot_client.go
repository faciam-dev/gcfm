package sdk

import "context"

// SnapshotClient provides snapshot operations.
type SnapshotClient interface {
	List(ctx context.Context, tenant string) ([]Snapshot, error)
	Create(ctx context.Context, tenant, bump, msg string) (Snapshot, error)
	Apply(ctx context.Context, tenant, ver string) error
	Diff(ctx context.Context, tenant, from, to string) (string, error)
}
