package widgetsrepo

import (
	"context"
	"time"
)

// Filter represents query parameters for listing widgets.
type Filter struct {
	Tenant  string
	ScopeIn []string
	Q       string
	Limit   int
	Offset  int
}

// Row represents a widget row stored in the database.
type Row struct {
	ID           string
	Name         string
	Version      string
	Type         string
	Scopes       []string
	Enabled      bool
	Description  *string
	Capabilities []string
	Homepage     *string
	Meta         map[string]any
	TenantScope  string
	Tenants      []string
	UpdatedAt    time.Time
}

// Repo defines the widget repository interface.
type Repo interface {
	List(ctx context.Context, f Filter) ([]Row, int, error)
	GetETagAndLastMod(ctx context.Context, f Filter) (string, time.Time, error)
	Upsert(ctx context.Context, r Row) error
	Remove(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (Row, error)
}
